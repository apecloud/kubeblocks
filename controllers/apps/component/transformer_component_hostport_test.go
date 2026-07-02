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

func TestComponentHostPortTransformerSkipsInjectedKBAgentPorts(t *testing.T) {
	transCtx := &componentTransformContext{
		ComponentOrig: &appsv1.Component{},
		SynthesizeComponent: &pkgcomponent.SynthesizedComponent{
			Network: &appsv1.ComponentNetwork{
				HostPorts: []appsv1.HostPort{
					{Name: "http", Port: 56595},
				},
			},
			PodSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "elasticsearch",
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: 9200},
						},
					},
					{
						Name: kbagent.ContainerName,
						Ports: []corev1.ContainerPort{
							{Name: kbagent.DefaultHTTPPortName, ContainerPort: kbagent.DefaultHTTPPort},
							{Name: kbagent.DefaultStreamingPortName, ContainerPort: kbagent.DefaultStreamingPort},
						},
					},
				},
			},
		},
	}

	if err := (&componentHostPortTransformer{}).Transform(transCtx, nil); err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	appPort := transCtx.SynthesizeComponent.PodSpec.Containers[0].Ports[0]
	if appPort.HostPort != 56595 {
		t.Fatalf("app http HostPort = %d, want 56595", appPort.HostPort)
	}

	for _, port := range transCtx.SynthesizeComponent.PodSpec.Containers[1].Ports {
		if port.HostPort != 0 {
			t.Fatalf("kbagent port %q HostPort = %d, want 0", port.Name, port.HostPort)
		}
	}
}
