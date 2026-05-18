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

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

const (
	testImageDigestA = "sha256:abc1234567890abcdef1234567890abcdef1234567890abcdef1234567890abc"
	testImageDigestB = "sha256:def0000000000000000000000000000000000000000000000000000000000000"
)

func TestIsImageMatchedDigestPinnedFallsBackToImageID(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "main",
				Image: "apecloud/kubeblocks@" + testImageDigestA,
			}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "main",
				Image:   "apecloud/kubeblocks:1.0.3-beta.7",
				ImageID: "docker.io/apecloud/kubeblocks@" + testImageDigestA,
			}},
		},
	}
	if !isImageMatched(pod) {
		t.Fatalf("expected digest-pinned spec to match via status.ImageID when status.Image lacks digest")
	}
}

func TestIsImageMatchedDigestPinnedRejectsOnImageIDMismatch(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "main",
				Image: "apecloud/kubeblocks@" + testImageDigestA,
			}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "main",
				Image:   "apecloud/kubeblocks:1.0.3-beta.7",
				ImageID: "docker.io/apecloud/kubeblocks@" + testImageDigestB,
			}},
		},
	}
	if isImageMatched(pod) {
		t.Fatalf("expected digest-pinned spec to reject when status.ImageID digest does not match")
	}
}

func TestIsImageMatchedTagOnlySpecKeepsStrictTagComparison(t *testing.T) {
	// Tag-only spec must continue to enforce strict status.Image tag match
	// even when status.ImageID carries a non-empty digest. The ImageID
	// fallback is intentionally limited to digest-pinned specs; tag-only
	// specs cannot be resolved to a digest at the controller level, so a
	// status tag drift must still surface as a non-match (otherwise a
	// pending image upgrade would be misreported as already realized).
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "main",
				Image: "apecloud/kubeblocks:1.0.3-beta.7",
			}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:    "main",
				Image:   "apecloud/kubeblocks:1.0.3-beta.6",
				ImageID: "docker.io/apecloud/kubeblocks@" + testImageDigestA,
			}},
		},
	}
	if isImageMatched(pod) {
		t.Fatalf("expected tag-only spec to reject when status.Image tag differs, ignoring status.ImageID")
	}
}

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
