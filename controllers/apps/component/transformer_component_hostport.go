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
		for i, c := range synthesizedComp.PodSpec.Containers {
			for j, p := range c.Ports {
				if hostPort, ok := ports[p.Name]; ok {
					synthesizedComp.PodSpec.Containers[i].Ports[j].HostPort = hostPort
				}
			}
		}
	}
	return nil
}
