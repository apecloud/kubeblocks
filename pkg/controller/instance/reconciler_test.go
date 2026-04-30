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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

// --- FixMetaReconciler ---

func TestFixMetaReconciler_PreCondition(t *testing.T) {
	r := NewFixMetaReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("already has finalizer unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Finalizers: []string{finalizer}},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("no finalizer satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestFixMetaReconciler_Reconcile(t *testing.T) {
	r := NewFixMetaReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Commit, result)
	assert.Contains(t, tree.GetRoot().GetFinalizers(), finalizer)
}

// --- RevisionUpdateReconciler ---

func TestRevisionUpdateReconciler_PreCondition(t *testing.T) {
	r := NewRevisionUpdateReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("status up to date unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Status:     workloads.InstanceStatus2{ObservedGeneration: 1},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("updating root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 2},
			Status:     workloads.InstanceStatus2{ObservedGeneration: 1},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestRevisionUpdateReconciler_Reconcile(t *testing.T) {
	r := NewRevisionUpdateReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "default", Generation: 3,
			UID: types.UID("test-uid"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
		},
		Status: workloads.InstanceStatus2{ObservedGeneration: 1},
	}
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	updated := tree.GetRoot().(*workloads.Instance)
	assert.NotEmpty(t, updated.Status.UpdateRevision)
	assert.Equal(t, int64(3), updated.Status.ObservedGeneration)
}

// --- StatusReconciler ---

func TestStatusReconciler_PreCondition(t *testing.T) {
	r := NewStatusReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("updating root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 2},
			Status:     workloads.InstanceStatus2{ObservedGeneration: 1},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("status-updating root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Status:     workloads.InstanceStatus2{ObservedGeneration: 1},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestStatusReconciler_Reconcile_NoPod(t *testing.T) {
	r := NewStatusReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "default", Generation: 1,
			UID: types.UID("uid1"),
		},
		Status: workloads.InstanceStatus2{ObservedGeneration: 1},
	}
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
}

// --- UpdateReconciler ---

func TestUpdateReconciler_PreCondition(t *testing.T) {
	r := NewUpdateReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("normal root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

// --- AlignmentReconciler ---

func TestAlignmentReconciler_PreCondition(t *testing.T) {
	r := NewAlignmentReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("normal root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestAlignmentReconciler_Reconcile(t *testing.T) {
	r := NewAlignmentReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "default",
			UID: types.UID("test-uid"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
		},
	}
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	// Should have a pod created
	pods := tree.List(&corev1.Pod{})
	assert.Len(t, pods, 1)
}

// --- AssistantObjectReconciler ---

func TestAssistantObjectReconciler_PreCondition(t *testing.T) {
	r := NewAssistantObjectReconciler()

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("normal root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

func TestAssistantObjectReconciler_Reconcile_NoAssistantObjects(t *testing.T) {
	r := NewAssistantObjectReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
}

func TestAssistantObjectReconciler_Reconcile_WithSharedConfigMap(t *testing.T) {
	r := NewAssistantObjectReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "default",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "cluster",
				constant.KBAppComponentLabelKey: "comp",
			},
		},
		Spec: workloads.InstanceSpec{
			InstanceAssistantObjects: []workloads.InstanceAssistantObject{
				{ConfigMap: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "env-cm", Namespace: "default"},
					Data:       map[string]string{"key": "value"},
				}},
			},
		},
	}
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	// Should have the configmap added
	cms := tree.List(&corev1.ConfigMap{})
	assert.Len(t, cms, 1)
}

// --- DeletionReconciler ---

func TestDeletionReconciler_PreCondition(t *testing.T) {
	r := NewDeletionReconciler(nil)

	t.Run("nil root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("non-deleting root unsatisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionUnsatisfied, r.PreCondition(tree))
	})

	t.Run("deleting root satisfied", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(inst)
		assert.Equal(t, kubebuilderx.ConditionSatisfied, r.PreCondition(tree))
	})
}

// --- buildBlockedCondition ---

func TestBuildBlockedCondition(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Generation: 5},
	}
	cond := buildBlockedCondition(inst, "some reason")
	assert.Equal(t, string(workloads.InstanceUpdateRestricted), cond.Type)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Equal(t, workloads.ReasonInstanceUpdateRestricted, cond.Reason)
	assert.Equal(t, "some reason", cond.Message)
	assert.Equal(t, int64(5), cond.ObservedGeneration)
}

// --- getPodUpdatePolicyInSpec ---

