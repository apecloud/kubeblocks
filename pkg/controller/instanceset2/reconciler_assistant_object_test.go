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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestGetHeadlessSvcSelector(t *testing.T) {
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "myapp",
					"env": "prod",
				},
			},
		},
	}
	selectors := getHeadlessSvcSelector(its)
	assert.Equal(t, "myapp", selectors["app"])
	assert.Equal(t, "prod", selectors["env"])
	assert.Equal(t, constant.ReleasePhaseStable, selectors[constant.KBAppReleasePhaseKey])
}

func TestBuildHeadlessSvc(t *testing.T) {
	t.Run("basic headless service", func(t *testing.T) {
		its := workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-its",
				Namespace: "default",
			},
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "main",
								Ports: []corev1.ContainerPort{
									{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
									{Name: "grpc", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
								},
							},
						},
					},
				},
			},
		}
		labels := map[string]string{"l": "v"}
		selectors := map[string]string{"s": "v"}

		svc := buildHeadlessSvc(its, labels, selectors)
		assert.Equal(t, "my-its-headless", svc.Name)
		assert.Equal(t, "default", svc.Namespace)
		assert.Equal(t, "v", svc.Labels["l"])
		assert.True(t, svc.Spec.PublishNotReadyAddresses)
		assert.Len(t, svc.Spec.Ports, 2)
		assert.Equal(t, "http", svc.Spec.Ports[0].Name)
		assert.Equal(t, int32(8080), svc.Spec.Ports[0].Port)
	})

	t.Run("duplicate port names get fallback name", func(t *testing.T) {
		its := workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "c1",
								Ports: []corev1.ContainerPort{
									{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
									{Name: "http", ContainerPort: 8081, Protocol: corev1.ProtocolTCP},
								},
							},
						},
					},
				},
			},
		}
		svc := buildHeadlessSvc(its, nil, nil)
		assert.Len(t, svc.Spec.Ports, 2)
		assert.Equal(t, "http", svc.Spec.Ports[0].Name)
		// Second port with same name gets a fallback
		assert.Equal(t, "tcp-8081", svc.Spec.Ports[1].Name)
	})

	t.Run("unnamed ports get fallback name", func(t *testing.T) {
		its := workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
			Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "c1",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 3306, Protocol: corev1.ProtocolTCP},
								},
							},
						},
					},
				},
			},
		}
		svc := buildHeadlessSvc(its, nil, nil)
		assert.Len(t, svc.Spec.Ports, 1)
		assert.Equal(t, "tcp-3306", svc.Spec.Ports[0].Name)
	})
}

func TestBuildConditionMessageWithNames(t *testing.T) {
	t.Run("sorted output", func(t *testing.T) {
		names := []string{"inst-2", "inst-0", "inst-1"}
		msg, err := buildConditionMessageWithNames(names)
		assert.NoError(t, err)
		// reverse=true reverses the default desc sort, so result is ascending
		assert.Equal(t, `["inst-0","inst-1","inst-2"]`, string(msg))
	})

	t.Run("empty names", func(t *testing.T) {
		msg, err := buildConditionMessageWithNames(nil)
		assert.NoError(t, err)
		assert.Equal(t, `null`, string(msg))
	})
}

func TestBuildTemplatesStatus(t *testing.T) {
	t.Run("filters empty name", func(t *testing.T) {
		m := map[string]*workloads.InstanceTemplateStatus{
			"":     {Name: "", Replicas: 1},
			"tpl1": {Name: "tpl1", Replicas: 2},
			"tpl2": {Name: "tpl2", Replicas: 3},
		}
		result := buildTemplatesStatus(m)
		assert.Len(t, result, 2)
		// sorted by name
		assert.Equal(t, "tpl1", result[0].Name)
		assert.Equal(t, "tpl2", result[1].Name)
	})

	t.Run("empty map", func(t *testing.T) {
		result := buildTemplatesStatus(map[string]*workloads.InstanceTemplateStatus{})
		assert.Empty(t, result)
	})
}

func TestBuildReadyCondition(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
	}

	t.Run("all ready", func(t *testing.T) {
		cond, err := buildReadyCondition(its, true, nil)
		assert.NoError(t, err)
		assert.Equal(t, string(workloads.InstanceReady), cond.Type)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
		assert.Equal(t, workloads.ReasonReady, cond.Reason)
		assert.Equal(t, int64(3), cond.ObservedGeneration)
	})

	t.Run("not ready", func(t *testing.T) {
		notReady := sets.New[string]("inst-0", "inst-1")
		cond, err := buildReadyCondition(its, false, notReady)
		assert.NoError(t, err)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, workloads.ReasonNotReady, cond.Reason)
		assert.NotEmpty(t, cond.Message)
	})
}

