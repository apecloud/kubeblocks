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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

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

func TestConfigHashOnlyInPlaceUpdate(t *testing.T) {
	oldPod := builder.NewPodBuilder("default", "valkey-0").
		AddAnnotations("kept", "value").
		SetContainers([]corev1.Container{{Name: "valkey", Image: "valkey:9"}}).
		GetObject()
	if err := configsToPod([]workloads.ConfigTemplate{{
		Name:       "valkey-replication-config",
		ConfigHash: ptr.To("old-hash"),
	}}, oldPod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}

	newPod := oldPod.DeepCopy()
	if err := configsToPod([]workloads.ConfigTemplate{{
		Name:       "valkey-replication-config",
		ConfigHash: ptr.To("new-hash"),
	}}, newPod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}
	if !configHashOnlyInPlaceUpdate(oldPod, newPod) {
		t.Fatalf("expected config hash only update")
	}

	newConfigHashPod := func(t *testing.T) *corev1.Pod {
		t.Helper()
		pod := oldPod.DeepCopy()
		if err := configsToPod([]workloads.ConfigTemplate{{
			Name:       "valkey-replication-config",
			ConfigHash: ptr.To("new-hash"),
		}}, pod); err != nil {
			t.Fatalf("configsToPod() error = %v", err)
		}
		return pod
	}

	cases := []struct {
		name   string
		mutate func(*corev1.Pod)
	}{{
		name: "label",
		mutate: func(pod *corev1.Pod) {
			pod.Labels = map[string]string{"extra": "label"}
		},
	}, {
		name: "non config-hash annotation",
		mutate: func(pod *corev1.Pod) {
			pod.Annotations["other"] = "changed"
		},
	}, {
		name: "container image",
		mutate: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Image = "valkey:10"
		},
	}, {
		name: "container resources",
		mutate: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			}
		},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := newConfigHashPod(t)
			tc.mutate(pod)
			if configHashOnlyInPlaceUpdate(oldPod, pod) {
				t.Fatalf("expected %s update not to be config hash only", tc.name)
			}
		})
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