func TestGetPodUpdatePolicyInSpec(t *testing.T) {
	t.Run("init containers differ returns upgrade policy", func(t *testing.T) {
		inst := &workloads.Instance{
			Spec: workloads.InstanceSpec{
				PodUpgradePolicy: kbappsv1.PreferInPlacePodUpdatePolicyType,
				PodUpdatePolicy:  kbappsv1.StrictInPlacePodUpdatePolicyType,
			},
		}
		old := &corev1.Pod{
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{{Name: "init", Image: "old:1"}},
				Containers:     []corev1.Container{{Name: "main", Image: "same:1"}},
			},
		}
		new := &corev1.Pod{
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{{Name: "init", Image: "new:1"}},
				Containers:     []corev1.Container{{Name: "main", Image: "same:1"}},
			},
		}
		assert.Equal(t, kbappsv1.PreferInPlacePodUpdatePolicyType, getPodUpdatePolicyInSpec(inst, old, new))
	})

	t.Run("containers differ returns upgrade policy", func(t *testing.T) {
		inst := &workloads.Instance{
			Spec: workloads.InstanceSpec{
				PodUpgradePolicy: kbappsv1.PreferInPlacePodUpdatePolicyType,
				PodUpdatePolicy:  kbappsv1.StrictInPlacePodUpdatePolicyType,
			},
		}
		old := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "old:1"}},
			},
		}
		new := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "new:1"}},
			},
		}
		assert.Equal(t, kbappsv1.PreferInPlacePodUpdatePolicyType, getPodUpdatePolicyInSpec(inst, old, new))
	})

	t.Run("same containers returns update policy", func(t *testing.T) {
		inst := &workloads.Instance{
			Spec: workloads.InstanceSpec{
				PodUpgradePolicy: kbappsv1.PreferInPlacePodUpdatePolicyType,
				PodUpdatePolicy:  kbappsv1.StrictInPlacePodUpdatePolicyType,
			},
		}
		old := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "main", Image: "same:1"}},
			},
		}
		new := old.DeepCopy()
		assert.Equal(t, kbappsv1.StrictInPlacePodUpdatePolicyType, getPodUpdatePolicyInSpec(inst, old, new))
	})
}

// --- StatusReconciler with failed pod ---

func TestStatusReconciler_Reconcile_WithFailedPod(t *testing.T) {
	r := NewStatusReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "default", Generation: 1,
			UID: types.UID("uid1"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
		},
		Status: workloads.InstanceStatus2{ObservedGeneration: 1},
	}
	tree.SetRoot(inst)

	// Add a failed pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test", Namespace: "default",
			Labels: map[string]string{
				appsv1.ControllerRevisionHashLabelKey: "rev-123",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}
	require.NoError(t, tree.Add(pod))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	updated := tree.GetRoot().(*workloads.Instance)
	assert.False(t, updated.Status.Ready)
	assert.False(t, updated.Status.Available)
	// Should have failure condition set
	assert.NotEmpty(t, updated.Status.Conditions)
}

// --- Tree loader helpers ---

func TestOwnedKinds(t *testing.T) {
	kinds := ownedKinds()
	assert.True(t, len(kinds) >= 5, "expected at least 5 owned kinds")
}

// --- StatusReconciler build helpers ---

func TestBuildReadyCondition(t *testing.T) {
	r := &statusReconciler{}
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
	}

	t.Run("ready", func(t *testing.T) {
		cond := r.buildReadyCondition(inst, true, "")
		assert.Equal(t, string(workloads.InstanceReady), cond.Type)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
		assert.Equal(t, workloads.ReasonReady, cond.Reason)
		assert.Equal(t, int64(3), cond.ObservedGeneration)
	})

	t.Run("not ready", func(t *testing.T) {
		cond := r.buildReadyCondition(inst, false, "inst-0")
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, workloads.ReasonNotReady, cond.Reason)
		assert.Equal(t, "inst-0", cond.Message)
	})
}

func TestBuildAvailableCondition(t *testing.T) {
	r := &statusReconciler{}
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Generation: 5},
	}

	t.Run("available", func(t *testing.T) {
		cond := r.buildAvailableCondition(inst, true, "")
		assert.Equal(t, string(workloads.InstanceAvailable), cond.Type)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
	})

	t.Run("not available", func(t *testing.T) {
		cond := r.buildAvailableCondition(inst, false, "inst-0")
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, workloads.ReasonNotAvailable, cond.Reason)
	})
}