func TestBuildAvailableCondition(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 5},
	}

	t.Run("all available", func(t *testing.T) {
		cond, err := buildAvailableCondition(its, true, nil)
		assert.NoError(t, err)
		assert.Equal(t, string(workloads.InstanceAvailable), cond.Type)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
	})

	t.Run("not available", func(t *testing.T) {
		notAvail := sets.New[string]("inst-0")
		cond, err := buildAvailableCondition(its, false, notAvail)
		assert.NoError(t, err)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, workloads.ReasonNotAvailable, cond.Reason)
	})
}

func TestBuildFailureCondition(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
	}

	t.Run("no failures returns nil", func(t *testing.T) {
		instances := []*workloads.Instance{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "inst-0", Generation: 1},
				Status:     workloads.InstanceStatus2{ObservedGeneration: 1, UpToDate: true, Ready: true},
			},
		}
		cond, err := buildFailureCondition(its, instances)
		assert.NoError(t, err)
		assert.Nil(t, cond)
	})
}

func TestSortInstanceStatus(t *testing.T) {
	statuses := []workloads.InstanceStatus{
		{PodName: "foo-1"},
		{PodName: "foo-0"},
		{PodName: "foo-2"},
	}
	sortInstanceStatus(statuses)
	// reverse=true reverses the default desc sort → ascending ordinals
	assert.Equal(t, "foo-0", statuses[0].PodName)
	assert.Equal(t, "foo-1", statuses[1].PodName)
	assert.Equal(t, "foo-2", statuses[2].PodName)
}

func TestSyncMemberStatus(t *testing.T) {
	t.Run("no roles does nothing", func(t *testing.T) {
		its := &workloads.InstanceSet{}
		statuses := []workloads.InstanceStatus{{PodName: "inst-0"}}
		instances := []*workloads.Instance{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "inst-0"},
				Status:     workloads.InstanceStatus2{Role: "leader"},
			},
		}
		syncMemberStatus(its, statuses, instances)
		assert.Equal(t, "", statuses[0].Role)
	})
}

func TestSyncInstancePVCStatus(t *testing.T) {
	its := &workloads.InstanceSet{}
	statuses := []workloads.InstanceStatus{{PodName: "inst-0"}}
	instances := []*workloads.Instance{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "inst-0"},
			Status:     workloads.InstanceStatus2{VolumeExpansion: true},
		},
	}
	syncInstancePVCStatus(its, statuses, instances)
	assert.True(t, statuses[0].VolumeExpansion)
}

func TestBuildFailureConditionWithFailures(t *testing.T) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
	}
	instances := []*workloads.Instance{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "inst-0", Generation: 1},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 1,
				UpToDate:           true,
				Conditions: []metav1.Condition{
					{
						Type:   string(workloads.InstanceFailure),
						Status: metav1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "inst-1", Generation: 1},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 1,
				UpToDate:           true,
				Ready:              true,
			},
		},
	}
	cond, err := buildFailureCondition(its, instances)
	assert.NoError(t, err)
	assert.NotNil(t, cond)
	assert.Equal(t, string(workloads.InstanceFailure), cond.Type)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Contains(t, cond.Message, "inst-0")
}

func TestSyncMemberStatusWithRoles(t *testing.T) {
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Roles: []workloads.ReplicaRole{
				{Name: "Leader", UpdatePriority: 3},
				{Name: "Follower", UpdatePriority: 2},
			},
		},
	}
	statuses := []workloads.InstanceStatus{
		{PodName: "inst-0"},
		{PodName: "inst-1"},
	}
	instances := []*workloads.Instance{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "inst-0", Generation: 1},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 1,
				UpToDate:           true,
				Ready:              true,
				Role:               "leader",
				Conditions: []metav1.Condition{
					{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "inst-1", Generation: 1},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 1,
				UpToDate:           true,
				Ready:              true,
				Role:               "follower",
				Conditions: []metav1.Condition{
					{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
				},
			},
		},
	}
	syncMemberStatus(its, statuses, instances)
	assert.Equal(t, "Leader", statuses[0].Role)
	assert.Equal(t, "Follower", statuses[1].Role)
}

