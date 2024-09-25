/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package apps

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// shardingServiceTransformer handles cluster sharding services.
type shardingServiceTransformer struct{}

var _ graph.Transformer = &shardingServiceTransformer{}

func (t *shardingServiceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*shardingTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}
	if common.IsCompactMode(transCtx.OrigCluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create service objects", "cluster", client.ObjectKeyFromObject(transCtx.OrigCluster))
		return nil
	}

	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	// list all owned cluster services and filter out the services without sharding defined
	services, err := listOwnedClusterServices(transCtx.Context, transCtx.Client, cluster, withoutShardingDefined)
	if err != nil {
		return err
	}

	handleServiceFunc := func(origSvc, genSvc *appsv1.ClusterService) error {
		service, err := t.buildService4Sharding(transCtx, cluster, origSvc, genSvc)
		if err != nil {
			return err
		}
		if err = createOrUpdateService(ctx, dag, graphCli, service, nil); err != nil {
			return err
		}
		delete(services, service.Name)
		return nil
	}

	for i := range cluster.Spec.Services {
		svc := &cluster.Spec.Services[i]
		// cluster service without sharding selector will be handled in cluster controller
		if len(svc.ShardingSelector) == 0 {
			continue
		}
		if len(svc.ShardingSelector) > 0 && len(svc.ComponentSelector) > 0 {
			return fmt.Errorf("the ShardingSelector and ComponentSelector of service can't be defined at the same time, service: %s", svc.Name)
		}
		shardServices, err := t.genShardServiceIfNeed(transCtx, cluster, svc)
		if err != nil {
			return err
		}
		for j := range shardServices {
			genSvc := shardServices[j]
			if err = handleServiceFunc(svc, genSvc); err != nil {
				return err
			}
		}
	}

	for svc := range services {
		graphCli.Delete(dag, services[svc])
	}

	return nil
}

func (t *shardingServiceTransformer) buildService4Sharding(transCtx *shardingTransformContext, cluster *appsv1.Cluster,
	origSvc, genSvc *appsv1.ClusterService) (*corev1.Service, error) {
	var (
		namespace   = cluster.Namespace
		clusterName = cluster.Name
	)

	serviceName := constant.GenerateClusterServiceName(cluster.Name, genSvc.ServiceName)
	builder := builder.NewServiceBuilder(namespace, serviceName).
		AddLabelsInMap(constant.GetClusterLabels(clusterName)).
		AddLabels(constant.KBAppShardingNameLabelKey, genSvc.ShardingSelector).
		AddAnnotationsInMap(genSvc.Annotations).
		SetSpec(&genSvc.Spec).
		AddSelectorsInMap(t.builtServiceSelector(cluster, genSvc.ShardingSelector)).
		Optimize4ExternalTraffic()

	if enableShardService(cluster, genSvc.ShardingSelector) {
		builder.AddSelector(constant.KBAppComponentLabelKey, genComponentSelector(origSvc, genSvc))
	}

	if len(genSvc.RoleSelector) > 0 {
		compDef, err := t.checkCompDef4ShardingSelector(transCtx, genSvc)
		if err != nil {
			return nil, err
		}
		if err := checkComponentRoles(compDef, genSvc); err != nil {
			return nil, err
		}
		builder.AddSelector(constant.RoleLabelKey, genSvc.RoleSelector)
	}

	svcObj := builder.GetObject()
	// use SetOwnerReference instead of SetControllerReference
	if err := intctrlutil.SetOwnership(cluster, svcObj, rscheme, constant.DBClusterFinalizerName, true); err != nil {
		if _, ok := err.(*controllerutil.AlreadyOwnedError); ok {
			return svcObj, nil
		}
		return nil, err
	}
	return builder.GetObject(), nil
}

func (t *shardingServiceTransformer) genShardServiceIfNeed(transCtx *shardingTransformContext,
	cluster *appsv1.Cluster, clusterService *appsv1.ClusterService) ([]*appsv1.ClusterService, error) {
	shardingName := ""
	shardingCompSpecs := make([]*appsv1.ClusterComponentSpec, 0)
	for k, v := range transCtx.ShardingToComponentSpecs {
		if k != clusterService.ShardingSelector {
			continue
		}
		shardingName = k
		shardingCompSpecs = v
	}

	if len(shardingName) == 0 {
		return nil, fmt.Errorf("the ShardingSelector of service is not defined, service: %s, shard: %s", clusterService.Name, clusterService.ShardingSelector)
	}

	if !enableShardService(cluster, shardingName) {
		return []*appsv1.ClusterService{clusterService}, nil
	}

	shardOrdinalClusterSvcs := make([]*appsv1.ClusterService, 0, len(shardingCompSpecs))
	for _, shardingCompSpec := range shardingCompSpecs {
		svc := clusterService.DeepCopy()
		svc.Name = fmt.Sprintf("%s-%s", clusterService.Name, shardingCompSpec.Name)
		if len(clusterService.ServiceName) == 0 {
			svc.ServiceName = shardingCompSpec.Name
		} else {
			svc.ServiceName = fmt.Sprintf("%s-%s", clusterService.ServiceName, shardingCompSpec.Name)
		}
		shardOrdinalClusterSvcs = append(shardOrdinalClusterSvcs, svc)
	}
	return shardOrdinalClusterSvcs, nil
}

func (t *shardingServiceTransformer) builtServiceSelector(cluster *appsv1.Cluster, shardingName string) map[string]string {
	selectors := map[string]string{
		constant.AppManagedByLabelKey:      constant.AppName,
		constant.AppInstanceLabelKey:       cluster.Name,
		constant.KBAppShardingNameLabelKey: shardingName,
	}
	return selectors
}

func (t *shardingServiceTransformer) checkCompDef4ShardingSelector(transCtx *shardingTransformContext, clusterService *appsv1.ClusterService) (*appsv1.ComponentDefinition, error) {
	shardingName := clusterService.ShardingSelector
	compSecs, ok := transCtx.ShardingToComponentSpecs[shardingName]
	if !ok || len(compSecs) == 0 {
		return nil, fmt.Errorf("the sharding selector of service is not exist, service: %s, shard: %s", clusterService.Name, shardingName)
	}
	compDef, ok := transCtx.ComponentDefs[compSecs[0].ComponentDef]
	if !ok {
		return nil, fmt.Errorf("the component definition of service selector is not defined, service: %s, shard: %s", clusterService.Name, shardingName)
	}
	return compDef, nil
}

func enableShardService(cluster *appsv1.Cluster, shardingName string) bool {
	enableShardSvcList, ok := cluster.Annotations[constant.ShardSvcAnnotationKey]
	if !ok || !slices.Contains(strings.Split(enableShardSvcList, ","), shardingName) {
		return false
	}
	return true
}

// genComponentSelector generates component selector for sharding service.
func genComponentSelector(origSvc, genSvc *appsv1.ClusterService) string {
	origSvcPrefix := constant.GenerateShardingNameSvcPrefix(origSvc.Name)
	if strings.HasPrefix(genSvc.Name, origSvcPrefix) {
		return strings.TrimPrefix(genSvc.Name, origSvcPrefix)
	}
	return genSvc.Name
}
