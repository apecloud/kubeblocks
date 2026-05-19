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
	"github.com/apecloud/kubeblocks/pkg/constant"
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

func TestSafeMetadataOnlyInPlaceUpdate(t *testing.T) {
	basePod := builder.NewPodBuilder("default", "valkey-0").
		AddAnnotations("kept", "value").
		AddLabels("app", "valkey").
		SetContainers([]corev1.Container{{Name: "valkey", Image: "valkey:9"}}).
		GetObject()
	if err := configsToPod([]workloads.ConfigTemplate{{
		Name:       "valkey-replication-config",
		ConfigHash: ptr.To("old-hash"),
	}}, basePod); err != nil {
		t.Fatalf("configsToPod() error = %v", err)
	}

	positiveCases := []struct {
		name   string
		mutate func(*corev1.Pod)
	}{{
		name: "config-hash annotation patch",
		mutate: func(pod *corev1.Pod) {
			if err := configsToPod([]workloads.ConfigTemplate{{
				Name:       "valkey-replication-config",
				ConfigHash: ptr.To("new-hash"),
			}}, pod); err != nil {
				t.Fatalf("configsToPod() error = %v", err)
			}
		},
	}, {
		name: "non-restart annotation added",
		mutate: func(pod *corev1.Pod) {
			pod.Annotations["custom"] = "value"
		},
	}, {
		name: "non-restart annotation value changed",
		mutate: func(pod *corev1.Pod) {
			pod.Annotations["kept"] = "changed"
		},
	}, {
		name: "label added",
		mutate: func(pod *corev1.Pod) {
			pod.Labels["extra"] = "value"
		},
	}, {
		name: "label value changed",
		mutate: func(pod *corev1.Pod) {
			pod.Labels["app"] = "valkey-renamed"
		},
	}, {
		name: "role label state synchronization",
		mutate: func(pod *corev1.Pod) {
			pod.Labels[constant.RoleLabelKey] = "primary"
		},
	}}
	for _, tc := range positiveCases {
		t.Run("skip switchover when "+tc.name, func(t *testing.T) {
			newPod := basePod.DeepCopy()
			tc.mutate(newPod)
			if !safeMetadataOnlyInPlaceUpdate(basePod, newPod) {
				t.Fatalf("expected %s to be a safe metadata-only update", tc.name)
			}
		})
	}

	negativeCases := []struct {
		name   string
		mutate func(*corev1.Pod)
	}{{
		name:   "no diff",
		mutate: func(pod *corev1.Pod) {},
	}, {
		name: "restart annotation added",
		mutate: func(pod *corev1.Pod) {
			pod.Annotations[constant.RestartAnnotationKey] = "2026-05-19T14:00:00Z"
		},
	}, {
		name: "restart annotation value changed",
		mutate: func(pod *corev1.Pod) {
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[constant.RestartAnnotationKey] = "next"
		},
	}, {
		name: "container image changed",
		mutate: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Image = "valkey:10"
		},
	}, {
		name: "container resources changed",
		mutate: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			}
		},
	}, {
		name: "container env added",
		mutate: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{Name: "EXTRA", Value: "v"})
		},
	}}
	for _, tc := range negativeCases {
		t.Run("invoke switchover when "+tc.name, func(t *testing.T) {
			newPod := basePod.DeepCopy()
			tc.mutate(newPod)
			if safeMetadataOnlyInPlaceUpdate(basePod, newPod) {
				t.Fatalf("expected %s not to be a safe metadata-only update", tc.name)
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
