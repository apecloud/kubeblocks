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

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

const (
	// TODO[ziang]: get the node port range from the controller-manager
	NodePortMin = 30000
	NodePortMax = 32767
)

// clusterServiceTransformer handles cluster services.
type clusterServiceTransformer struct {
}

var _ graph.Transformer = &clusterServiceTransformer{}

func (t *clusterServiceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	services, err := t.listOwnedClusterServices(transCtx, cluster)
	if err != nil {
		return err
	}

	npServices, err := listAllNodePortServices(transCtx)
	if err != nil {
		return err
	}
	npAllocator := NewPortAllocator(NodePortMin, NodePortMax, npServices)

	for _, svc := range cluster.Spec.Services {
		service, err := t.buildService(transCtx, cluster, &svc)
		if err != nil {
			return err
		}
		if err = createOrUpdateService(ctx, dag, graphCli, service, npAllocator); err != nil {
			return err
		}
		delete(services, service.Name)
	}

	for svc := range services {
		graphCli.Delete(dag, services[svc])
	}

	return nil
}

func (t *clusterServiceTransformer) buildService(transCtx *clusterTransformContext,
	cluster *appsv1alpha1.Cluster, service *appsv1alpha1.Service) (*corev1.Service, error) {
	var (
		namespace   = cluster.Namespace
		clusterName = cluster.Name
	)

	serviceName := constant.GenerateClusterServiceName(clusterName, service.ServiceName)
	builder := builder.NewServiceBuilder(namespace, serviceName).
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

		if len(service.RoleSelector) > 0 {
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
		constant.AppManagedByLabelKey: "",
		constant.AppInstanceLabelKey:  "",
	}
	for _, key := range maps.Keys(selectors) {
		if val, ok := cluster.Labels[key]; ok {
			selectors[key] = val
		}
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

func createOrUpdateService(ctx graph.TransformContext, dag *graph.DAG, graphCli model.GraphClient, service *corev1.Service, npAllocator *PortAllocator) error {
	key := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	obj := &corev1.Service{}
	if err := ctx.GetClient().Get(ctx.GetContext(), key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			if err := resolveNodePortForSvc(service, npAllocator); err != nil {
				return err
			}
			graphCli.Create(dag, service)
			return nil
		}
		return err
	}
	objCopy := obj.DeepCopy()
	objCopy.Spec = service.Spec

	resolveServiceDefaultFields(&obj.Spec, &objCopy.Spec)

	if err := resolveNodePortForSvc(objCopy, npAllocator); err != nil {
		return err
	}

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

func resolveNodePortForSvc(obj *corev1.Service, npAllocator *PortAllocator) error {
	if !isExternalService(&obj.Spec) {
		return nil
	}

	getKey := func(svc *corev1.Service) types.NamespacedName {
		return types.NamespacedName{
			Namespace: svc.Namespace,
			Name:      svc.Name,
		}
	}

	for _, item := range obj.Spec.Ports {
		if item.NodePort != 0 {
			svc, inUse := npAllocator.InUse(item.NodePort)
			if !inUse || getKey(svc) == getKey(obj) {
				npAllocator.UsePort(item.NodePort, obj)
				continue
			}
			return fmt.Errorf("node port %d is already in use by service %s", item.NodePort, svc.Name)
		}
		port, err := npAllocator.AllocatePort(obj)
		if err != nil {
			return err
		}
		item.NodePort = port
	}
	return nil
}

// TODO[ziang]: may be we can use a configmap to record the ports allocated
type PortAllocator struct {
	min    int32
	max    int32
	cursor int32
	used   map[int32]*corev1.Service
}

func NewPortAllocator(min, max int32, used map[int32]*corev1.Service) *PortAllocator {
	return &PortAllocator{
		min:    min,
		max:    max,
		used:   used,
		cursor: min,
	}
}

func (p *PortAllocator) AllocatePort(obj *corev1.Service) (int32, error) {
	result, err := p.AllocatePorts(obj, 1)
	if err != nil {
		return 0, err
	}
	return result[0], nil
}

func (p *PortAllocator) AllocatePorts(obj *corev1.Service, count int32) ([]int32, error) {
	cursor := p.cursor
	result := make([]int32, count)
	for ; cursor <= p.max && int32(len(result)) < count; {
		if _, ok := p.used[cursor]; ok {
			continue
		}
		result = append(result, cursor)
	}

	if int32(len(result)) != count {
		return nil, fmt.Errorf("not enough node ports")
	}

	p.cursor = cursor
	for _, port := range result {
		p.used[port] = obj
	}

	return result, nil
}

func (p *PortAllocator) InUse(port int32) (*corev1.Service, bool) {
	svc, ok := p.used[port]
	return svc, ok
}

func (p *PortAllocator) UsePort(port int32, obj *corev1.Service) {
	p.used[port] = obj
}

func isExternalService(obj *corev1.ServiceSpec) bool {
	return obj.Type == corev1.ServiceTypeNodePort || obj.Type == corev1.ServiceTypeLoadBalancer
}
