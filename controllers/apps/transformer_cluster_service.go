/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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

	convertedServices, err := t.convertLegacyClusterCompSpecServices(transCtx, cluster)
	if err != nil {
		return err
	}

	handleServiceFunc := func(svc *appsv1alpha1.ClusterService) error {
		service, err := t.buildService(transCtx, cluster, svc)
		if err != nil {
			return err
		}
		if err = createOrUpdateService(ctx, dag, graphCli, service, nil); err != nil {
			return err
		}
		delete(services, service.Name)
		return nil
	}

	for _, svc := range cluster.Spec.Services {
		if err = handleServiceFunc(&svc); err != nil {
			return err
		}
	}

	for _, svc := range convertedServices {
		if err = handleServiceFunc(&svc); err != nil {
			return err
		}
	}

	for svc := range services {
		graphCli.Delete(dag, services[svc])
	}

	return nil
}

// convertLegacyClusterCompSpecServices converts legacy services defined in Cluster.Spec.ComponentSpecs[x].Services to Cluster.Spec.Services.
func (t *clusterServiceTransformer) convertLegacyClusterCompSpecServices(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) ([]appsv1alpha1.ClusterService, error) {
	convertedServices := make([]appsv1alpha1.ClusterService, 0)
	for _, clusterCompSpec := range transCtx.ComponentSpecs {
		if len(clusterCompSpec.Services) == 0 {
			continue
		}

		// We only handle legacy services defined based on Cluster.Spec.ComponentSpecs[x].Services prior to version 0.8.0 of kubeblocks.
		// After kubeblocks 0.8.0 it should be defined via Cluster.Spec.Services.
		if transCtx.ClusterDef == nil || len(clusterCompSpec.ComponentDefRef) == 0 {
			continue
		}

		clusterCompDef := transCtx.ClusterDef.GetComponentDefByName(clusterCompSpec.ComponentDefRef)
		if clusterCompDef == nil {
			continue
		}
		defaultLegacyServicePorts := clusterCompDef.Service.ToSVCPorts()

		for _, item := range clusterCompSpec.Services {
			legacyService := &appsv1alpha1.ClusterService{
				Service: appsv1alpha1.Service{
					Name:        constant.GenerateClusterServiceName(cluster.Name, item.Name),
					ServiceName: constant.GenerateClusterServiceName(cluster.Name, item.Name),
					Annotations: item.Annotations,
					Spec: corev1.ServiceSpec{
						Ports: defaultLegacyServicePorts,
						Type:  item.ServiceType,
					},
				},
				ComponentSelector: clusterCompSpec.Name,
			}
			legacyServiceName := constant.GenerateComponentServiceName(cluster.Name, clusterCompSpec.Name, item.Name)
			legacyServiceExist, err := checkLegacyServiceExist(transCtx, legacyServiceName, cluster.Namespace)
			if err != nil {
				return nil, err
			}
			// the generation converted service name is different with the exist legacy service name, if the legacy service exist, we should use the legacy service name
			if legacyServiceExist {
				legacyService.Name = legacyServiceName
				legacyService.ServiceName = legacyServiceName
			}
			switch clusterCompDef.WorkloadType {
			case appsv1alpha1.Replication:
				legacyService.RoleSelector = constant.Primary
			case appsv1alpha1.Consensus:
				legacyService.RoleSelector = constant.Leader
			}
			convertedServices = append(convertedServices, *legacyService)
		}
	}
	return convertedServices, nil
}

func (t *clusterServiceTransformer) buildService(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster, clusterService *appsv1alpha1.ClusterService) (*corev1.Service, error) {
	var (
		namespace   = cluster.Namespace
		clusterName = cluster.Name
	)

	serviceName := constant.GenerateClusterServiceName(cluster.Name, clusterService.ServiceName)
	builder := builder.NewServiceBuilder(namespace, serviceName).
		AddLabelsInMap(constant.GetClusterWellKnownLabels(clusterName)).
		AddAnnotationsInMap(clusterService.Annotations).
		SetSpec(&clusterService.Spec).
		AddSelectorsInMap(t.builtinSelector(cluster)).
		Optimize4ExternalTraffic()

	if len(clusterService.ComponentSelector) > 0 {
		compDef, err := t.checkComponent(transCtx, clusterService)
		if err != nil {
			return nil, err
		}
		builder.AddSelector(constant.KBAppComponentLabelKey, clusterService.ComponentSelector)

		if len(clusterService.RoleSelector) > 0 {
			if err := t.checkComponentRoles(compDef, clusterService); err != nil {
				return nil, err
			}
			builder.AddSelector(constant.RoleLabelKey, clusterService.RoleSelector)
		}
	}
	return builder.GetObject(), nil
}

func (t *clusterServiceTransformer) builtinSelector(cluster *appsv1alpha1.Cluster) map[string]string {
	selectors := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppInstanceLabelKey:  cluster.Name,
	}
	return selectors
}

func (t *clusterServiceTransformer) checkComponent(transCtx *clusterTransformContext, clusterService *appsv1alpha1.ClusterService) (*appsv1alpha1.ComponentDefinition, error) {
	compName := clusterService.ComponentSelector
	for _, comp := range transCtx.ComponentSpecs {
		if comp.Name == compName {
			compDef, ok := transCtx.ComponentDefs[comp.ComponentDef]
			if !ok {
				return nil, fmt.Errorf("the component definition of service selector is not defined, service: %s, component: %s", clusterService.Name, compName)
			}
			return compDef, nil
		}
	}
	return nil, fmt.Errorf("the component of service selector is not exist, service: %s, component: %s", clusterService.Name, compName)
}

func (t *clusterServiceTransformer) checkComponentRoles(compDef *appsv1alpha1.ComponentDefinition, clusterService *appsv1alpha1.ClusterService) error {
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
	cluster *appsv1alpha1.Cluster) (map[string]*corev1.Service, error) {
	svcList := &corev1.ServiceList{}
	labels := client.MatchingLabels(constant.GetClusterWellKnownLabels(cluster.Name))
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
	if err := ctx.GetClient().Get(ctx.GetContext(), key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			graphCli.Create(dag, service)
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

	resolveServiceDefaultFields(&obj.Spec, &objCopy.Spec)

	if !reflect.DeepEqual(obj, objCopy) {
		graphCli.Update(dag, obj, objCopy)
	}
	return nil
}

func resolveServiceDefaultFields(obj, objCopy *corev1.ServiceSpec) {
	// TODO: how about the order changed?
	for i, port := range objCopy.Ports {
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
	if len(objCopy.IPFamilies) == 0 {
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

func checkLegacyServiceExist(ctx graph.TransformContext, serviceName, namespace string) (bool, error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      serviceName,
	}
	obj := &corev1.Service{}
	if err := ctx.GetClient().Get(ctx.GetContext(), key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