func TestBuildFailureCondition(t *testing.T) {
	r := &statusReconciler{}
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
	}

	t.Run("terminating pod returns nil", func(t *testing.T) {
		now := metav1.Now()
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now, Finalizers: []string{"f"}},
		}
		assert.Nil(t, r.buildFailureCondition(inst, pod))
	})

	t.Run("running pod returns nil", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}
		assert.Nil(t, r.buildFailureCondition(inst, pod))
	})

	t.Run("failed pod returns condition", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "inst-0"},
			Status:     corev1.PodStatus{Phase: corev1.PodFailed},
		}
		cond := r.buildFailureCondition(inst, pod)
		require.NotNil(t, cond)
		assert.Equal(t, string(workloads.InstanceFailure), cond.Type)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
		assert.Equal(t, "inst-0", cond.Message)
	})
}

func TestObservedRoleOfPod(t *testing.T) {
	r := &statusReconciler{}

	t.Run("no roles", func(t *testing.T) {
		inst := &workloads.Instance{}
		pod := &corev1.Pod{}
		assert.Equal(t, "", r.observedRoleOfPod(inst, pod))
	})

	t.Run("with roles but pod not ready", func(t *testing.T) {
		inst := &workloads.Instance{
			Spec: workloads.InstanceSpec{
				Roles: []workloads.ReplicaRole{{Name: "Leader"}},
			},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{constant.RoleLabelKey: "leader"},
			},
		}
		// PodIsReadyWithLabel requires conditions — without them, returns empty
		assert.Equal(t, "", r.observedRoleOfPod(inst, pod))
	})

	t.Run("with roles and ready pod", func(t *testing.T) {
		inst := &workloads.Instance{
			Spec: workloads.InstanceSpec{
				Roles: []workloads.ReplicaRole{
					{Name: "Leader"},
					{Name: "Follower"},
				},
			},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{constant.RoleLabelKey: "leader"},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			},
		}
		assert.Equal(t, "Leader", r.observedRoleOfPod(inst, pod))
	})

	t.Run("with roles and ready pod but unknown role", func(t *testing.T) {
		inst := &workloads.Instance{
			Spec: workloads.InstanceSpec{
				Roles: []workloads.ReplicaRole{{Name: "Leader"}},
			},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{constant.RoleLabelKey: "unknown"},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			},
		}
		assert.Equal(t, "", r.observedRoleOfPod(inst, pod))
	})
}

func TestHasRunningVolumeExpansion(t *testing.T) {
	r := &statusReconciler{}

	t.Run("no pvcs", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "inst-0", Namespace: "default"},
		}
		tree.SetRoot(inst)
		assert.False(t, r.hasRunningVolumeExpansion(tree, inst))
	})

	t.Run("pvc with expansion in progress", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "inst-0", Namespace: "default",
				UID: types.UID("uid1"),
			},
			Spec: workloads.InstanceSpec{
				VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{
					{ObjectMeta: metav1.ObjectMeta{Name: "data"}},
				},
			},
		}
		tree.SetRoot(inst)

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-inst-0",
				Namespace: "default",
				Labels: map[string]string{
					constant.KBAppPodNameLabelKey: "inst-0",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("20Gi"),
					},
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		}
		require.NoError(t, tree.Add(pvc))
		assert.True(t, r.hasRunningVolumeExpansion(tree, inst))
	})

	t.Run("pvc already expanded", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "inst-0", Namespace: "default",
				UID: types.UID("uid1"),
			},
			Spec: workloads.InstanceSpec{
				VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{
					{ObjectMeta: metav1.ObjectMeta{Name: "data"}},
				},
			},
		}
		tree.SetRoot(inst)

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-inst-0",
				Namespace: "default",
				Labels: map[string]string{
					constant.KBAppPodNameLabelKey: "inst-0",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("20Gi"),
					},
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		}
		require.NoError(t, tree.Add(pvc))
		assert.False(t, r.hasRunningVolumeExpansion(tree, inst))
	})
}

func TestCheckObjectProvisionPolicy(t *testing.T) {
	r := &assistantObjectReconciler{}

	t.Run("current ordinal matches", func(t *testing.T) {
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-comp-0"},
		}
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "env-0"}}
		result := r.checkObjectProvisionPolicy(inst, obj)
		assert.NotNil(t, result)
	})

	t.Run("different ordinal skipped", func(t *testing.T) {
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-comp-0"},
		}
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "env-1"}}
		result := r.checkObjectProvisionPolicy(inst, obj)
		assert.Nil(t, result)
	})
}

func TestObservedConfigsOfPod(t *testing.T) {
	r := &statusReconciler{}

	t.Run("no configs", func(t *testing.T) {
		pod := &corev1.Pod{}
		configs, err := r.observedConfigsOfPod(pod)
		assert.NoError(t, err)
		assert.Nil(t, configs)
	})

	t.Run("with configs", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constant.CMInsConfigurationHashLabelKey: `{"cfg1":"hash1"}`,
				},
			},
		}
		configs, err := r.observedConfigsOfPod(pod)
		require.NoError(t, err)
		assert.Len(t, configs, 1)
		assert.Equal(t, "cfg1", configs[0].Name)
	})
}

