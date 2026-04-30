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

package instanceset2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestParseParentNameAndOrdinal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantParent  string
		wantOrdinal int
	}{
		{
			name:        "standard pod name",
			input:       "test-its-0",
			wantParent:  "test-its",
			wantOrdinal: 0,
		},
		{
			name:        "higher ordinal",
			input:       "my-instance-42",
			wantParent:  "my-instance",
			wantOrdinal: 42,
		},
		{
			name:        "no numeric suffix",
			input:       "my-instance-abc",
			wantParent:  "my-instance-abc",
			wantOrdinal: -1,
		},
		{
			name:        "no dash",
			input:       "nodash",
			wantParent:  "nodash",
			wantOrdinal: -1,
		},
		{
			name:        "empty string",
			input:       "",
			wantParent:  "",
			wantOrdinal: -1,
		},
		{
			name:        "multiple dashes with ordinal",
			input:       "a-b-c-3",
			wantParent:  "a-b-c",
			wantOrdinal: 3,
		},
		{
			name:        "trailing dash only",
			input:       "test-",
			wantParent:  "test-",
			wantOrdinal: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, ordinal := parseParentNameAndOrdinal(tt.input)
			assert.Equal(t, tt.wantParent, parent)
			assert.Equal(t, tt.wantOrdinal, ordinal)
		})
	}
}

func TestSortObjects(t *testing.T) {
	rolePriorityMap := map[string]int{
		"":         0,
		"learner":  1,
		"follower": 2,
		"leader":   3,
	}

	makePod := func(name, role string) *corev1.Pod {
		labels := map[string]string{}
		if role != "" {
			labels[constant.RoleLabelKey] = role
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
		}
	}

	t.Run("sort by role priority ascending", func(t *testing.T) {
		pods := []client.Object{
			makePod("pod-leader-0", "leader"),
			makePod("pod-follower-1", "follower"),
			makePod("pod-learner-0", "learner"),
			makePod("pod-unknown-0", ""),
		}

		sortObjects(pods, rolePriorityMap, false)

		assert.Equal(t, "pod-unknown-0", pods[0].GetName())
		assert.Equal(t, "pod-learner-0", pods[1].GetName())
		assert.Equal(t, "pod-follower-1", pods[2].GetName())
		assert.Equal(t, "pod-leader-0", pods[3].GetName())
	})

	t.Run("sort reversed", func(t *testing.T) {
		pods := []client.Object{
			makePod("pod-unknown-0", ""),
			makePod("pod-leader-0", "leader"),
			makePod("pod-follower-0", "follower"),
		}

		sortObjects(pods, rolePriorityMap, true)

		assert.Equal(t, "pod-leader-0", pods[0].GetName())
		assert.Equal(t, "pod-follower-0", pods[1].GetName())
		assert.Equal(t, "pod-unknown-0", pods[2].GetName())
	})

	t.Run("same role sorted by name descending then ordinal descending", func(t *testing.T) {
		pods := []client.Object{
			makePod("foo-0", "follower"),
			makePod("foo-2", "follower"),
			makePod("foo-1", "follower"),
		}

		sortObjects(pods, rolePriorityMap, false)

		// same parent "foo", sorted by ordinal descending: 2, 1, 0
		assert.Equal(t, "foo-2", pods[0].GetName())
		assert.Equal(t, "foo-1", pods[1].GetName())
		assert.Equal(t, "foo-0", pods[2].GetName())
	})

	t.Run("different parent names, same role", func(t *testing.T) {
		pods := []client.Object{
			makePod("aaa-0", "follower"),
			makePod("zzz-0", "follower"),
			makePod("mmm-0", "follower"),
		}

		sortObjects(pods, rolePriorityMap, false)

		// same role, sorted by parent name descending: zzz, mmm, aaa
		assert.Equal(t, "zzz-0", pods[0].GetName())
		assert.Equal(t, "mmm-0", pods[1].GetName())
		assert.Equal(t, "aaa-0", pods[2].GetName())
	})

	t.Run("empty list", func(t *testing.T) {
		var pods []client.Object
		sortObjects(pods, rolePriorityMap, false)
		assert.Empty(t, pods)
	})
}

func TestBaseSort_NilRolePriorityFunc(t *testing.T) {
	items := []int{3, 1, 2}
	getNameNOrdinal := func(i int) (string, int) {
		return "x", items[i]
	}
	// nil role priority func should not panic
	baseSort(items, getNameNOrdinal, nil, false)
	// all have same role priority (0), sorted by ordinal desc
	assert.Equal(t, []int{3, 2, 1}, items)
}

