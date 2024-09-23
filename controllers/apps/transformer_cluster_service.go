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
	"reflect"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterServiceTransformer handles cluster services.
type clusterServiceTransformer struct{}

var _ graph.Transformer = &clusterServiceTransformer{}

func (t *clusterServiceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}
	if common.IsCompactMode(transCtx.OrigCluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create service objects", "cluster", client.ObjectKeyFromObject(transCtx.OrigCluster))
		return nil
	}

	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	services, err := t.listOwnedClusterServices(transCtx, cluster)
	if err != nil {
		return err
	}

	handleServiceFunc := func(origSvc, genSvc *appsv1.ClusterService) error {
		service, err := t.buildService(transCtx, cluster, origSvc, genSvc)
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
		if len(svc.ShardingSelector) > 0 && len(svc.ComponentSelector) > 0 {
			return fmt.Errorf("the ShardingSelector and ComponentSelector of service can't be defined at the same time, service: %s", svc.Name)
		}
		genServices, err := t.genMultiServiceIfNeed(transCtx, cluster, svc)
		if err != nil {
			return err
		}
		for j := range genServices {
			genSvc := genServices[j]
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

func (t *clusterServiceTransformer) buildService(transCtx *clusterTransformContext, cluster *appsv1.Cluster,
	origSvc, genSvc *appsv1.ClusterService) (*corev1.Service, error) {
	var (
		namespace   = cluster.Namespace
		clusterName = cluster.Name
	)

	serviceName := constant.GenerateClusterServiceName(cluster.Name, genSvc.ServiceName)
	builder := builder.NewServiceBuilder(namespace, serviceName).
		AddLabelsInMap(constant.GetClusterLabels(clusterName)).
		AddAnnotationsInMap(genSvc.Annotations).
		SetSpec(&genSvc.Spec).
		AddSelectorsInMap(t.builtinSelector(cluster)).
		Optimize4ExternalTraffic()

	if len(genSvc.ShardingSelector) > 0 {
		builder.AddSelector(constant.KBAppShardingNameLabelKey, genSvc.ShardingSelector)
		if enableShardService(cluster, genSvc.ShardingSelector) {
			builder.AddSelector(constant.KBAppComponentLabelKey, genComponentSelector(origSvc, genSvc))
		}
	} else if len(genSvc.ComponentSelector) > 0 {
		builder.AddSelector(constant.KBAppComponentLabelKey, genSvc.ComponentSelector)
	}

	if len(genSvc.RoleSelector) > 0 {
		compDef, err := t.checkComponent(transCtx, genSvc)
		if err != nil {
			return nil, err
		}
		if err := t.checkComponentRoles(compDef, genSvc); err != nil {
			return nil, err
		}
		builder.AddSelector(constant.RoleLabelKey, genSvc.RoleSelector)
	}

	return builder.GetObject(), nil
}

func (t *clusterServiceTransformer) genMultiServiceIfNeed(transCtx *clusterTransformContext,
	cluster *appsv1.Cluster, clusterService *appsv1.ClusterService) ([]*appsv1.ClusterService, error) {
	if len(clusterService.ShardingSelector) == 0 || len(cluster.Spec.ShardingSpecs) == 0 {
		return []*appsv1.ClusterService{clusterService}, nil
	}

	shardingName := ""
	shardingCompSpecs := make([]*appsv1.ClusterComponentSpec, 0)
	for k, v := range transCtx.ShardingComponentSpecs {
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

func (t *clusterServiceTransformer) builtinSelector(cluster *appsv1.Cluster) map[string]string {
	selectors := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppInstanceLabelKey:  cluster.Name,
	}
	return selectors
}

func (t *clusterServiceTransformer) checkComponent(transCtx *clusterTransformContext, clusterService *appsv1.ClusterService) (*appsv1.ComponentDefinition, error) {
	compName := clusterService.ComponentSelector
	for _, compSpec := range transCtx.ComponentSpecs {
		if compSpec.Name == compName {
			compDef, ok := transCtx.ComponentDefs[compSpec.ComponentDef]
			if !ok {
				return nil, fmt.Errorf("the component definition of service selector is not defined, service: %s, component: %s", clusterService.Name, compName)
			}
			return compDef, nil
		}
	}
	return nil, fmt.Errorf("the component of service selector is not exist, service: %s, component: %s", clusterService.Name, compName)
}

func (t *clusterServiceTransformer) checkComponentRoles(compDef *appsv1.ComponentDefinition, clusterService *appsv1.ClusterService) error {
	definedRoles := make(map[string]bool)
	for _, role := range compDef.Spec.Roles {
		definedRoles[strings.ToLower(role.Name)] = true
	}
	if !definedRoles[strings.ToLower(clusterService.RoleSelector)] {
		return fmt.Errorf("role selector for service is not defined, service: %s, role: %s", clusterService.Name, clusterService.RoleSelector)
	}
	return nil
}

func (t *clusterServiceTransformer) listOwnedClusterServices(transCtx *clusterTransformContext,
	cluster *appsv1.Cluster) (map[string]*corev1.Service, error) {
	svcList := &corev1.ServiceList{}
	labels := client.MatchingLabels(constant.GetClusterLabels(cluster.Name))
	if err := transCtx.Client.List(transCtx.Context, svcList, labels, client.InNamespace(cluster.Namespace)); err != nil {
		return nil, err
	}

	services := make(map[string]*corev1.Service)
	for i, svc := range svcList.Items {
		if model.IsOwnerOf(cluster, &svc) {
			services[svc.Name] = &svcList.Items[i]
		}
	}
	return services, nil
}

func createOrUpdateService(ctx graph.TransformContext, dag *graph.DAG, graphCli model.GraphClient, service *corev1.Service, owner client.Object) error {
	key := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	obj := &corev1.Service{}
	if err := ctx.GetClient().Get(ctx.GetContext(), key, obj, inDataContext4C()); err != nil {
		if apierrors.IsNotFound(err) {
			graphCli.Create(dag, service, inDataContext4G())
			return nil
		}
		return err
	}

	// don't update service not owned by the owner, to keep compatible with existed cluster
	if owner != nil && !model.IsOwnerOf(owner, obj) {
		return nil
	}

	objCopy := obj.DeepCopy()
	objCopy.Spec = service.Spec

	ctrlutil.MergeMetadataMapInplace(service.Labels, &objCopy.Labels)
	ctrlutil.MergeMetadataMapInplace(service.Annotations, &objCopy.Annotations)

	resolveServiceDefaultFields(&obj.Spec, &objCopy.Spec)

	if !reflect.DeepEqual(obj, objCopy) {
		graphCli.Update(dag, obj, objCopy, inDataContext4G())
	}
	return nil
}

func resolveServiceDefaultFields(obj, objCopy *corev1.ServiceSpec) {
	// TODO: how about the order changed?
	for i, port := range objCopy.Ports {
		if i == len(obj.Ports) {
			break
		}
		// if the service type is NodePort or LoadBalancer, and the nodeport is not set, we should use the nodeport of the exist service
		if (objCopy.Type == corev1.ServiceTypeNodePort || objCopy.Type == corev1.ServiceTypeLoadBalancer) && port.NodePort == 0 && obj.Ports[i].NodePort != 0 {
			objCopy.Ports[i].NodePort = obj.Ports[i].NodePort
		}
		if port.TargetPort.IntVal != 0 {
			continue
		}
		port.TargetPort = obj.Ports[i].TargetPort
		if reflect.DeepEqual(port, obj.Ports[i]) {
			objCopy.Ports[i].TargetPort = obj.Ports[i].TargetPort
		}
	}
	if len(objCopy.ClusterIP) == 0 {
		objCopy.ClusterIP = obj.ClusterIP
	}
	if len(objCopy.ClusterIPs) == 0 {
		objCopy.ClusterIPs = obj.ClusterIPs
	}
	if len(objCopy.Type) == 0 {
		objCopy.Type = obj.Type
	}
	if len(objCopy.SessionAffinity) == 0 {
		objCopy.SessionAffinity = obj.SessionAffinity
	}
	if len(objCopy.IPFamilies) == 0 || (len(objCopy.IPFamilies) == 1 && *objCopy.IPFamilyPolicy != corev1.IPFamilyPolicySingleStack) {
		objCopy.IPFamilies = obj.IPFamilies
	}
	if objCopy.IPFamilyPolicy == nil {
		objCopy.IPFamilyPolicy = obj.IPFamilyPolicy
	}
	if objCopy.InternalTrafficPolicy == nil {
		objCopy.InternalTrafficPolicy = obj.InternalTrafficPolicy
	}
	if objCopy.ExternalTrafficPolicy == "" && obj.ExternalTrafficPolicy != "" {
		objCopy.ExternalTrafficPolicy = obj.ExternalTrafficPolicy
	}
}

// func checkLegacyServiceExist(ctx graph.TransformContext, serviceName, namespace string) (bool, error) {
//	key := types.NamespacedName{
//		Namespace: namespace,
//		Name:      serviceName,
//	}
//	obj := &corev1.Service{}
//	if err := ctx.GetClient().Get(ctx.GetContext(), key, obj); err != nil {
//		if apierrors.IsNotFound(err) {
//			return false, nil
//		}
//		return false, err
//	}
//	return true, nil
// }

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