// --- createOrUpdateOwned (ordinal assistant objects) ---

func TestCreateOrUpdateOwned_Create(t *testing.T) {
	inst := newTestInstance("cluster-component-0",
		ordinalConfigMapAssistantObject("env-0"))
	tree := newTestTree(inst)

	reconciler := NewAssistantObjectReconciler().(*assistantObjectReconciler)
	err := reconciler.createOrUpdate(tree, inst, inst.Spec.InstanceAssistantObjects[0])
	require.NoError(t, err)

	cms := tree.List(&corev1.ConfigMap{})
	require.Len(t, cms, 1)
	cm := cms[0].(*corev1.ConfigMap)
	// ordinal objects get instance labels
	assert.Contains(t, cm.Labels, constant.AppManagedByLabelKey)
	// ordinal objects get owner reference
	assert.Len(t, cm.OwnerReferences, 1)
}

func TestCreateOrUpdateOwned_Update(t *testing.T) {
	inst := newTestInstance("cluster-component-0",
		ordinalConfigMapAssistantObject("env-0"))
	tree := newTestTree(inst)

	reconciler := NewAssistantObjectReconciler().(*assistantObjectReconciler)
	// First create
	require.NoError(t, reconciler.createOrUpdate(tree, inst, inst.Spec.InstanceAssistantObjects[0]))

	// Update with new data
	inst.Spec.InstanceAssistantObjects[0].ConfigMap.Data["k"] = "updated"
	require.NoError(t, reconciler.createOrUpdate(tree, inst, inst.Spec.InstanceAssistantObjects[0]))

	cms := tree.List(&corev1.ConfigMap{})
	require.Len(t, cms, 1)
	cm := cms[0].(*corev1.ConfigMap)
	assert.Equal(t, "updated", cm.Data["k"])
}

// --- DeletionReconciler.Reconcile ---

func TestDeletionReconciler_Reconcile_DeleteAll(t *testing.T) {
	now := metav1.Now()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID:               types.UID("uid1"),
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizer},
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    testCluster,
				constant.KBAppComponentLabelKey: testComponent,
			},
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
		},
	}
	cli := newFakeClient(t, inst)
	tree := newTestTree(inst)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
		},
	}
	require.NoError(t, tree.Add(pod))

	r := NewDeletionReconciler(cli)
	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	// Pod was deleted as a secondary object, so the first reconcile returns after that.
	// The pod should be gone.
	pods := tree.List(&corev1.Pod{})
	assert.Len(t, pods, 0)
}

func TestDeletionReconciler_Reconcile_NoSecondaryObjects(t *testing.T) {
	now := metav1.Now()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID:               types.UID("uid1"),
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizer},
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    testCluster,
				constant.KBAppComponentLabelKey: testComponent,
			},
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
		},
	}
	cli := newFakeClient(t, inst)
	tree := newTestTree(inst)
	// No secondary objects — root should be deleted directly

	r := NewDeletionReconciler(cli)
	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
	assert.Nil(t, tree.GetRoot())
}

func TestDeletionReconciler_Reconcile_RetainPVC(t *testing.T) {
	now := metav1.Now()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID:               types.UID("uid1"),
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizer},
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    testCluster,
				constant.KBAppComponentLabelKey: testComponent,
			},
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
			PersistentVolumeClaimRetentionPolicy: &kbappsv1.PersistentVolumeClaimRetentionPolicy{
				WhenDeleted: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
		},
	}
	cli := newFakeClient(t, inst)
	tree := newTestTree(inst)

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data-inst-0", Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: inst.UID, Name: inst.Name, Kind: "Instance", APIVersion: "workloads.kubeblocks.io/v1"},
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			},
		},
	}
	require.NoError(t, tree.Add(pvc))

	r := NewDeletionReconciler(cli)
	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
}

func TestDeletionReconciler_Reconcile_ScaledDown_RetainPVC(t *testing.T) {
	now := metav1.Now()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID:               types.UID("uid1"),
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizer},
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    testCluster,
				constant.KBAppComponentLabelKey: testComponent,
			},
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
			ScaledDown: ptr.To(true),
			PersistentVolumeClaimRetentionPolicy: &kbappsv1.PersistentVolumeClaimRetentionPolicy{
				WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
		},
	}
	cli := newFakeClient(t, inst)
	tree := newTestTree(inst)

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data-inst-0", Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{UID: inst.UID, Name: inst.Name, Kind: "Instance", APIVersion: "workloads.kubeblocks.io/v1"},
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			},
		},
	}
	require.NoError(t, tree.Add(pvc))

	r := NewDeletionReconciler(cli)
	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
}

