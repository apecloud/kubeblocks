/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	"fmt"
	"reflect"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
)

type componentPrometheusIntegrationTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentPrometheusIntegrationTransformer{}

func (i componentPrometheusIntegrationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info(
			"Component is in compact mode, no need to create monitor services related objects",
			"component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	synthesizeComp := transCtx.SynthesizeComponent
	graphCli, _ := transCtx.Client.(model.GraphClient)

	if synthesizeComp.MonitorIntegration == nil {
		return nil
	}
	if err := i.buildPrometheusMonitorService(transCtx, synthesizeComp.MonitorIntegration, graphCli, dag); err != nil {
		return err
	}
	return i.buildVMMonitorService(transCtx, synthesizeComp.MonitorIntegration, graphCli)
}

func (i componentPrometheusIntegrationTransformer) buildPrometheusMonitorService(transCtx *componentTransformContext, monitorIntegration *appsv1alpha1.MonitorIntegration, graphCli model.GraphClient, dag *graph.DAG) error {
	var running *monitoringv1.ServiceMonitor

	objects, err := listMonitorServices(transCtx.GetContext(),
		i.Client,
		transCtx.Cluster.Name,
		transCtx.Component.Name,
		transCtx.Component,
		intctrlutil.MonitorServiceSignature)
	if err != nil {
		return err
	}
	if monitorIntegration.ServiceMonitorTemplate == nil {
		deleteObjects(objects, graphCli, dag)
		return nil
	}

	cmp := func(obj monitoringv1.ServiceMonitor) bool {
		return obj.Namespace == monitorIntegration.ServiceMonitorTemplate.Namespace &&
			strings.HasPrefix(obj.Name, monitorIntegration.ServiceMonitorTemplate.Name)
	}
	index := slices.IndexFunc(objects, cmp)
	if index >= 0 {
		running = objects[index].DeepCopy()
		objects = slices.Delete(objects, index, index+1)
	}

	deleteObjects(objects, graphCli, dag)
	return createOrUpdateMonitorService(transCtx, running, monitorIntegration.ServiceMonitorTemplate, graphCli, dag)
}

func createOrUpdateMonitorService(transCtx *componentTransformContext, existing *monitoringv1.ServiceMonitor, template *appsv1alpha1.ServiceMonitorTemplate, graphCli model.GraphClient, dag *graph.DAG) error {
	expected, err := createMonitorService(transCtx, template, transCtx.Component)
	if err != nil {
		return err
	}

	if existing == nil {
		graphCli.Create(dag, expected, inDataContext4G())
	}

	objCopy := existing.DeepCopy()
	objCopy.Spec = expected.Spec

	if !reflect.DeepEqual(existing, objCopy) {
		graphCli.Update(dag, existing, objCopy, inDataContext4G())
	}
	return nil
}

func createMonitorService(transCtx *componentTransformContext, template *appsv1alpha1.ServiceMonitorTemplate, owner client.Object) (*monitoringv1.ServiceMonitor, error) {
	genName := common.SimpleNameGenerator.GenerateName(template.Name)
	service := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      genName,
			Namespace: template.Namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:       transCtx.Cluster.Name,
				constant.KBAppShardingNameLabelKey: transCtx.Component.Name,
			},
		},
		Spec: template.ServiceMonitorSpec,
	}

	for key, value := range template.Labels {
		if _, ok := service.Labels[key]; !ok {
			service.Labels[key] = value
		}
	}

	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(service, owner, scheme); err != nil {
		return nil, err
	}

	return service, nil
}

func (i componentPrometheusIntegrationTransformer) buildVMMonitorService(transCtx *componentTransformContext, integration *appsv1alpha1.MonitorIntegration, graphCli model.GraphClient) error {
	// TODO: support vm operator
	if integration.VMMonitorTemplate == nil {
		return nil
	}
	return fmt.Errorf("not support vm")
}
