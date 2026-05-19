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

package instance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

func TestFilterInPlaceFields(t *testing.T) {
	src := &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"app": "test"},
			Annotations: map[string]string{"other": "value", constant.RestartAnnotationKey: "true"},
		},
		Spec: corev1.PodSpec{
			ActiveDeadlineSeconds: func() *int64 { v := int64(30); return &v }(),
			Tolerations: []corev1.Toleration{
				{Key: "key1", Effect: corev1.TaintEffectNoSchedule},
			},
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "nginx:1.19",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
			InitContainers: []corev1.Container{
				{Name: "init", Image: "busybox:latest"},
			},
		},
	}

	result := filterInPlaceFields(src)

	// labels should be nil
	assert.Nil(t, result.Labels)
	// only restart annotation should remain
	assert.Equal(t, "true", result.Annotations[constant.RestartAnnotationKey])
	assert.NotContains(t, result.Annotations, "other")
	// images should be empty
	assert.Equal(t, "", result.Spec.Containers[0].Image)
	assert.Equal(t, "", result.Spec.InitContainers[0].Image)
	// active deadline seconds should be nil
	assert.Nil(t, result.Spec.ActiveDeadlineSeconds)
	// tolerations should be nil
	assert.Nil(t, result.Spec.Tolerations)
	// cpu and memory resources should be removed
	_, hasCPU := result.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
	assert.False(t, hasCPU)
	_, hasMem := result.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory]
	assert.False(t, hasMem)

	// original should not be mutated
	assert.Equal(t, "nginx:1.19", src.Spec.Containers[0].Image)
}

func TestFilterInPlaceFields_NoAnnotations(t *testing.T) {
	src := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
		},
	}
	result := filterInPlaceFields(src)
	assert.Nil(t, result.Annotations)
	assert.Equal(t, "", result.Spec.Containers[0].Image)
}

func TestCopyRequestsNLimitsFields(t *testing.T) {
	t.Run("with resources", func(t *testing.T) {
		container := &corev1.Container{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		}
		requests, limits := copyRequestsNLimitsFields(container)
		assert.True(t, requests.Cpu().Equal(resource.MustParse("100m")))
		assert.True(t, requests.Memory().Equal(resource.MustParse("128Mi")))
		assert.True(t, limits.Cpu().Equal(resource.MustParse("200m")))
		assert.True(t, limits.Memory().Equal(resource.MustParse("256Mi")))
	})

	t.Run("empty resources", func(t *testing.T) {
		container := &corev1.Container{}
		requests, limits := copyRequestsNLimitsFields(container)
		assert.Empty(t, requests)
		assert.Empty(t, limits)
	})
}

func TestEqualField(t *testing.T) {
	t.Run("map subset match", func(t *testing.T) {
		old := map[string]string{"a": "1", "b": "2", "c": "3"}
		new := map[string]string{"a": "1", "b": "2"}
		assert.True(t, equalField(old, new))
	})

	t.Run("map value mismatch", func(t *testing.T) {
		old := map[string]string{"a": "1"}
		new := map[string]string{"a": "2"}
		assert.False(t, equalField(old, new))
	})

	t.Run("map missing key", func(t *testing.T) {
		old := map[string]string{"a": "1"}
		new := map[string]string{"b": "1"}
		assert.False(t, equalField(old, new))
	})

	t.Run("new map larger than old", func(t *testing.T) {
		old := map[string]string{"a": "1"}
		new := map[string]string{"a": "1", "b": "2"}
		assert.False(t, equalField(old, new))
	})

	t.Run("string equal", func(t *testing.T) {
		assert.True(t, equalField("hello", "hello"))
	})

	t.Run("string not equal", func(t *testing.T) {
		assert.False(t, equalField("hello", "world"))
	})

	t.Run("container images match", func(t *testing.T) {
		old := []corev1.Container{{Name: "c1", Image: "nginx:1.19"}}
		new := []corev1.Container{{Name: "c1", Image: "nginx:1.19"}}
		assert.True(t, equalField(old, new))
	})

	t.Run("container images mismatch", func(t *testing.T) {
		old := []corev1.Container{{Name: "c1", Image: "nginx:1.18"}}
		new := []corev1.Container{{Name: "c1", Image: "nginx:1.19"}}
		assert.False(t, equalField(old, new))
	})

	t.Run("container count mismatch", func(t *testing.T) {
		old := []corev1.Container{{Name: "c1"}}
		new := []corev1.Container{{Name: "c1"}, {Name: "c2"}}
		assert.False(t, equalField(old, new))
	})

	t.Run("tolerations always match", func(t *testing.T) {
		old := []corev1.Toleration{{Key: "k1"}}
		new := []corev1.Toleration{{Key: "k2"}}
		assert.True(t, equalField(old, new))
	})

	t.Run("resource list equal", func(t *testing.T) {
		old := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		}
		new := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		}
		assert.True(t, equalField(old, new))
	})

	t.Run("resource list cpu mismatch", func(t *testing.T) {
		old := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}
		new := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}
		assert.False(t, equalField(old, new))
	})
}

func TestEqualBasicInPlaceFields(t *testing.T) {
	t.Run("equal pods", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"a": "1"},
				Labels:      map[string]string{"l": "1"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "c1", Image: "img:1"}},
			},
		}
		assert.True(t, equalBasicInPlaceFields(pod, pod.DeepCopy()))
	})

	t.Run("different image", func(t *testing.T) {
		old := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "c1", Image: "img:1"}},
			},
		}
		new := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "c1", Image: "img:2"}},
			},
		}
		assert.False(t, equalBasicInPlaceFields(old, new))
	})
}

