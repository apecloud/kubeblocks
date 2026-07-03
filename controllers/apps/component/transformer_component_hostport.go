/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

type componentHostPortTransformer struct{}

var _ graph.Transformer = &componentHostPortTransformer{}

func (t *componentHostPortTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if isCompDeleting(transCtx.ComponentOrig) {
		return nil
	}

	synthesizedComp := transCtx.SynthesizeComponent
	if synthesizedComp == nil ||
		synthesizedComp.PodSpec.HostNetwork ||
		synthesizedComp.Network == nil ||
		synthesizedComp.Network.HostNetwork {
		return nil
	}

	ports := map[string]int32{}
	for _, hostPort := range synthesizedComp.Network.HostPorts {
		ports[hostPort.Name] = hostPort.Port
	}
	if len(ports) > 0 {
		// Per the ComponentNetwork API contract, non-host-network hostPort
		// mappings are restricted to ports defined in
		// cmpd.spec.runtime.containers.ports; ports of injected containers or
		// ports injected into runtime containers after synthesis are not
		// eligible, regardless of their names.
		runtimePorts := map[string]sets.Set[string]{}
		for _, c := range transCtx.CompDef.Spec.Runtime.Containers {
			portNames := sets.New[string]()
			for _, p := range c.Ports {
				if p.Name != "" {
					portNames.Insert(p.Name)
				}
			}
			runtimePorts[c.Name] = portNames
		}
		for i, c := range synthesizedComp.PodSpec.Containers {
			definedPorts, ok := runtimePorts[c.Name]
			if !ok {
				continue
			}
			for j, p := range c.Ports {
				if !definedPorts.Has(p.Name) {
					continue
				}
				if hostPort, ok := ports[p.Name]; ok {
					synthesizedComp.PodSpec.Containers[i].Ports[j].HostPort = hostPort
				}
			}
		}
	}
	return nil
}
