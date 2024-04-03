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
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/apecloud/kubeblocks/pkg/constant"
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

	if !isHostNetworkEnabled(transCtx) {
		return nil
	}

	synthesizedComp := transCtx.SynthesizeComponent
	ports, err := allocateHostPorts(synthesizedComp)
	if err != nil {
		return err
	}
	return updateObjectsWithAllocatedPorts(synthesizedComp, ports)
}

func isHostNetworkEnabled(transCtx *componentTransformContext) bool {
	synthesizedComp := transCtx.SynthesizeComponent
	if synthesizedComp.HostNetwork == nil {
		return false
	}
	// legacy definition, ignore the cluster annotations
	if synthesizedComp.PodSpec.HostNetwork {
		return true
	}
	// TODO: use component.annotations
	cluster := transCtx.Cluster
	if cluster.Annotations == nil {
		return false
	}
	comps, ok := cluster.Annotations[constant.HostNetworkAnnotationKey]
	if !ok {
		return false
	}
	return slices.Index(strings.Split(comps, ","), synthesizedComp.Name) >= 0
}

func allocateHostPorts(synthesizedComp *component.SynthesizedComponent) (map[string]map[string]int32, error) {
	ports := map[string]map[string]bool{}
	for _, c := range synthesizedComp.HostNetwork.ContainerPorts {
		containerPorts := map[string]bool{}
		for _, p := range c.Ports {
			containerPorts[p] = true
		}
		ports[c.Container] = containerPorts
	}

	pm := intctrlutil.GetPortManager()
	needAllocate := func(c string, p string, _ int32) bool {
		containerPorts, ok := ports[c]
		if !ok {
			return false
		}
		return containerPorts[p]
	}
	return allocateHostPortsWithFunc(pm, synthesizedComp, needAllocate)
}

func allocateHostPortsWithFunc(pm *intctrlutil.PortManager, synthesizedComp *component.SynthesizedComponent,
	needAllocate func(string, string, int32) bool) (map[string]map[string]int32, error) {
	ports := map[string]map[string]int32{}
	for _, c := range synthesizedComp.PodSpec.Containers {
		for _, p := range c.Ports {
			portKey := intctrlutil.BuildHostPortName(synthesizedComp.ClusterName, synthesizedComp.Name, c.Name, p.Name)
			if needAllocate(c.Name, p.Name, p.ContainerPort) {
				port, err := pm.AllocatePort(portKey)
				if err != nil {
					return nil, err
				}
				ports[c.Name][p.Name] = port
			} else {
				if err := pm.UsePort(portKey, p.ContainerPort); err != nil {
					return nil, err
				}
			}
		}
	}
	return ports, nil
}

func updateObjectsWithAllocatedPorts(synthesizedComp *component.SynthesizedComponent, ports map[string]map[string]int32) error {
	synthesizedComp.PodSpec.HostNetwork = true
	if synthesizedComp.HostNetwork != nil && synthesizedComp.HostNetwork.DNSPolicy != nil {
		synthesizedComp.PodSpec.DNSPolicy = *synthesizedComp.HostNetwork.DNSPolicy
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
	if err := updateMonitorPorts(synthesizedComp, ports); err != nil {
		return err
	}
	if err := updateLorrySpecAfterPortsChanged(synthesizedComp); err != nil {
		return err
	}
	return nil
}

func updateMonitorPorts(synthesizedComp *component.SynthesizedComponent, ports map[string]map[string]int32) error {
	if !synthesizedComp.Monitor.Enable {
		return nil
	}

	portMapping := make(map[int32]int32)
	for _, c := range synthesizedComp.PodSpec.Containers {
		containerPorts, ok := ports[c.Name]
		for _, p := range c.Ports {
			portMapping[p.ContainerPort] = func() int32 {
				if ok {
					port, okk := containerPorts[p.Name]
					if okk {
						return port
					}
				}
				return p.ContainerPort
			}()
		}
	}

	newScrapePort, ok := portMapping[synthesizedComp.Monitor.ScrapePort]
	if !ok {
		return fmt.Errorf("monitor scrape port %d not found", synthesizedComp.Monitor.ScrapePort)
	}
	synthesizedComp.Monitor.ScrapePort = newScrapePort
	return nil
}

func updateLorrySpecAfterPortsChanged(synthesizeComp *component.SynthesizedComponent) error {
	lorryContainer := intctrlutil.GetLorryContainer(synthesizeComp.PodSpec.Containers)
	if lorryContainer == nil {
		return nil
	}

	lorryHTTPPort := getLorryHTTPPort(lorryContainer)
	lorryGRPCPort := getLorryGRPCPort(lorryContainer)
	if err := updateLorry(synthesizeComp, lorryContainer, lorryHTTPPort, lorryGRPCPort); err != nil {
		return err
	}

	if err := updateReadinessProbe(synthesizeComp, lorryHTTPPort); err != nil {
		return err
	}
	return nil
}

func updateLorry(synthesizeComp *component.SynthesizedComponent, container *corev1.Container, httpPort, grpcPort int) error {
	container.Command = []string{"lorry",
		"--port", strconv.Itoa(httpPort),
		"--grpcport", strconv.Itoa(grpcPort),
	}

	if container.StartupProbe != nil && container.StartupProbe.TCPSocket != nil {
		container.StartupProbe.TCPSocket.Port = intstr.FromInt(httpPort)
	}

	for i := range container.Env {
		if container.Env[i].Name != constant.KBEnvServicePort {
			continue
		}
		if len(synthesizeComp.PodSpec.Containers) > 0 {
			mainContainer := synthesizeComp.PodSpec.Containers[0]
			if len(mainContainer.Ports) > 0 {
				port := mainContainer.Ports[0]
				dbPort := port.ContainerPort
				container.Env[i] = corev1.EnvVar{
					Name:      constant.KBEnvServicePort,
					Value:     strconv.Itoa(int(dbPort)),
					ValueFrom: nil,
				}
			}
		}
	}
	return nil
}

func updateReadinessProbe(synthesizeComp *component.SynthesizedComponent, lorryHTTPPort int) error {
	var container *corev1.Container
	for i := range synthesizeComp.PodSpec.Containers {
		container = &synthesizeComp.PodSpec.Containers[i]
		if container.ReadinessProbe == nil {
			continue
		}
		if container.ReadinessProbe.HTTPGet == nil {
			continue
		}
		if container.ReadinessProbe.HTTPGet.Path == constant.LorryRoleProbePath ||
			container.ReadinessProbe.HTTPGet.Path == constant.LorryVolumeProtectPath {
			container.ReadinessProbe.HTTPGet.Port = intstr.FromInt(lorryHTTPPort)
		}
	}
	return nil
}

func getLorryHTTPPort(container *corev1.Container) int {
	for _, port := range container.Ports {
		if port.Name == constant.LorryHTTPPortName {
			return int(port.ContainerPort)
		}
	}
	return 0
}

func getLorryGRPCPort(container *corev1.Container) int {
	for _, port := range container.Ports {
		if port.Name == constant.LorryGRPCPortName {
			return int(port.ContainerPort)
		}
	}
	return 0
}
