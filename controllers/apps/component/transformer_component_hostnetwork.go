/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type componentHostNetworkTransformer struct{}

var _ graph.Transformer = &componentHostNetworkTransformer{}

func (t *componentHostNetworkTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if isCompDeleting(transCtx.ComponentOrig) {
		return nil
	}

	if !component.IsHostNetworkEnabled(transCtx.SynthesizeComponent) {
		return nil
	}

	synthesizedComp := transCtx.SynthesizeComponent
	ports, err := allocateHostPorts(synthesizedComp)
	if err != nil {
		return err
	}

	comp := transCtx.Component
	updateObjectsWithAllocatedPorts(synthesizedComp, comp, ports)

	return nil
}

func allocateHostPorts(synthesizedComp *component.SynthesizedComponent) (map[string]map[string]int32, error) {
	ports := map[string]map[string]bool{}
	for _, c := range synthesizedComp.HostNetwork.ContainerPorts {
		var originalContainer *corev1.Container
		for _, container := range synthesizedComp.PodSpec.Containers {
			if container.Name == c.Container {
				originalContainer = &container
				break
			}
		}
		for _, p := range c.Ports {
			if _, ok := ports[c.Container]; !ok {
				ports[c.Container] = map[string]bool{}
			}
			var originalPort *corev1.ContainerPort
			if originalContainer != nil {
				for _, port := range originalContainer.Ports {
					if port.Name == p {
						originalPort = &port
						break
					}
				}
			}
			if originalPort != nil && originalPort.HostPort > 0 {
				ports[c.Container][p] = false
			} else {
				ports[c.Container][p] = true
			}
		}
	}

	pm := intctrlutil.GetPortManager()
	needAllocate := func(c string, p string) bool {
		containerPorts, ok := ports[c]
		if !ok {
			return false
		}
		return containerPorts[p]
	}
	return allocateHostPortsWithFunc(pm, synthesizedComp, needAllocate)
}

func allocateHostPortsWithFunc(pm *intctrlutil.PortManager, synthesizedComp *component.SynthesizedComponent,
	needAllocate func(string, string) bool) (map[string]map[string]int32, error) {
	ports := map[string]map[string]int32{}
	insert := func(c, pk string, pv int32) {
		if _, ok := ports[c]; !ok {
			ports[c] = map[string]int32{}
		}
		ports[c][pk] = pv
	}
	for _, c := range synthesizedComp.PodSpec.Containers {
		for _, p := range c.Ports {
			portKey := intctrlutil.BuildHostPortName(synthesizedComp.ClusterName, synthesizedComp.Name, c.Name, p.Name)
			if needAllocate(c.Name, p.Name) {
				port, err := pm.AllocatePort(portKey)
				if err != nil {
					return nil, err
				}
				insert(c.Name, p.Name, port)
			} else {
				if err := pm.UsePort(portKey, p.ContainerPort); err != nil {
					return nil, err
				}
			}
		}
	}
	return ports, nil
}

func updateObjectsWithAllocatedPorts(synthesizedComp *component.SynthesizedComponent,
	comp *appsv1.Component, ports map[string]map[string]int32) {
	synthesizedComp.PodSpec.HostNetwork = true
	if comp.Spec.Network != nil && comp.Spec.Network.DNSPolicy != nil {
		synthesizedComp.PodSpec.DNSPolicy = *comp.Spec.Network.DNSPolicy
	} else {
		synthesizedComp.PodSpec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}
	for i, c := range synthesizedComp.PodSpec.Containers {
		containerPorts, ok := ports[c.Name]
		if ok {
			for j, p := range c.Ports {
				if port, okk := containerPorts[p.Name]; okk {
					synthesizedComp.PodSpec.Containers[i].Ports[j].ContainerPort = port
				}
			}
		}
	}
	component.UpdateKBAgentContainer4HostNetwork(synthesizedComp)
}