// --- loadAssistantObjects ---

func TestLoadAssistantObjects(t *testing.T) {
	t.Run("nil root", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		cli := newFakeClient(t)
		err := loadAssistantObjects(context.Background(), cli, tree)
		assert.NoError(t, err)
	})

	t.Run("loads existing objects", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "env-cm", Namespace: "default",
			},
			Data: map[string]string{"key": "value"},
		}
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "inst-0", Namespace: "default",
				UID: types.UID("uid1"),
			},
			Spec: workloads.InstanceSpec{
				InstanceAssistantObjects: []workloads.InstanceAssistantObject{
					{ConfigMap: &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "env-cm", Namespace: "default"},
					}},
				},
			},
		}
		cli := newFakeClient(t, cm)
		tree := newTestTree(inst)

		err := loadAssistantObjects(context.Background(), cli, tree)
		require.NoError(t, err)
		cms := tree.List(&corev1.ConfigMap{})
		assert.Len(t, cms, 1)
	})

	t.Run("skips not found objects", func(t *testing.T) {
		inst := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "inst-0", Namespace: "default",
				UID: types.UID("uid1"),
			},
			Spec: workloads.InstanceSpec{
				InstanceAssistantObjects: []workloads.InstanceAssistantObject{
					{ConfigMap: &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "missing-cm", Namespace: "default"},
					}},
				},
			},
		}
		cli := newFakeClient(t)
		tree := newTestTree(inst)

		err := loadAssistantObjects(context.Background(), cli, tree)
		assert.NoError(t, err)
		cms := tree.List(&corev1.ConfigMap{})
		assert.Len(t, cms, 0)
	})
}

// --- AlignmentReconciler with PVCs ---

func TestAlignmentReconciler_Reconcile_WithPVCs(t *testing.T) {
	r := NewAlignmentReconciler()
	tree := kubebuilderx.NewObjectTree()
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
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("10Gi"),
							},
						},
					},
				},
			},
		},
	}
	tree.SetRoot(inst)

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	// Should have a pod and a PVC created
	pods := tree.List(&corev1.Pod{})
	assert.Len(t, pods, 1)
	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	assert.Len(t, pvcs, 1)
}

func TestAlignmentReconciler_Reconcile_DeleteStalePVC(t *testing.T) {
	r := NewAlignmentReconciler()
	tree := kubebuilderx.NewObjectTree()
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
			// no VolumeClaimTemplates — PVCs should be deleted
		},
	}
	tree.SetRoot(inst)

	// Add a stale PVC that is no longer desired
	stalePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data-inst-0", Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			},
		},
	}
	require.NoError(t, tree.Add(stalePVC))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	assert.Len(t, pvcs, 0)
}

// --- UpdateReconciler.Reconcile ---

func TestUpdateReconciler_Reconcile_OnDeleteStrategy(t *testing.T) {
	r := NewUpdateReconciler()
	tree := kubebuilderx.NewObjectTree()
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
			InstanceUpdateStrategyType: ptr.To(kbappsv1.OnDeleteStrategyType),
		},
	}
	tree.SetRoot(inst)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
		},
	}
	require.NoError(t, tree.Add(pod))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
	// Pod should still exist (OnDelete skips updates)
	pods := tree.List(&corev1.Pod{})
	assert.Len(t, pods, 1)
}

func TestUpdateReconciler_Reconcile_NotAligned(t *testing.T) {
	r := NewUpdateReconciler()
	tree := kubebuilderx.NewObjectTree()
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
	tree.SetRoot(inst)
	// No pod in tree — not aligned, should return early

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
}

func TestUpdateReconciler_Reconcile_DeletePendingPod(t *testing.T) {
	r := NewUpdateReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID: types.UID("uid1"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:2.0"}},
				},
			},
		},
		Status: workloads.InstanceStatus2{
			UpdateRevision: "different-rev",
		},
	}
	tree.SetRoot(inst)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			Labels: map[string]string{
				appsv1.ControllerRevisionHashLabelKey: "old-rev",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
	require.NoError(t, tree.Add(pod))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
	// Pending pod should be deleted
	pods := tree.List(&corev1.Pod{})
	assert.Len(t, pods, 0)
}

// --- getPodUpdatePolicy paths ---

func TestGetPodUpdatePolicy_NoOps(t *testing.T) {
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

	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	pod.Status.Phase = corev1.PodRunning

	// Set UpdateRevision to match
	inst.Status.UpdateRevision = getPodRevision(pod)

	policy, _, err := getPodUpdatePolicy(inst, pod)
	require.NoError(t, err)
	assert.Equal(t, noOpsPolicy, policy)
}

