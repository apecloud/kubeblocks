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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestParseReplicasNMaxUnavailable(t *testing.T) {
	tests := []struct {
		name           string
		strategy       *workloads.InstanceUpdateStrategy
		totalReplicas  int
		wantReplicas   int
		wantMaxUnavail int
		wantErr        bool
	}{
		{
			name:           "nil strategy",
			strategy:       nil,
			totalReplicas:  5,
			wantReplicas:   5,
			wantMaxUnavail: 1,
		},
		{
			name:           "nil rolling update",
			strategy:       &workloads.InstanceUpdateStrategy{},
			totalReplicas:  5,
			wantReplicas:   5,
			wantMaxUnavail: 1,
		},
		{
			name: "explicit replicas",
			strategy: &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas: ptr.To(intstr.FromInt32(3)),
				},
			},
			totalReplicas:  10,
			wantReplicas:   3,
			wantMaxUnavail: 1,
		},
		{
			name: "explicit maxUnavailable",
			strategy: &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					MaxUnavailable: ptr.To(intstr.FromInt32(2)),
				},
			},
			totalReplicas:  5,
			wantReplicas:   5,
			wantMaxUnavail: 2,
		},
		{
			name: "percentage replicas",
			strategy: &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas: ptr.To(intstr.FromString("50%")),
				},
			},
			totalReplicas:  10,
			wantReplicas:   5,
			wantMaxUnavail: 1,
		},
		{
			name: "maxUnavailable 0% rounds up to 1",
			strategy: &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					MaxUnavailable: ptr.To(intstr.FromString("1%")),
				},
			},
			totalReplicas:  3,
			wantReplicas:   3,
			wantMaxUnavail: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicas, maxUnavail, err := parseReplicasNMaxUnavailable(tt.strategy, tt.totalReplicas)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantReplicas, replicas)
			assert.Equal(t, tt.wantMaxUnavail, maxUnavail)
		})
	}
}

func TestUpdateReconciler_PreCondition(t *testing.T) {
	r := NewUpdateReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				DeletionTimestamp: &now,
				Finalizers:        []string{"test"},
			},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("paused root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec:       workloads.InstanceSetSpec{Paused: true},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("normal root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestDeletionReconciler_PreCondition(t *testing.T) {
	r := NewDeletionReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("non-deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				DeletionTimestamp: &now,
				Finalizers:        []string{"test"},
			},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})

	t.Run("paused deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				DeletionTimestamp: &now,
				Finalizers:        []string{"test"},
			},
			Spec: workloads.InstanceSetSpec{Paused: true},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})
}

func TestDeletionReconciler_ReconcileWithNoSecondary(t *testing.T) {
	r := NewDeletionReconciler()
	tree := kubebuilderx.NewObjectTree()
	now := metav1.Now()
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			DeletionTimestamp: &now,
			Finalizers:        []string{"test"},
		},
	}
	tree.SetRoot(its)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
	assert.Nil(t, tree.GetRoot())
}

func TestDeletionReconciler_ReconcileWithSecondary(t *testing.T) {
	r := NewDeletionReconciler()
	tree := kubebuilderx.NewObjectTree()
	now := metav1.Now()
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			DeletionTimestamp: &now,
			Finalizers:        []string{"test"},
		},
	}
	tree.SetRoot(its)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "svc-1"},
	}
	require.NoError(t, tree.Add(svc))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
	assert.NotNil(t, tree.GetRoot())
	assert.Empty(t, tree.GetSecondaryObjects())
}

