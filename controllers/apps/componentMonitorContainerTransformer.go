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
	"slices"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type componentMonitorContainerTransformer struct{}

var _ graph.Transformer = &componentMonitorContainerTransformer{}

func (c componentMonitorContainerTransformer) Transform(ctx graph.TransformContext, _ *graph.DAG) (err error) {
	transCtx, _ := ctx.(*componentTransformContext)
	compOrig := transCtx.ComponentOrig
	compDef := transCtx.CompDef
	synthesizeComp := transCtx.SynthesizeComponent

	if model.IsObjectDeleting(compOrig) {
		return
	}
	if common.IsCompactMode(compOrig.Annotations) {
		transCtx.V(1).Info(
			"Component is in compact mode, no need to inject sidecar containers to podTemplate",
			"component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return
	}

	if !synthesizeComp.MonitorEnabled {
		removeMonitorContainer(compDef.Spec.PrometheusExporter, synthesizeComp)
	}
	return
}

func removeMonitorContainer(exporter *appsv1alpha1.PrometheusExporter, synthesizeComp *component.SynthesizedComponent) {
	if exporter == nil || exporter.ContainerName == "" {
		return
	}

	_ = slices.DeleteFunc(synthesizeComp.PodSpec.Containers, func(container corev1.Container) bool {
		return container.Name == exporter.ContainerName
	})
}