func TestGetPodUpdatePolicy_EmptyUpdateRevision(t *testing.T) {
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
		Status: workloads.InstanceStatus2{
			UpdateRevision: "", // empty
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			Labels: map[string]string{
				appsv1.ControllerRevisionHashLabelKey: "different-rev",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
		},
	}

	policy, _, err := getPodUpdatePolicy(inst, pod)
	require.NoError(t, err)
	// empty UpdateRevision → noOps
	assert.Equal(t, noOpsPolicy, policy)
}

func TestGetPodUpdatePolicy_InPlaceBasicUpdate(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID: types.UID("uid1"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"new-key": "new-val"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
		},
	}

	// Build the pod from the instance
	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	// Set UpdateRevision to match pod revision so it's not a revision mismatch
	inst.Status.UpdateRevision = getPodRevision(pod)

	// Now mutate the annotation on the pod so it differs
	pod.Annotations = map[string]string{"old-key": "old-val"}

	policy, _, err := getPodUpdatePolicy(inst, pod)
	require.NoError(t, err)
	assert.Equal(t, inPlaceUpdatePolicy, policy)
}

// --- StatusReconciler with ready pod ---

func TestStatusReconciler_Reconcile_ReadyPod(t *testing.T) {
	r := NewStatusReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID:        types.UID("uid1"),
			Generation: 1,
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
		},
	}
	tree.SetRoot(inst)

	// Build a proper pod and set it as ready
	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
	}
	inst.Status.UpdateRevision = getPodRevision(pod)
	require.NoError(t, tree.Add(pod))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	updated := tree.GetRoot().(*workloads.Instance)
	assert.True(t, updated.Status.Ready)
	assert.True(t, updated.Status.UpToDate)
}

// --- deleteSecondaryObjects with assistant objects ---

func TestDeleteSecondaryObjects_WithAssistantObjects(t *testing.T) {
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-component-0", Namespace: "default",
			UID: types.UID("uid1"),
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    testCluster,
				constant.KBAppComponentLabelKey: testComponent,
			},
		},
		Spec: workloads.InstanceSpec{
			InstanceAssistantObjects: []workloads.InstanceAssistantObject{
				sharedConfigMapAssistantObject("cluster-component-env", "v1"),
			},
		},
	}

	shared := managedSharedConfigMap("cluster-component-env")
	tree := newTestTree(inst)
	require.NoError(t, tree.Add(shared))

	cli := newFakeClient(t, inst)
	r := NewDeletionReconciler(cli).(*deletionReconciler)
	has, err := r.deleteSecondaryObjects(tree, inst, false)
	require.NoError(t, err)
	// shared object should be skipped (not deleted)
	assert.False(t, has)
}

// --- AssistantObjectReconciler.Reconcile with ordinal ---

func TestAssistantObjectReconciler_Reconcile_WithOrdinalConfigMap(t *testing.T) {
	inst := newTestInstance("cluster-component-0",
		ordinalConfigMapAssistantObject("env-0"))
	tree := newTestTree(inst)

	r := NewAssistantObjectReconciler()
	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	cms := tree.List(&corev1.ConfigMap{})
	assert.Len(t, cms, 1)
}

// --- NewTreeLoader ---

func TestNewTreeLoader(t *testing.T) {
	loader := NewTreeLoader()
	assert.NotNil(t, loader)
	_, ok := loader.(*treeLoader)
	assert.True(t, ok)
}

// --- assistantObjectKey ---

func TestAssistantObjectKey(t *testing.T) {
	t.Run("valid configmap", func(t *testing.T) {
		ao := workloads.InstanceAssistantObject{
			ConfigMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "default"},
			},
		}
		obj, name, err := assistantObjectKey(ao)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.NotNil(t, name)
	})

	t.Run("empty object", func(t *testing.T) {
		ao := workloads.InstanceAssistantObject{}
		obj, name, err := assistantObjectKey(ao)
		assert.NoError(t, err)
		assert.Nil(t, obj)
		assert.Nil(t, name)
	})
}

// --- skipAssistantObjectSecondaryDeletion ---

func TestSkipAssistantObjectSecondaryDeletion(t *testing.T) {
	inst := newTestInstance("cluster-component-0")

	t.Run("shared object returns true", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shared-cm"},
		}
		assert.True(t, skipAssistantObjectSecondaryDeletion(inst, obj))
	})

	t.Run("current ordinal returns false", func(t *testing.T) {
		obj := ordinalConfigMap("env-0")
		assert.False(t, skipAssistantObjectSecondaryDeletion(inst, obj))
	})

	t.Run("other ordinal returns true", func(t *testing.T) {
		obj := ordinalConfigMap("env-1")
		assert.True(t, skipAssistantObjectSecondaryDeletion(inst, obj))
	})
}

