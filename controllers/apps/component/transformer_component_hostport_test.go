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
	"testing"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	pkgcomponent "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
)

func TestComponentHostPortTransformerAppliesToRuntimeDefinedPortsOnly(t *testing.T) {
	// Per the ComponentNetwork API contract, non-host-network hostPort
	// mappings are restricted to ports defined in
	// cmpd.spec.runtime.containers.ports: only the runtime container's own
	// declared port may receive the hostPort, never ports of injected
	// containers or ports injected after synthesis, even when they carry the
	// same name as the mapping entry.
	transCtx := &componentTransformContext{
		ComponentOrig: &appsv1.Component{},
		CompDef: &appsv1.ComponentDefinition{
			Spec: appsv1.ComponentDefinitionSpec{
				Runtime: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "elasticsearch",
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: 9200},
						},
					}},
				},
			},
		},
		SynthesizeComponent: &pkgcomponent.SynthesizedComponent{
			Network: &appsv1.ComponentNetwork{
				HostPorts: []appsv1.HostPort{
					{Name: "http", Port: 56595},
					{Name: "injected", Port: 56596},
				},
			},
			PodSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "elasticsearch",
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: 9200},
							// a port injected into the runtime container after
							// synthesis, not part of the cmpd runtime definition
							{Name: "injected", ContainerPort: 9300},
						},
					},
					{
						Name: kbagent.ContainerName,
						Ports: []corev1.ContainerPort{
							{Name: kbagent.DefaultHTTPPortName, ContainerPort: kbagent.DefaultHTTPPort},
							{Name: kbagent.DefaultStreamingPortName, ContainerPort: kbagent.DefaultStreamingPort},
						},
					},
					{
						// a non-kbagent injected sidecar reusing the port name
						Name: "metrics-sidecar",
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: 9114},
						},
					},
				},
			},
		},
	}

	if err := (&componentHostPortTransformer{}).Transform(transCtx, nil); err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	appPorts := transCtx.SynthesizeComponent.PodSpec.Containers[0].Ports
	if appPorts[0].HostPort != 56595 {
		t.Fatalf("app http HostPort = %d, want 56595", appPorts[0].HostPort)
	}
	if appPorts[1].HostPort != 0 {
		t.Fatalf("injected port on runtime container HostPort = %d, want 0", appPorts[1].HostPort)
	}

	for _, container := range transCtx.SynthesizeComponent.PodSpec.Containers[1:] {
		for _, port := range container.Ports {
			if port.HostPort != 0 {
				t.Fatalf("injected container %q port %q HostPort = %d, want 0", container.Name, port.Name, port.HostPort)
			}
		}
	}
}