func TestAssistantObjectReconciler_Reconcile(t *testing.T) {
	t.Run("creates headless service for non-multicluster ITS", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(3)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test-its", Namespace: "default"},
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "myapp"},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "main",
								Ports: []corev1.ContainerPort{
									{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								},
							},
						},
					},
				},
			},
		}
		tree.SetRoot(its)

		r := NewAssistantObjectReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		svcList := tree.List(&corev1.Service{})
		require.Len(t, svcList, 1)
		svc := svcList[0].(*corev1.Service)
		assert.Equal(t, "test-its-headless", svc.Name)
		assert.True(t, svc.Spec.PublishNotReadyAddresses)
		assert.Equal(t, int32(8080), svc.Spec.Ports[0].Port)
	})

	t.Run("skips headless service for multicluster ITS", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(3)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-its",
				Namespace: "default",
				Annotations: map[string]string{
					constant.KBAppMultiClusterPlacementKey: "cluster-a",
				},
			},
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "myapp"},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "main",
								Ports: []corev1.ContainerPort{
									{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								},
							},
						},
					},
				},
			},
		}
		tree.SetRoot(its)

		r := NewAssistantObjectReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		svcList := tree.List(&corev1.Service{})
		assert.Empty(t, svcList)
	})

	t.Run("disableDefaultHeadlessService skips service creation", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(1)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test-its", Namespace: "default"},
			Spec: workloads.InstanceSetSpec{
				Replicas:                      &replicas,
				DisableDefaultHeadlessService: true,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "myapp"},
				},
			},
		}
		tree.SetRoot(its)

		r := NewAssistantObjectReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)
		assert.Empty(t, tree.List(&corev1.Service{}))
	})

	t.Run("updates existing headless service", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(1)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test-its", Namespace: "default"},
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "myapp"},
				},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "main",
								Ports: []corev1.ContainerPort{
									{Name: "http", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
								},
							},
						},
					},
				},
			},
		}
		tree.SetRoot(its)

		// Pre-existing headless service with old port
		oldSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test-its-headless", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				ClusterIP: "None",
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 8080, Protocol: corev1.ProtocolTCP},
				},
			},
		}
		require.NoError(t, tree.Add(oldSvc))

		r := NewAssistantObjectReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		svcList := tree.List(&corev1.Service{})
		require.Len(t, svcList, 1)
		svc := svcList[0].(*corev1.Service)
		assert.Equal(t, int32(9090), svc.Spec.Ports[0].Port)
	})

	t.Run("deletes stale service", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(1)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test-its", Namespace: "default"},
			Spec: workloads.InstanceSetSpec{
				Replicas:                      &replicas,
				DisableDefaultHeadlessService: true,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "myapp"},
				},
			},
		}
		tree.SetRoot(its)

		// Stale service that should be deleted
		staleSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "stale-svc", Namespace: "default"},
		}
		require.NoError(t, tree.Add(staleSvc))

		r := NewAssistantObjectReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)
		assert.Empty(t, tree.List(&corev1.Service{}))
	})
}

func TestSetInstanceStatus(t *testing.T) {
	t.Run("sets status for instances", func(t *testing.T) {
		its := &workloads.InstanceSet{}
		instances := []*workloads.Instance{
			{ObjectMeta: metav1.ObjectMeta{Name: "inst-1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "inst-0"}},
		}

		setInstanceStatus(its, instances)

		require.Len(t, its.Status.InstanceStatus, 2)
		// should be sorted by ordinal
		assert.Equal(t, "inst-0", its.Status.InstanceStatus[0].PodName)
		assert.Equal(t, "inst-1", its.Status.InstanceStatus[1].PodName)
	})

	t.Run("syncs config and PVC status", func(t *testing.T) {
		its := &workloads.InstanceSet{}
		instances := []*workloads.Instance{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "inst-0"},
				Status: workloads.InstanceStatus2{
					VolumeExpansion: true,
					Configs: []workloads.InstanceConfigStatus{
						{Name: "cfg1"},
					},
				},
			},
		}

		setInstanceStatus(its, instances)
		require.Len(t, its.Status.InstanceStatus, 1)
		assert.True(t, its.Status.InstanceStatus[0].VolumeExpansion)
		assert.Len(t, its.Status.InstanceStatus[0].Configs, 1)
	})
}