// --- sharedAssistantObjectReferencedByOthers ---

func TestSharedAssistantObjectReferencedByOthers(t *testing.T) {
	t.Run("referenced by another instance", func(t *testing.T) {
		inst0 := newTestInstance("cluster-component-0",
			sharedConfigMapAssistantObject("cluster-component-env", "v1"))
		inst1 := newTestInstance("cluster-component-1",
			sharedConfigMapAssistantObject("cluster-component-env", "v1"))

		_, objKey, err := assistantObjectKey(inst0.Spec.InstanceAssistantObjects[0])
		require.NoError(t, err)
		referenced, err := sharedAssistantObjectReferencedByOthers(inst0, *objKey, []workloads.Instance{*inst0, *inst1})
		require.NoError(t, err)
		assert.True(t, referenced)
	})

	t.Run("not referenced by any other instance", func(t *testing.T) {
		inst0 := newTestInstance("cluster-component-0",
			sharedConfigMapAssistantObject("cluster-component-env", "v1"))

		_, objKey, err := assistantObjectKey(inst0.Spec.InstanceAssistantObjects[0])
		require.NoError(t, err)
		referenced, err := sharedAssistantObjectReferencedByOthers(inst0, *objKey, []workloads.Instance{*inst0})
		require.NoError(t, err)
		assert.False(t, referenced)
	})
}

// --- mergeInPlaceFields with tolerations ---

func TestMergeInPlaceFields_Tolerations(t *testing.T) {
	src := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1", Image: "img:1"}},
			Tolerations: []corev1.Toleration{
				{Key: "key1", Effect: corev1.TaintEffectNoSchedule},
				{Key: "key2", Effect: corev1.TaintEffectNoExecute},
			},
		},
	}
	dst := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1", Image: "img:old"}},
			Tolerations: []corev1.Toleration{
				{Key: "key1", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}

	mergeInPlaceFields(src, dst)
	assert.Equal(t, "img:1", dst.Spec.Containers[0].Image)
	// key2 should be merged
	assert.GreaterOrEqual(t, len(dst.Spec.Tolerations), 2)
}

// --- isPodUpdated paths ---

func TestIsPodUpdated_DifferentRevision(t *testing.T) {
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
		Status: workloads.InstanceStatus2{
			UpdateRevision: "new-rev",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			Labels: map[string]string{
				appsv1.ControllerRevisionHashLabelKey: "old-rev",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	updated, err := isPodUpdated(inst, pod)
	require.NoError(t, err)
	assert.False(t, updated)
}

// --- deleteUnreferencedSharedAssistantObjects more paths ---

func TestDeleteUnreferencedSharedAssistantObjects_NonManaged(t *testing.T) {
	// A non-managed (no annotation) ConfigMap should not be deleted
	nonManaged := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-component-env",
			Namespace: testNamespace,
		},
		Data: map[string]string{"k": "v"},
	}
	inst := newTestInstance("cluster-component-0")
	cli := newFakeClient(t, inst)
	tree := newTestTree(inst)
	require.NoError(t, tree.Add(nonManaged))

	r := NewDeletionReconciler(cli).(*deletionReconciler)
	err := r.deleteUnreferencedSharedAssistantObjects(tree, inst)
	require.NoError(t, err)

	// Non-managed object should remain
	obj, err := tree.Get(nonManaged)
	require.NoError(t, err)
	assert.NotNil(t, obj)
}

// --- UpdateReconciler with a ready, up-to-date pod (noOps path) ---

func TestUpdateReconciler_Reconcile_ReadyPodNoOps(t *testing.T) {
	r := NewUpdateReconciler()
	tree := kubebuilderx.NewObjectTree()
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
	tree.SetRoot(inst)

	// Build a pod that matches the inst spec exactly
	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
	}
	// Set container statuses so image matching works
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{Name: "main", Image: "nginx:1.19", Ready: true},
	}
	inst.Status.UpdateRevision = getPodRevision(pod)
	require.NoError(t, tree.Add(pod))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
	// Pod should still exist — no update needed
	pods := tree.List(&corev1.Pod{})
	assert.Len(t, pods, 1)
}

// --- UpdateReconciler with StrictInPlace blocked ---

