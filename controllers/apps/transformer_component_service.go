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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// componentServiceTransformer handles component services.
type componentServiceTransformer struct{}

var _ graph.Transformer = &componentServiceTransformer{}

func (t *componentServiceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	cctx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(cctx.ComponentOrig) {
		return nil
	}

	synthesizeComp := cctx.SynthesizeComponent
	graphCli, _ := cctx.Client.(model.GraphClient)
	for _, service := range synthesizeComp.ComponentServices {
		svc, err := t.buildService(synthesizeComp, &service)
		if err != nil {
			return err
		}
		if err = t.createOrUpdate(ctx, dag, graphCli, svc); err != nil {
			return err
		}
	}
	return nil
}

func (t *componentServiceTransformer) buildService(synthesizeComp *component.SynthesizedComponent,
	service *appsv1alpha1.ComponentService) (*corev1.Service, error) {
	var (
		namespace   = synthesizeComp.Namespace
		clusterName = synthesizeComp.ClusterName
		compName    = synthesizeComp.Name
	)

	// TODO: service.ServiceName
	serviceName := constant.GenerateComponentServiceEndpoint(clusterName, synthesizeComp.Name, string(service.ServiceName))
	labels := constant.GetComponentWellKnownLabels(clusterName, compName)
	builder := builder.NewServiceBuilder(namespace, serviceName).
		AddLabelsInMap(labels).
		SetSpec(&corev1.ServiceSpec{
			Ports:                         service.Ports,
			Selector:                      service.Selector,
			ClusterIP:                     service.ClusterIP,
			ClusterIPs:                    service.ClusterIPs,
			Type:                          service.Type,
			ExternalIPs:                   service.ExternalIPs,
			SessionAffinity:               service.SessionAffinity,
			LoadBalancerIP:                service.LoadBalancerIP,
			LoadBalancerSourceRanges:      service.LoadBalancerSourceRanges,
			ExternalName:                  service.ExternalName,
			ExternalTrafficPolicy:         service.ExternalTrafficPolicy,
			HealthCheckNodePort:           service.HealthCheckNodePort,
			PublishNotReadyAddresses:      service.PublishNotReadyAddresses,
			SessionAffinityConfig:         service.SessionAffinityConfig,
			IPFamilies:                    service.IPFamilies,
			IPFamilyPolicy:                service.IPFamilyPolicy,
			AllocateLoadBalancerNodePorts: service.AllocateLoadBalancerNodePorts,
			LoadBalancerClass:             service.LoadBalancerClass,
			InternalTrafficPolicy:         service.InternalTrafficPolicy,
		}).
		Optimize4ExternalTraffic()

	// TODO(component): role selector
	if len(service.RoleSelector) > 0 {
		if err := t.checkRoles(synthesizeComp, service.Name, service.RoleSelector); err != nil {
			return nil, err
		}
		builder.AddSelector(constant.RoleLabelKey, strings.Join(service.RoleSelector, ","))
	}
	return builder.GetObject(), nil
}

func (t *componentServiceTransformer) checkRoles(synthesizeComp *component.SynthesizedComponent,
	name string, roles []string) error {
	definedRoles := make(map[string]bool)
	for _, role := range synthesizeComp.Roles {
		definedRoles[strings.ToLower(role.Name)] = true
	}
	for _, role := range roles {
		if !definedRoles[strings.ToLower(role)] {
			return fmt.Errorf("role selector for service is not defined, service: %s, role: %s", name, role)
		}
	}
	return nil
}

func (t *componentServiceTransformer) createOrUpdate(ctx graph.TransformContext,
	dag *graph.DAG, graphCli model.GraphClient, service *corev1.Service) error {
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
	if !reflect.DeepEqual(obj, objCopy) {
		graphCli.Update(dag, obj, objCopy)
	}
	return nil
}