func TestRevisionUpdateReconciler_PreCondition(t *testing.T) {
	r := NewRevisionUpdateReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("status up to date unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Status:     workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("updating root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 2},
			Status:     workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)
		assert.True(t, model.IsObjectUpdating(its))
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestRevisionUpdateReconciler_Reconcile(t *testing.T) {
	r := NewRevisionUpdateReconciler()
	tree := kubebuilderx.NewObjectTree()
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 3},
		Status:     workloads.InstanceSetStatus{ObservedGeneration: 1},
	}
	tree.SetRoot(its)

	inst0 := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Generation: 1,
			Annotations: map[string]string{constant.KubeBlocksGenerationKey: "3"},
		},
		Status: workloads.InstanceStatus2{ObservedGeneration: 1, UpToDate: true},
	}
	inst1 := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-1", Generation: 1,
			Annotations: map[string]string{constant.KubeBlocksGenerationKey: "2"},
		},
		Status: workloads.InstanceStatus2{ObservedGeneration: 1, UpToDate: true},
	}
	require.NoError(t, tree.Add(inst0, inst1))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	updatedIts := tree.GetRoot().(*workloads.InstanceSet)
	assert.Equal(t, int32(1), updatedIts.Status.UpdatedReplicas)
	assert.Equal(t, int64(3), updatedIts.Status.ObservedGeneration)
}

func TestFixMetaReconciler_PreCondition(t *testing.T) {
	r := NewFixMetaReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("already has finalizer unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Finalizers: []string{finalizer}},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("no finalizer satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestFixMetaReconciler_Reconcile(t *testing.T) {
	r := NewFixMetaReconciler()
	tree := kubebuilderx.NewObjectTree()
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	tree.SetRoot(its)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Commit, result)
	assert.Contains(t, tree.GetRoot().GetFinalizers(), finalizer)
}

func TestAPIVersionReconciler_PreCondition(t *testing.T) {
	r := NewAPIVersionReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("non-nil root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestAPIVersionReconciler_Reconcile(t *testing.T) {
	r := NewAPIVersionReconciler()

	t.Run("unsupported api version returns commit", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		// Without CRDAPIVersion annotation, ObjectAPIVersionSupported returns false
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(its)

		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Commit, result)
	})
}

func TestAlignmentReconciler_PreCondition(t *testing.T) {
	r := NewAlignmentReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("paused root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec:       workloads.InstanceSetSpec{Paused: true},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("normal root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestStatusReconciler_PreCondition(t *testing.T) {
	r := NewStatusReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("updating root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 2},
			Status:     workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("status-updating root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Status:     workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestAssistantObjectReconciler_PreCondition(t *testing.T) {
	r := NewAssistantObjectReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("normal root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestValidationReconciler_PreCondition(t *testing.T) {
	r := NewValidationReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("non-nil root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(its)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestValidateUnsupportedFeatures(t *testing.T) {
	t.Run("nil its", func(t *testing.T) {
		assert.NoError(t, validateUnsupportedFeatures(nil))
	})

	t.Run("nil annotations", func(t *testing.T) {
		assert.NoError(t, validateUnsupportedFeatures(&workloads.InstanceSet{}))
	})

	t.Run("no unsupported annotations", func(t *testing.T) {
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"other": "value"}},
		}
		assert.NoError(t, validateUnsupportedFeatures(its))
	})

	t.Run("node selector once errors", func(t *testing.T) {
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constant.NodeSelectorOnceAnnotationKey: `{"inst-0":"node-a"}`,
				},
			},
		}
		err := validateUnsupportedFeatures(its)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), constant.NodeSelectorOnceAnnotationKey)
	})
}

func TestOwnedKinds(t *testing.T) {
	kinds := ownedKinds()
	assert.Len(t, kinds, 2)
}

func newAlignedITS(name string, replicas int32) *workloads.InstanceSet {
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Generation: 1,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "busybox:latest"},
					},
				},
			},
		},
	}
}

func newReadyInstance(name string, itsGeneration int64) *workloads.Instance {
	return &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  "default",
			Generation: 1,
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: fmt.Sprintf("%d", itsGeneration),
			},
		},
		Status: workloads.InstanceStatus2{
			ObservedGeneration: 1,
			UpToDate:           true,
			Ready:              true,
			Conditions: []metav1.Condition{
				{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
				{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
			},
		},
	}
}

