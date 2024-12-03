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
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type componentHostNetworkTransformer struct{}

var _ graph.Transformer = &componentHostNetworkTransformer{}

func (t *componentHostNetworkTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
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
	return updateObjectsWithAllocatedPorts(synthesizedComp, ports)
}

func allocateHostPorts(synthesizedComp *component.SynthesizedComponent) (map[string]map[string]int32, error) {
	// container -> portName -> allocateRequired
	ports := map[string]map[string]bool{}

	for _, c := range synthesizedComp.PodSpec.Containers {
		ports[c.Name] = make(map[string]bool)
		for _, p := range c.Ports {
			ports[c.Name][p.Name] = false
		}
	}

	for _, c := range synthesizedComp.HostNetwork.ContainerPorts {
		if containerPorts, exists := ports[c.Container]; exists {
			for _, p := range c.Ports {
				containerPorts[p] = true
			}
		}
	}

	pm := intctrlutil.GetPortManager()
	needAllocate := func(containerName string, portName string) bool {
		if containerPorts, ok := ports[containerName]; ok {
			return containerPorts[portName]
		}
		return false
	}

	return allocateHostPortsWithFunc(pm, synthesizedComp, needAllocate)
}

func allocateHostPortsWithFunc(pm *intctrlutil.PortManager, synthesizedComp *component.SynthesizedComponent,
	needAllocate func(string, string) bool) (map[string]map[string]int32, error) {
	// container -> portName -> port
	ports := map[string]map[string]int32{}

	insert := func(containerName, portName string, portValue int32) {
		if _, ok := ports[containerName]; !ok {
			ports[containerName] = map[string]int32{}
		}
		ports[containerName][portName] = portValue
	}

	for _, container := range synthesizedComp.PodSpec.Containers {
		for _, port := range container.Ports {
			portKey := intctrlutil.BuildHostPortName(
				synthesizedComp.ClusterName,
				synthesizedComp.Name,
				container.Name,
				port.Name,
			)

			if needAllocate(container.Name, port.Name) {
				allocatedPort, err := pm.AllocatePort(portKey)
				if err != nil {
					return nil, fmt.Errorf("failed to allocate port for container %s, port %s: %v",
						container.Name, port.Name, err)
				}
				insert(container.Name, port.Name, allocatedPort)
			} else {
				if err := pm.UsePort(portKey, port.ContainerPort); err != nil {
					return nil, fmt.Errorf("failed to use port for container %s, port %s: %v",
						container.Name, port.Name, err)
				}
			}
		}
	}
	return ports, nil
}

func updateObjectsWithAllocatedPorts(synthesizedComp *component.SynthesizedComponent, ports map[string]map[string]int32) error {
	synthesizedComp.PodSpec.HostNetwork = true
	synthesizedComp.PodSpec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
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
	return nil
}