func TestCopyAndMerge(t *testing.T) {
	t.Run("service merge", func(t *testing.T) {
		oldSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "my-svc",
				Namespace:   "default",
				Labels:      map[string]string{"old": "label"},
				Annotations: map[string]string{"old": "ann"},
				Finalizers:  []string{"keep-me"},
				OwnerReferences: []metav1.OwnerReference{
					{Name: "owner1", UID: "uid1"},
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{Name: "old-port", Port: 80},
				},
				Selector: map[string]string{"old": "selector"},
			},
		}
		newSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "my-svc",
				Namespace:   "default",
				Labels:      map[string]string{"new": "label"},
				Annotations: map[string]string{"new": "ann"},
				Finalizers:  []string{"keep-me", "new-finalizer"},
				OwnerReferences: []metav1.OwnerReference{
					{Name: "owner1", UID: "uid1"},
					{Name: "owner2", UID: "uid2"},
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{Name: "new-port", Port: 8080},
				},
				Selector:                 map[string]string{"new": "selector"},
				PublishNotReadyAddresses: true,
			},
		}

		result := copyAndMerge(oldSvc, newSvc)
		require.NotNil(t, result)
		svc := result.(*corev1.Service)

		// Spec fields come from new
		assert.Equal(t, map[string]string{"new": "selector"}, svc.Spec.Selector)
		assert.Equal(t, corev1.ServiceTypeNodePort, svc.Spec.Type)
		assert.True(t, svc.Spec.PublishNotReadyAddresses)
		assert.Equal(t, []corev1.ServicePort{{Name: "new-port", Port: 8080}}, svc.Spec.Ports)

		// Labels/annotations merged (new overwrites old)
		assert.Equal(t, "label", svc.Labels["new"])
		assert.Equal(t, "ann", svc.Annotations["new"])

		// Finalizers merged
		assert.Contains(t, svc.Finalizers, "keep-me")
		assert.Contains(t, svc.Finalizers, "new-finalizer")

		// OwnerReferences merged
		assert.Len(t, svc.OwnerReferences, 2)
	})

	t.Run("type mismatch returns nil", func(t *testing.T) {
		oldSvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc"}}
		newPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod"}}
		result := copyAndMerge(oldSvc, newPod)
		assert.Nil(t, result)
	})

	t.Run("non-service type returns newObj", func(t *testing.T) {
		oldPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "old"}}
		newPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "new"}}
		result := copyAndMerge(oldPod, newPod)
		assert.Equal(t, "new", result.GetName())
	})
}

func TestCopyAndMergeInstance(t *testing.T) {
	t.Run("changed spec returns updated instance", func(t *testing.T) {
		oldInst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "inst-0",
				Labels:      map[string]string{"old": "l"},
				Annotations: map[string]string{"old": "a"},
			},
			Spec: workloads.InstanceSpec{
				InstanceSetName:      "old-its",
				InstanceTemplateName: "old-tpl",
				MinReadySeconds:      10,
			},
		}
		newInst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "inst-0",
				Labels:      map[string]string{"new": "l"},
				Annotations: map[string]string{"new": "a"},
			},
			Spec: workloads.InstanceSpec{
				InstanceSetName:      "new-its",
				InstanceTemplateName: "new-tpl",
				MinReadySeconds:      30,
			},
		}

		result := copyAndMergeInstance(oldInst, newInst)
		require.NotNil(t, result)
		assert.Equal(t, "new-its", result.Spec.InstanceSetName)
		assert.Equal(t, "new-tpl", result.Spec.InstanceTemplateName)
		assert.Equal(t, int32(30), result.Spec.MinReadySeconds)
		// Labels/annotations merged
		assert.Equal(t, "l", result.Labels["new"])
		assert.Equal(t, "a", result.Annotations["new"])
	})

	t.Run("identical specs return nil", func(t *testing.T) {
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "inst-0",
				Labels:      map[string]string{"k": "v"},
				Annotations: map[string]string{"k": "v"},
			},
			Spec: workloads.InstanceSpec{
				InstanceSetName: "its",
			},
		}
		result := copyAndMergeInstance(inst, inst.DeepCopy())
		assert.Nil(t, result)
	})
}

func TestGetInstanceTemplateMap(t *testing.T) {
	t.Run("nil annotations", func(t *testing.T) {
		m, err := getInstanceTemplateMap(nil)
		require.NoError(t, err)
		assert.Nil(t, m)
	})

	t.Run("no template-ref annotation", func(t *testing.T) {
		m, err := getInstanceTemplateMap(map[string]string{"other": "value"})
		require.NoError(t, err)
		assert.Nil(t, m)
	})

	t.Run("valid template-ref", func(t *testing.T) {
		ann := map[string]string{
			templateRefAnnotationKey: `{"inst-0":"tpl-a","inst-1":"tpl-b"}`,
		}
		m, err := getInstanceTemplateMap(ann)
		require.NoError(t, err)
		assert.Equal(t, "tpl-a", m["inst-0"])
		assert.Equal(t, "tpl-b", m["inst-1"])
	})

	t.Run("invalid JSON", func(t *testing.T) {
		ann := map[string]string{
			templateRefAnnotationKey: `{invalid`,
		}
		_, err := getInstanceTemplateMap(ann)
		assert.Error(t, err)
	})
}

func TestGetHeadlessSvcName(t *testing.T) {
	assert.Equal(t, "my-its-headless", getHeadlessSvcName("my-its"))
	assert.Equal(t, "-headless", getHeadlessSvcName(""))
}
