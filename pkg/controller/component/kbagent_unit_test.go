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
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestBuildKBAgentContainerUsesConfiguredImagePullPolicy(t *testing.T) {
	previous := viperx.GetString(constant.KBImagePullPolicy)
	viperx.Set(constant.KBImagePullPolicy, string(corev1.PullNever))
	t.Cleanup(func() {
		viperx.Set(constant.KBImagePullPolicy, previous)
	})

	synthesizedComp := &SynthesizedComponent{
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "mysql",
				Image: "mysql:8.0",
			}},
		},
		LifecycleActions: SynthesizedLifecycleActions{
			ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{
				PostProvision: &appsv1.Action{
					Exec: &appsv1.ExecAction{
						Image:   "custom-action-image",
						Command: []string{"echo", "hello"},
					},
				},
			},
		},
	}

	if err := buildKBAgentContainer(synthesizedComp); err != nil {
		t.Fatalf("buildKBAgentContainer() error = %v", err)
	}

	var agent, initAgent *corev1.Container
	for i := range synthesizedComp.PodSpec.Containers {
		if synthesizedComp.PodSpec.Containers[i].Name == kbagent.ContainerName {
			agent = &synthesizedComp.PodSpec.Containers[i]
			break
		}
	}
	for i := range synthesizedComp.PodSpec.InitContainers {
		if synthesizedComp.PodSpec.InitContainers[i].Name == kbagent.InitContainerName {
			initAgent = &synthesizedComp.PodSpec.InitContainers[i]
			break
		}
	}
	if agent == nil {
		t.Fatalf("kbagent container not found")
	}
	if agent.ImagePullPolicy != corev1.PullNever {
		t.Fatalf("kbagent imagePullPolicy = %q, want %q", agent.ImagePullPolicy, corev1.PullNever)
	}
	if initAgent == nil {
		t.Fatalf("init-kbagent container not found")
	}
	if initAgent.ImagePullPolicy != corev1.PullNever {
		t.Fatalf("init-kbagent imagePullPolicy = %q, want %q", initAgent.ImagePullPolicy, corev1.PullNever)
	}
}