func TestStatusReconciler_Reconcile(t *testing.T) {
	t.Run("no instances", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(0)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
			},
			Status: workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)

		r := NewStatusReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		updatedIts := tree.GetRoot().(*workloads.InstanceSet)
		assert.Equal(t, int32(0), updatedIts.Status.Replicas)
		assert.Equal(t, int32(0), updatedIts.Status.ReadyReplicas)
	})

	t.Run("all instances ready and available", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(2)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
			},
			Status: workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)

		inst0 := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-0",
				Generation:  1,
				Annotations: map[string]string{constant.KubeBlocksGenerationKey: "1"},
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
		inst1 := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-1",
				Generation:  1,
				Annotations: map[string]string{constant.KubeBlocksGenerationKey: "1"},
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
		require.NoError(t, tree.Add(inst0, inst1))

		r := NewStatusReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		updatedIts := tree.GetRoot().(*workloads.InstanceSet)
		assert.Equal(t, int32(2), updatedIts.Status.Replicas)
		assert.Equal(t, int32(2), updatedIts.Status.ReadyReplicas)
		assert.Equal(t, int32(2), updatedIts.Status.AvailableReplicas)
		assert.Equal(t, int32(2), updatedIts.Status.UpdatedReplicas)
		assert.Equal(t, int32(2), updatedIts.Status.CurrentReplicas)
	})

	t.Run("some instances not ready", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(2)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
			},
			Status: workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)

		inst0 := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-0",
				Generation:  1,
				Annotations: map[string]string{constant.KubeBlocksGenerationKey: "1"},
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
		inst1 := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-1",
				Generation:  1,
				Annotations: map[string]string{constant.KubeBlocksGenerationKey: "1"},
			},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 1,
				UpToDate:           true,
			},
		}
		require.NoError(t, tree.Add(inst0, inst1))

		r := NewStatusReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		assert.Equal(t, kubebuilderx.Continue, result)

		updatedIts := tree.GetRoot().(*workloads.InstanceSet)
		assert.Equal(t, int32(2), updatedIts.Status.Replicas)
		assert.Equal(t, int32(1), updatedIts.Status.ReadyReplicas)
		assert.Equal(t, int32(1), updatedIts.Status.AvailableReplicas)
	})

	t.Run("instance with failure condition", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(1)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
			},
			Status: workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)

		inst0 := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-0",
				Generation:  1,
				Annotations: map[string]string{constant.KubeBlocksGenerationKey: "1"},
			},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 1,
				UpToDate:           true,
				Conditions: []metav1.Condition{
					{Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue},
				},
			},
		}
		require.NoError(t, tree.Add(inst0))

		r := NewStatusReconciler()
		_, err := r.Reconcile(tree)
		require.NoError(t, err)

		updatedIts := tree.GetRoot().(*workloads.InstanceSet)
		// Should have failure condition set
		var hasFailure bool
		for _, cond := range updatedIts.Status.Conditions {
			if cond.Type == string(workloads.InstanceFailure) {
				hasFailure = true
				assert.Equal(t, metav1.ConditionTrue, cond.Status)
			}
		}
		assert.True(t, hasFailure)
	})

	t.Run("minReadySeconds with unavailable instances returns retry", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		replicas := int32(1)
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 1},
			Spec: workloads.InstanceSetSpec{
				Replicas:        &replicas,
				MinReadySeconds: 30,
			},
			Status: workloads.InstanceSetStatus{ObservedGeneration: 1},
		}
		tree.SetRoot(its)

		inst0 := &workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-0",
				Generation:  1,
				Annotations: map[string]string{constant.KubeBlocksGenerationKey: "1"},
			},
			Status: workloads.InstanceStatus2{
				ObservedGeneration: 1,
				UpToDate:           true,
				Ready:              true,
				Conditions: []metav1.Condition{
					{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
					// No available condition — instance is ready but not yet available
				},
			},
		}
		require.NoError(t, tree.Add(inst0))

		r := NewStatusReconciler()
		result, err := r.Reconcile(tree)
		require.NoError(t, err)
		// Should return RetryAfter since available != ready and minReadySeconds > 0
		assert.NotEqual(t, kubebuilderx.Continue, result)
	})
}

// unused import guard
var _ = ptr.To[int32]