func TestAlignmentReconciler_Reconcile(t *testing.T) {
	t.Run("aligned instances no-op", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := newAlignedITS("test-its", 2)
		tree.SetRoot(its)

		inst0 := newReadyInstance("test-its-0", 1)
		inst1 := newReadyInstance("test-its-1", 1)
		require.NoError(t, tree.Add(inst0, inst1))

		r := NewAlignmentReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		// Should still have 2 instances
		instances := tree.List(&workloads.Instance{})
		assert.Len(t, instances, 2)
	})

	t.Run("scale up creates new instance", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := newAlignedITS("test-its", 2)
		tree.SetRoot(its)

		// Only 1 existing instance, need 2
		inst0 := newReadyInstance("test-its-0", 1)
		// Make it available so the ordered-ready check passes
		require.NoError(t, tree.Add(inst0))

		r := NewAlignmentReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		instances := tree.List(&workloads.Instance{})
		// Should have created test-its-1
		assert.Len(t, instances, 2)
	})

	t.Run("scale down marks instance for deletion", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := newAlignedITS("test-its", 1)
		tree.SetRoot(its)

		inst0 := newReadyInstance("test-its-0", 1)
		inst1 := newReadyInstance("test-its-1", 1)
		require.NoError(t, tree.Add(inst0, inst1))

		r := NewAlignmentReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		// inst-1 should be marked as ScaledDown
		instances := tree.List(&workloads.Instance{})
		assert.Len(t, instances, 2) // still in tree but marked
		for _, obj := range instances {
			inst := obj.(*workloads.Instance)
			if inst.Name == "test-its-1" {
				assert.True(t, ptr.Deref(inst.Spec.ScaledDown, false), "inst-1 should be marked ScaledDown")
			}
		}
	})

	t.Run("scale down deletes already-scaled-down instance", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := newAlignedITS("test-its", 1)
		tree.SetRoot(its)

		inst0 := newReadyInstance("test-its-0", 1)
		inst1 := newReadyInstance("test-its-1", 1)
		inst1.Spec.ScaledDown = ptr.To(true) // already marked
		require.NoError(t, tree.Add(inst0, inst1))

		r := NewAlignmentReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		instances := tree.List(&workloads.Instance{})
		assert.Len(t, instances, 1)
		assert.Equal(t, "test-its-0", instances[0].GetName())
	})
}

func TestUpdateReconciler_Reconcile(t *testing.T) {
	t.Run("instances not aligned returns early", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := newAlignedITS("test-its", 2)
		tree.SetRoot(its)

		// Only 1 instance but spec says 2 — not aligned
		inst0 := newReadyInstance("test-its-0", 1)
		require.NoError(t, tree.Add(inst0))

		r := NewUpdateReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)
	})

	t.Run("all instances up-to-date no-op", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := newAlignedITS("test-its", 2)
		tree.SetRoot(its)

		inst0 := newReadyInstance("test-its-0", 1)
		inst1 := newReadyInstance("test-its-1", 1)
		require.NoError(t, tree.Add(inst0, inst1))

		r := NewUpdateReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)
	})

	t.Run("OnDelete strategy skips rolling update", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := newAlignedITS("test-its", 2)
		its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{
			Type: "OnDelete",
		}
		tree.SetRoot(its)

		inst0 := newReadyInstance("test-its-0", 1)
		inst1 := newReadyInstance("test-its-1", 1)
		require.NoError(t, tree.Add(inst0, inst1))

		r := NewUpdateReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)
	})

	t.Run("stale instance gets updated", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := newAlignedITS("test-its", 1)
		its.Generation = 2
		tree.SetRoot(its)

		// Instance is from gen 1, ITS is now gen 2
		inst0 := newReadyInstance("test-its-0", 1)
		require.NoError(t, tree.Add(inst0))

		r := NewUpdateReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)
	})
}