func TestEqualResourcesInPlaceFields(t *testing.T) {
	t.Run("equal resources", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "c1",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
							Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
						},
					},
				},
			},
		}
		assert.True(t, equalResourcesInPlaceFields(pod, pod.DeepCopy()))
	})

	t.Run("different requests", func(t *testing.T) {
		old := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "c1",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
						},
					},
				},
			},
		}
		new := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "c1",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
						},
					},
				},
			},
		}
		assert.False(t, equalResourcesInPlaceFields(old, new))
	})

	t.Run("requests defaults to limits", func(t *testing.T) {
		old := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "c1",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
							Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
						},
					},
				},
			},
		}
		// new has nil requests, defaults to limits
		new := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "c1",
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
						},
					},
				},
			},
		}
		assert.True(t, equalResourcesInPlaceFields(old, new))
	})
}

func TestEqualBasicInPlaceFields_AnnotationsDiffer(t *testing.T) {
	old := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"a": "1"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1", Image: "img:1"}},
		},
	}
	new := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"a": "2"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1", Image: "img:1"}},
		},
	}
	assert.False(t, equalBasicInPlaceFields(old, new))
}

func TestEqualBasicInPlaceFields_LabelsDiffer(t *testing.T) {
	old := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"l": "1"}},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1", Image: "img:1"}},
		},
	}
	new := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"l": "2"}},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1", Image: "img:1"}},
		},
	}
	assert.False(t, equalBasicInPlaceFields(old, new))
}

func TestEqualBasicInPlaceFields_ActiveDeadlineDiffers(t *testing.T) {
	old := &corev1.Pod{
		Spec: corev1.PodSpec{
			ActiveDeadlineSeconds: ptr.To(int64(30)),
			Containers:           []corev1.Container{{Name: "c1", Image: "img:1"}},
		},
	}
	new := &corev1.Pod{
		Spec: corev1.PodSpec{
			ActiveDeadlineSeconds: ptr.To(int64(60)),
			Containers:           []corev1.Container{{Name: "c1", Image: "img:1"}},
		},
	}
	assert.False(t, equalBasicInPlaceFields(old, new))
}

func TestEqualBasicInPlaceFields_InitContainersDiffer(t *testing.T) {
	old := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init", Image: "busybox:1.0"}},
			Containers:     []corev1.Container{{Name: "c1", Image: "img:1"}},
		},
	}
	new := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init", Image: "busybox:2.0"}},
			Containers:     []corev1.Container{{Name: "c1", Image: "img:1"}},
		},
	}
	assert.False(t, equalBasicInPlaceFields(old, new))
}

func TestEqualResourcesInPlaceFields_ContainerNotFound(t *testing.T) {
	old := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1"}},
		},
	}
	new := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c2"}},
		},
	}
	assert.False(t, equalResourcesInPlaceFields(old, new))
}

func TestEqualResourcesInPlaceFields_LimitsDiffer(t *testing.T) {
	old := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "c1",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
					},
				},
			},
		},
	}
	new := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "c1",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
					},
				},
			},
		},
	}
	assert.False(t, equalResourcesInPlaceFields(old, new))
}

func TestGetPodUpdatePolicy_ConfigRestart(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID: types.UID("uid1"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
			Configs: []workloads.ConfigTemplate{{
				Name:       "cfg1",
				ConfigHash: ptr.To("new-hash"),
				Restart:    ptr.To(true),
			}},
		},
	}

	pod := builder.NewPodBuilder("default", "inst-0").GetObject()
	require.NoError(t, configsToPod([]workloads.ConfigTemplate{{
		Name:       "cfg1",
		ConfigHash: ptr.To("old-hash"),
	}}, pod))

	policy, _, err := getPodUpdatePolicy(inst, pod)
	require.NoError(t, err)
	assert.Equal(t, recreatePolicy, policy)
}

func TestIsPodUpdated_UpToDate(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID: types.UID("uid1"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
		},
	}

	// Build the actual pod from the instance to ensure same revision
	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	pod.Status.Phase = corev1.PodRunning

	// Set UpdateRevision to match
	inst.Status.UpdateRevision = getPodRevision(pod)

	updated, err := isPodUpdated(inst, pod)
	require.NoError(t, err)
	assert.True(t, updated)
}

func TestMergeInPlaceFields(t *testing.T) {
	src := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"new": "label"},
			Annotations: map[string]string{"new": "ann"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "c1", Image: "nginx:new"},
			},
			InitContainers: []corev1.Container{
				{Name: "init", Image: "busybox:new"},
			},
		},
	}
	dst := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"old": "label"},
			Annotations: map[string]string{"old": "ann"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "c1", Image: "nginx:old"},
			},
			InitContainers: []corev1.Container{
				{Name: "init", Image: "busybox:old"},
			},
		},
	}

	mergeInPlaceFields(src, dst)
	assert.Equal(t, "label", dst.Labels["new"])
	assert.Equal(t, "ann", dst.Annotations["new"])
	assert.Equal(t, "nginx:new", dst.Spec.Containers[0].Image)
	assert.Equal(t, "busybox:new", dst.Spec.InitContainers[0].Image)
}