func TestUpdateReconciler_Reconcile_StrictInPlaceBlocked(t *testing.T) {
	r := NewUpdateReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID: types.UID("uid1"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					// different init container to trigger PodUpgradePolicy (recreate)
					InitContainers: []corev1.Container{{Name: "init", Image: "busybox:2.0"}},
					Containers:     []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
				},
			},
			PodUpdatePolicy: kbappsv1.StrictInPlacePodUpdatePolicyType,
			Configs: []workloads.ConfigTemplate{{
				Name:       "cfg1",
				ConfigHash: ptr.To("hash-new"),
				Restart:    ptr.To(true),
			}},
		},
	}
	tree.SetRoot(inst)

	// Build a pod that has different config (triggers recreate via config restart)
	pod, err := buildInstancePod(inst, "")
	require.NoError(t, err)
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.PodReady, Status: corev1.ConditionTrue},
	}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{Name: "main", Image: "nginx:1.19", Ready: true},
	}
	// Set old config hash on pod
	require.NoError(t, configsToPod([]workloads.ConfigTemplate{{
		Name:       "cfg1",
		ConfigHash: ptr.To("hash-old"),
	}}, pod))
	inst.Status.UpdateRevision = getPodRevision(pod)
	require.NoError(t, tree.Add(pod))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	// Should have blocked condition set
	updated := tree.GetRoot().(*workloads.Instance)
	found := false
	for _, cond := range updated.Status.Conditions {
		if cond.Type == string(workloads.InstanceUpdateRestricted) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected InstanceUpdateRestricted condition")
}

// --- UpdateReconciler with unready pod (blocks update) ---

func TestUpdateReconciler_Reconcile_UnreadyPodBlocks(t *testing.T) {
	r := NewUpdateReconciler()
	tree := kubebuilderx.NewObjectTree()
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			UID: types.UID("uid1"),
		},
		Spec: workloads.InstanceSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "main", Image: "nginx:2.0"}},
				},
			},
		},
		Status: workloads.InstanceStatus2{
			UpdateRevision: "new-rev",
		},
	}
	tree.SetRoot(inst)

	// A running but not-ready pod should block update
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			Labels: map[string]string{
				appsv1.ControllerRevisionHashLabelKey: "old-rev",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx:1.19"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "main", Image: "nginx:1.19", Ready: false},
			},
		},
	}
	require.NoError(t, tree.Add(pod))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)
	// Pod should still exist — not updated because it's not ready
	pods := tree.List(&corev1.Pod{})
	assert.Len(t, pods, 1)
}

// --- AlignmentReconciler with PVC update ---

func TestAlignmentReconciler_Reconcile_UpdatePVC(t *testing.T) {
	r := NewAlignmentReconciler()
	tree := kubebuilderx.NewObjectTree()
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
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("20Gi"),
							},
						},
					},
				},
			},
		},
	}
	tree.SetRoot(inst)

	// Add an existing PVC with smaller size
	existingPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data-inst-0", Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	require.NoError(t, tree.Add(existingPVC))

	result, err := r.Reconcile(tree)
	require.NoError(t, err)
	assert.Equal(t, kubebuilderx.Continue, result)

	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	assert.Len(t, pvcs, 1)
	pvc := pvcs[0].(*corev1.PersistentVolumeClaim)
	// PVC should be updated to 20Gi
	storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	assert.Equal(t, resource.MustParse("20Gi"), storage)
}

// --- isPodUpdated with terminating pod ---

func TestIsPodUpdated_TerminatingPod(t *testing.T) {
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
	now := metav1.Now()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inst-0", Namespace: "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{"test"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	// Terminating pods are handled by isCreated && !isTerminating check in status reconciler
	// but isPodUpdated itself should still work
	updated, err := isPodUpdated(inst, pod)
	require.NoError(t, err)
	assert.False(t, updated) // Different revision
}

// --- equalField with different kinds ---

func TestEqualField_DifferentTypes(t *testing.T) {
	assert.False(t, equalField("string", 123))
}

// --- mergeInPlaceFields with init container name mismatch ---

func TestMergeInPlaceFields_InitContainerNameMismatch(t *testing.T) {
	src := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{{Name: "c1", Image: "img:1"}},
			InitContainers: []corev1.Container{{Name: "init-src", Image: "busybox:new"}},
		},
	}
	dst := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers:     []corev1.Container{{Name: "c1", Image: "img:old"}},
			InitContainers: []corev1.Container{{Name: "init-dst", Image: "busybox:old"}},
		},
	}

	mergeInPlaceFields(src, dst)
	// Container image should be merged
	assert.Equal(t, "img:1", dst.Spec.Containers[0].Image)
	// Init container with different name should not be merged
	assert.Equal(t, "busybox:old", dst.Spec.InitContainers[0].Image)
}

