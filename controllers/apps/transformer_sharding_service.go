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
	if model.IsObjectDeleting(transCtx.OrigCluster) || len(transCtx.Cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}
	if common.IsCompactMode(transCtx.OrigCluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create service objects", "cluster", client.ObjectKeyFromObject(transCtx.OrigCluster))
		return nil
	}

	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	// list all owned cluster services and filter out the services without sharding defined
	services, err := listOwnedClusterServices(transCtx.Context, transCtx.Client, cluster, withoutShardingLabel)
	if err != nil {
		return err
	}

	protoServices, err := t.buildShardingServices(transCtx, cluster)
	if err != nil {
		return err
	}

	toCreateServices, toDeleteServices, toUpdateServices := mapDiff(services, protoServices)

	for svc := range toCreateServices {
		graphCli.Create(dag, protoServices[svc], inDataContext4G())
	}
	for svc := range toUpdateServices {
		updateService(dag, graphCli, services[svc], protoServices[svc])
	}
	for svc := range toDeleteServices {
		graphCli.Delete(dag, services[svc], inDataContext4G())
	}

	return nil
}

func (t *shardingServiceTransformer) buildShardingServices(transCtx *shardingTransformContext, cluster *appsv1.Cluster) (map[string]*corev1.Service, error) {
	services := make(map[string]*corev1.Service)
	for i := range cluster.Spec.Services {
		svc := &cluster.Spec.Services[i]
		if len(svc.ComponentSelector) == 0 {
			continue
		}
		if _, exists := transCtx.ShardingToComponentSpecs[svc.ComponentSelector]; !exists {
			if !isComponentSelector(svc.ComponentSelector, cluster.Spec.ComponentSpecs) {
				return nil, fmt.Errorf("the component selector of service is not exist, service: %s, component slector: %s", svc.Name, svc.ComponentSelector)
			}
			// componentSelector points to a component, not a sharding, so skip
			continue
		}
		service, err := t.buildShardingService(transCtx, cluster, svc, svc.ComponentSelector)
		if err != nil {
			return nil, err
		}
		services[service.Name] = service
	}
	return services, nil
}

func (t *shardingServiceTransformer) buildShardingService(transCtx *shardingTransformContext, cluster *appsv1.Cluster,
	svc *appsv1.ClusterService, shardingSelector string) (*corev1.Service, error) {
	var (
		namespace   = cluster.Namespace
		clusterName = cluster.Name
	)
	serviceName := constant.GenerateClusterServiceName(cluster.Name, svc.ServiceName)
	builder := builder.NewServiceBuilder(namespace, serviceName).
		AddLabelsInMap(constant.GetClusterLabels(clusterName)).
		AddLabels(constant.KBAppShardingNameLabelKey, shardingSelector).
		AddAnnotationsInMap(svc.Annotations).
		SetSpec(&svc.Spec).
		AddSelectorsInMap(t.builtSelector(cluster, shardingSelector)).
		Optimize4ExternalTraffic()

	if len(svc.RoleSelector) > 0 {
		compDef, err := t.checkCompDef4ShardingSelector(transCtx, svc, shardingSelector)
		if err != nil {
			return nil, err
		}
		if err := checkComponentRoles(compDef, svc); err != nil {
			return nil, err
		}
		builder.AddSelector(constant.RoleLabelKey, svc.RoleSelector)
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

func (t *shardingServiceTransformer) builtSelector(cluster *appsv1.Cluster, shardingName string) map[string]string {
	selectors := map[string]string{
		constant.AppManagedByLabelKey:      constant.AppName,
		constant.AppInstanceLabelKey:       cluster.Name,
		constant.KBAppShardingNameLabelKey: shardingName,
	}
	return selectors
}

func (t *shardingServiceTransformer) checkCompDef4ShardingSelector(transCtx *shardingTransformContext,
	clusterService *appsv1.ClusterService, shardingSelector string) (*appsv1.ComponentDefinition, error) {
	compSecs, ok := transCtx.ShardingToComponentSpecs[shardingSelector]
	if !ok || len(compSecs) == 0 {
		return nil, fmt.Errorf("the sharding selector of service is not exist, service: %s, shard: %s", clusterService.Name, shardingSelector)
	}
	compDef, ok := transCtx.ComponentDefs[compSecs[0].ComponentDef]
	if !ok {
		return nil, fmt.Errorf("the component definition of service selector is not defined, service: %s, shard: %s", clusterService.Name, shardingSelector)
	}
	return compDef, nil
}

// isShardingSelector checks if the given component selector exists in the sharding specs.
func isShardingSelector(selector string, cluster *appsv1.Cluster) bool {
	if len(cluster.Spec.ShardingSpecs) == 0 || len(selector) == 0 {
		return false
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		if shardingSpec.Name == selector {
			return true
		}
	}
	return false
}
