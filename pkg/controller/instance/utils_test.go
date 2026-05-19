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

package instance

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

func TestConfigsToUpdateTreatsNilAndEmptyConfigHashAsEqual(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "valkey-0").
		SetConfigs([]workloads.ConfigTemplate{{
			Name: "valkey-replication-config",
		}}).
		GetObject()
	pod := builder.NewPodBuilder("default", "valkey-0").GetObject()
	if err := configsToPod([]workloads.ConfigTemplate{{
		Name:       "valkey-replication-config",
		ConfigHash: ptr.To(""),
	}}, pod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}

	toUpdate, err := configsToUpdate(inst, pod)
	if err != nil {
		t.Fatalf("configsToUpdate() error = %v", err)
	}
	if len(toUpdate) != 0 {
		t.Fatalf("expected no config drift, got %#v", toUpdate)
	}
}

func TestConfigsToUpdateStillReportsRealConfigHashMismatch(t *testing.T) {
	inst := builder.NewInstanceBuilder("default", "valkey-0").
		SetConfigs([]workloads.ConfigTemplate{{
			Name:       "valkey-replication-config",
			ConfigHash: ptr.To("desired-hash"),
		}}).
		GetObject()
	pod := builder.NewPodBuilder("default", "valkey-0").GetObject()
	if err := configsToPod([]workloads.ConfigTemplate{{
		Name:       "valkey-replication-config",
		ConfigHash: ptr.To(""),
	}}, pod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}

	toUpdate, err := configsToUpdate(inst, pod)
	if err != nil {
		t.Fatalf("configsToUpdate() error = %v", err)
	}
	if len(toUpdate) != 1 {
		t.Fatalf("expected one config drift item, got %#v", toUpdate)
	}
	if toUpdate[0].Name != "valkey-replication-config" {
		t.Fatalf("unexpected config name %q", toUpdate[0].Name)
	}
}

func TestIsImageMatched(t *testing.T) {
	pod := builder.NewPodBuilder("default", "valkey-0").GetObject()

	tests := []struct {
		name        string
		specImage   string
		statusImage string
		statusID    string
		want        bool
	}{
		{
			name:        "tagless spec accepts status tag and registry",
			specImage:   "nginx",
			statusImage: "docker.io/nginx:latest@sha256:0f37a86c04f8",
			want:        true,
		},
		{
			name:        "digest spec matches imageID digest",
			specImage:   "docker.io/nginx@sha256:0f37a86c04f8",
			statusImage: "docker.io/nginx:latest",
			statusID:    "docker.io/nginx@sha256:0f37a86c04f8",
			want:        true,
		},
		{
			name:        "digest spec ignores local status image when imageID matches",
			specImage:   "docker.io/nginx@sha256:0f37a86c04f8",
			statusImage: "sha256:runtime-local-image-id",
			statusID:    "docker.io/nginx@sha256:0f37a86c04f8",
			want:        true,
		},
		{
			name:        "digest spec rejects different imageID digest",
			specImage:   "docker.io/nginx@sha256:0f37a86c04f8",
			statusImage: "docker.io/nginx@sha256:0f37a86c04f8",
			statusID:    "docker.io/nginx@sha256:different",
			want:        false,
		},
		{
			name:        "tag spec rejects different status image tag even with imageID",
			specImage:   "docker.io/nginx:1.0.0",
			statusImage: "docker.io/nginx:latest",
			statusID:    "docker.io/nginx@sha256:0f37a86c04f8",
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod.Spec.Containers = []corev1.Container{{
				Name:  "container",
				Image: tt.specImage,
			}}
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
				Name:    "container",
				Image:   tt.statusImage,
				ImageID: tt.statusID,
			}}
			if got := isImageMatched(pod); got != tt.want {
				t.Fatalf("isImageMatched() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainerImageComparisonIgnoresRegistryRewrites(t *testing.T) {
	oldPod := builder.NewPodBuilder("default", "valkey-0").
		AddInitContainer(corev1.Container{Name: "init", Image: "172.31.255.3:5000/apecloud/kbagent:1.0.3-beta.5"}).
		AddContainer(corev1.Container{Name: "redis", Image: "172.31.255.3:5000/apecloud/redis:8.4.0"}).
		GetObject()
	newPod := builder.NewPodBuilder("default", "valkey-0").
		AddInitContainer(corev1.Container{Name: "init", Image: "192.168.173.140:6451/apecloud/kbagent:1.0.3-beta.5"}).
		AddContainer(corev1.Container{Name: "redis", Image: "192.168.173.140:6451/apecloud/redis:8.4.0"}).
		GetObject()
	inst := builder.NewInstanceBuilder("default", "valkey-0").
		SetPodUpdatePolicy(kbappsv1.ReCreatePodUpdatePolicyType).
		SetPodUpgradePolicy(kbappsv1.PreferInPlacePodUpdatePolicyType).
		GetObject()

	if !equalBasicInPlaceFields(oldPod, newPod) {
		t.Fatal("expected registry-only image rewrite to match basic in-place fields")
	}
	if got := getPodUpdatePolicyInSpec(inst, oldPod, newPod); got != kbappsv1.ReCreatePodUpdatePolicyType {
		t.Fatalf("getPodUpdatePolicyInSpec() = %q, want %q", got, kbappsv1.ReCreatePodUpdatePolicyType)
	}

	newPod.Spec.Containers[0].Image = "192.168.173.140:6451/apecloud/redis-stack:8.4.0"
	if equalBasicInPlaceFields(oldPod, newPod) {
		t.Fatal("expected different image basename to be detected")
	}
	if got := getPodUpdatePolicyInSpec(inst, oldPod, newPod); got != kbappsv1.PreferInPlacePodUpdatePolicyType {
		t.Fatalf("getPodUpdatePolicyInSpec() = %q, want %q", got, kbappsv1.PreferInPlacePodUpdatePolicyType)
	}
}
