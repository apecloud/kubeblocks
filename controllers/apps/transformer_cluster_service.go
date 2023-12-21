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

	for _, svc := range cluster.Spec.Services {
		genServices, err := t.genMultiServicesIfNeed(transCtx, cluster, &svc)
		if err != nil {
			return err
		}
		for _, genSvc := range genServices {
			service, err := t.buildService(transCtx, cluster, genSvc)
			if err != nil {
				return err
			}
			if err = createOrUpdateService(ctx, dag, graphCli, service); err != nil {
				return err
			}
			delete(services, service.Name)
		}
	}

	for svc := range services {
		graphCli.Delete(dag, services[svc])
	}

	return nil
}

func (t *clusterServiceTransformer) genMultiServicesIfNeed(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster, service *appsv1alpha1.Service) ([]*appsv1alpha1.Service, error) {
	if !service.GeneratePodOrdinalService {
		serviceName := constant.GenerateClusterServiceName(cluster.Name, service.ServiceName)
		service.ServiceName = serviceName
		return []*appsv1alpha1.Service{service}, nil
	}

	if len(service.ComponentSelector) == 0 {
		return nil, fmt.Errorf("the componentSelector of service is required when generatePodOrdinalService is true, service: %s", service.Name)
	}

	compName := ""
	compReplicas := int32(0)
	for _, compSpec := range transCtx.ComponentSpecs {
		if compSpec.Name == service.ComponentSelector {
			compName = service.ComponentSelector
			compReplicas = compSpec.Replicas
			break
		}
	}

	if len(compName) == 0 {
		return nil, fmt.Errorf("the componentSelector does not exist, service: %s, componentSelector: %s", service.Name, service.ComponentSelector)
	}

	podOrdinalServices := make([]*appsv1alpha1.Service, 0, compReplicas)
	for i := int32(0); i < compReplicas; i++ {
		svc := service.DeepCopy()
		svc.Name = fmt.Sprintf("%s-%d", service.Name, i)
		serviceNamePrefix := constant.GenerateClusterComponentName(cluster.Name, compName)
		if len(service.ServiceName) == 0 {
			svc.ServiceName = fmt.Sprintf("%s-%d", serviceNamePrefix, i)
		} else {
			svc.ServiceName = fmt.Sprintf("%s-%s-%d", serviceNamePrefix, service.ServiceName, i)
		}
		if svc.Spec.Selector == nil {
			svc.Spec.Selector = make(map[string]string)
		}
		// TODO(xingran): use StatefulSet's podName as default selector to select unique pod
		svc.Spec.Selector[constant.StatefulSetPodNameLabelKey] = constant.GeneratePodName(cluster.Name, compName, int(i))
		podOrdinalServices = append(podOrdinalServices, svc)
	}

	return podOrdinalServices, nil
}

func (t *clusterServiceTransformer) buildService(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster, service *appsv1alpha1.Service) (*corev1.Service, error) {
	var (
		namespace   = cluster.Namespace
		clusterName = cluster.Name
	)

	builder := builder.NewServiceBuilder(namespace, service.ServiceName).
		AddLabelsInMap(constant.GetClusterWellKnownLabels(clusterName)).
		SetSpec(&service.Spec).
		AddSelectorsInMap(t.builtinSelector(cluster)).
		Optimize4ExternalTraffic()

	if len(service.ComponentSelector) > 0 {
		compDef, err := t.checkComponent(transCtx, service)
		if err != nil {
			return nil, err
		}
		builder.AddSelector(constant.KBAppComponentLabelKey, service.ComponentSelector)

		if len(service.RoleSelector) > 0 && !service.GeneratePodOrdinalService {
			if err := t.checkComponentRoles(compDef, service); err != nil {
				return nil, err
			}
			builder.AddSelector(constant.RoleLabelKey, service.RoleSelector)
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

func (t *clusterServiceTransformer) checkComponent(transCtx *clusterTransformContext, service *appsv1alpha1.Service) (*appsv1alpha1.ComponentDefinition, error) {
	compName := service.ComponentSelector
	for _, comp := range transCtx.ComponentSpecs {
		if comp.Name == compName {
			compDef, ok := transCtx.ComponentDefs[comp.ComponentDef]
			if !ok {
				return nil, fmt.Errorf("the component definition of service selector is not defined, service: %s, component: %s", service.Name, compName)
			}
			return compDef, nil
		}
	}
	return nil, fmt.Errorf("the component of service selector is not exist, service: %s, component: %s", service.Name, compName)
}

func (t *clusterServiceTransformer) checkComponentRoles(compDef *appsv1alpha1.ComponentDefinition, service *appsv1alpha1.Service) error {
	definedRoles := make(map[string]bool)
	for _, role := range compDef.Spec.Roles {
		definedRoles[strings.ToLower(role.Name)] = true
	}
	if !definedRoles[strings.ToLower(service.RoleSelector)] {
		return fmt.Errorf("role selector for service is not defined, service: %s, role: %s", service.Name, service.RoleSelector)
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

func createOrUpdateService(ctx graph.TransformContext, dag *graph.DAG, graphCli model.GraphClient, service *corev1.Service) error {
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
}
