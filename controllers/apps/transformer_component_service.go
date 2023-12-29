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
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
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
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create service related objects", "component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	cluster := transCtx.Cluster
	synthesizeComp := transCtx.SynthesizeComponent
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, service := range synthesizeComp.ComponentServices {
		// component controller does not handle the default headless service; the default headless service is managed by the RSM.
		if t.skipDefaultHeadlessSvc(synthesizeComp, &service) {
			continue
		}
		// component controller does not handle the nodeport service if the feature gate is not enabled.
		if t.skipNodePortService(synthesizeComp, &service) {
			continue
		}
		// component controller does not handle the pod ordinal service if the feature gate is not enabled.
		if t.skipPodOrdinalService(synthesizeComp, &service) {
			continue
		}

		genServices, err := t.genMultiServicesIfNeed(cluster, synthesizeComp, &service)
		if err != nil {
			return err
		}
		for _, genService := range genServices {
			svc, err := t.buildService(transCtx.Component, synthesizeComp, genService)
			if err != nil {
				return err
			}
			if err = createOrUpdateService(ctx, dag, graphCli, svc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *componentServiceTransformer) genMultiServicesIfNeed(cluster *appsv1alpha1.Cluster,
	synthesizeComp *component.SynthesizedComponent, compService *appsv1alpha1.ComponentService) ([]*appsv1alpha1.ComponentService, error) {
	if !compService.GeneratePodOrdinalService {
		serviceName := constant.GenerateComponentServiceName(cluster.Name, synthesizeComp.Name, compService.ServiceName)
		compService.ServiceName = serviceName
		return []*appsv1alpha1.ComponentService{compService}, nil
	}

	podOrdinalServices := make([]*appsv1alpha1.ComponentService, 0, synthesizeComp.Replicas)
	for i := int32(0); i < synthesizeComp.Replicas; i++ {
		svc := compService.DeepCopy()
		svc.Name = fmt.Sprintf("%s-%d", compService.Name, i)
		serviceNamePrefix := constant.GenerateClusterComponentName(cluster.Name, synthesizeComp.Name)
		if len(compService.ServiceName) == 0 {
			svc.ServiceName = fmt.Sprintf("%s-%d", serviceNamePrefix, i)
		} else {
			svc.ServiceName = fmt.Sprintf("%s-%s-%d", serviceNamePrefix, compService.ServiceName, i)
		}
		if svc.Spec.Selector == nil {
			svc.Spec.Selector = make(map[string]string)
		}
		// TODO(xingran): use StatefulSet's podName as default selector to select unique pod
		svc.Spec.Selector[constant.StatefulSetPodNameLabelKey] = constant.GeneratePodName(cluster.Name, synthesizeComp.Name, int(i))
		podOrdinalServices = append(podOrdinalServices, svc)
	}

	return podOrdinalServices, nil
}

func (t *componentServiceTransformer) skipNodePortService(synthesizeComp *component.SynthesizedComponent, compService *appsv1alpha1.ComponentService) bool {
	if compService == nil {
		return true
	}
	if compService.Spec.Type != corev1.ServiceTypeNodePort {
		return false
	}
	if synthesizeComp.ComponentDefFeatureGate == nil || !synthesizeComp.ComponentDefFeatureGate.NodePort {
		return true
	}
	return false
}

func (t *componentServiceTransformer) skipPodOrdinalService(synthesizeComp *component.SynthesizedComponent, compService *appsv1alpha1.ComponentService) bool {
	if compService == nil {
		return true
	}
	if !compService.GeneratePodOrdinalService {
		return false
	}
	if synthesizeComp.ComponentDefFeatureGate == nil || !synthesizeComp.ComponentDefFeatureGate.PodOrdinalService {
		return true
	}
	return false
}

func (t *componentServiceTransformer) buildService(comp *appsv1alpha1.Component,
	synthesizeComp *component.SynthesizedComponent, service *appsv1alpha1.ComponentService) (*corev1.Service, error) {
	var (
		namespace   = synthesizeComp.Namespace
		clusterName = synthesizeComp.ClusterName
		compName    = synthesizeComp.Name
	)

	labels := constant.GetComponentWellKnownLabels(clusterName, compName)
	builder := builder.NewServiceBuilder(namespace, service.ServiceName).
		AddLabelsInMap(labels).
		SetSpec(&service.Spec).
		AddSelectorsInMap(t.builtinSelector(comp)).
		Optimize4ExternalTraffic()

	if len(service.RoleSelector) > 0 && !service.GeneratePodOrdinalService {
		if err := t.checkRoleSelector(synthesizeComp, service.Name, service.RoleSelector); err != nil {
			return nil, err
		}
		builder.AddSelector(constant.RoleLabelKey, service.RoleSelector)
	}
	return builder.GetObject(), nil
}

func (t *componentServiceTransformer) builtinSelector(comp *appsv1alpha1.Component) map[string]string {
	selectors := map[string]string{
		constant.AppManagedByLabelKey:   "",
		constant.AppInstanceLabelKey:    "",
		constant.KBAppComponentLabelKey: "",
	}
	for _, key := range maps.Keys(selectors) {
		if val, ok := comp.Labels[key]; ok {
			selectors[key] = val
		}
	}
	return selectors
}

func (t *componentServiceTransformer) checkRoleSelector(synthesizeComp *component.SynthesizedComponent,
	name string, roleSelector string) error {
	definedRoles := make(map[string]bool)
	for _, role := range synthesizeComp.Roles {
		definedRoles[strings.ToLower(role.Name)] = true
	}
	if !definedRoles[strings.ToLower(roleSelector)] {
		return fmt.Errorf("role selector for service is not defined, service: %s, role: %s", name, roleSelector)
	}
	return nil
}

func (t *componentServiceTransformer) skipDefaultHeadlessSvc(synthesizeComp *component.SynthesizedComponent, service *appsv1alpha1.ComponentService) bool {
	svcName := constant.GenerateComponentServiceName(synthesizeComp.ClusterName, synthesizeComp.Name, service.ServiceName)
	defaultHeadlessSvcName := constant.GenerateDefaultComponentHeadlessServiceName(synthesizeComp.ClusterName, synthesizeComp.Name)
	return svcName == defaultHeadlessSvcName
}
