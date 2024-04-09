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

type componentSidecarContainerTransformer struct{}

var _ graph.Transformer = &componentSidecarContainerTransformer{}

func (c componentSidecarContainerTransformer) Transform(ctx graph.TransformContext, _ *graph.DAG) (err error) {
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

	containers := injectSidecarContainers(compDef, synthesizeComp)
	injectHostNetwork(transCtx, synthesizeComp, containers)
	return
}

func injectHostNetwork(transCtx *componentTransformContext, synthesizeComp *component.SynthesizedComponent, containers []corev1.Container) {
	if len(containers) == 0 || !isHostNetworkEnabled(transCtx) {
		return
	}

	for _, container := range containers {
		if len(container.Ports) > 0 {
			synthesizeComp.HostNetwork.ContainerPorts = append(
				synthesizeComp.HostNetwork.ContainerPorts,
				appsv1alpha1.HostNetworkContainerPort{
					Container: container.Name,
					Ports:     buildHostNetworkPortsFromContainer(container.Ports),
				})
		}
	}
}

func buildHostNetworkPortsFromContainer(containerPorts []corev1.ContainerPort) []string {
	var ports []string
	for _, port := range containerPorts {
		ports = append(ports, port.Name)
	}
	return ports
}

func injectSidecarContainers(compDef *appsv1alpha1.ComponentDefinition, synthesizeComp *component.SynthesizedComponent) []corev1.Container {
	if len(synthesizeComp.Sidecars) == 0 {
		return nil
	}

	var containers []corev1.Container
	for _, sidecar := range compDef.Spec.SidecarContainerSpecs {
		if !slices.Contains(synthesizeComp.Sidecars, sidecar.Name) {
			continue
		}
		containers = append(containers, sidecar.Container)
	}

	// replace containers env default credential placeholder
	replacedEnvs := component.GetEnvReplacementMapForConnCredential(synthesizeComp.ClusterName)
	for _, c := range containers {
		c.Env = component.ReplaceSecretEnvVars(replacedEnvs, c.Env)
	}
	synthesizeComp.PodSpec.Containers = append(synthesizeComp.PodSpec.Containers, containers...)
	return containers
}
