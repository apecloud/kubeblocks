/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instanceset2

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
)

func TestRevisionUpdateInvalidatesOnlyAffectedITS2Instances(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*workloads.InstanceSet)
	}{
		{
			name: "pod revision changes for one template",
			mutate: func(its *workloads.InstanceSet) {
				its.Spec.Instances[0].Env = []corev1.EnvVar{{Name: "REVISION_CHANGE", Value: "true"}}
			},
		},
		{
			name: "in-place resources change for one template",
			mutate: func(its *workloads.InstanceSet) {
				its.Spec.Instances[0].Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			its, tree, _ := newITS2InstanceStatusFixture(t, nil)
			oldRevisions, err := revisionmap.Decode(its.Status.UpdateRevisions)
			if err != nil {
				t.Fatal(err)
			}
			its.Generation++
			tt.mutate(its)

			if NewStatusReconciler().PreCondition(tree) != kubebuilderx.ConditionUnsatisfied {
				t.Fatal("status reconciler must remain before revision update and wait for the new generation target")
			}
			if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			newRevisions, err := revisionmap.Decode(its.Status.UpdateRevisions)
			if err != nil {
				t.Fatal(err)
			}
			if newRevisions["demo-0"] == oldRevisions["demo-0"] || newRevisions["demo-1"] != oldRevisions["demo-1"] {
				t.Fatalf("Instance spec revision change was not scoped to demo-0: old=%v new=%v", oldRevisions, newRevisions)
			}
			assertITS2UpToDate(t, its, "demo-0", false)
			assertITS2UpToDate(t, its, "demo-1", true)

			// The second pass uses the new Instance-spec revisions and keeps only the affected
			// current Instance stale until its own controller applies the desired spec.
			if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			assertITS2UpToDate(t, its, "demo-0", false)
			assertITS2UpToDate(t, its, "demo-1", true)
		})
	}
}

func TestITS2RevisionUpdateTracksConfigAndPVCChanges(t *testing.T) {
	t.Run("dynamic config", func(t *testing.T) {
		configs := []workloads.ConfigTemplate{{Name: "mysql", ConfigHash: ptr.To("old")}}
		its, tree, _ := newITS2InstanceStatusFixture(t, configs)
		its.Generation++
		its.Spec.Configs[0].ConfigHash = ptr.To("new")
		if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		assertITS2UpToDate(t, its, "demo-0", false)
		assertITS2UpToDate(t, its, "demo-1", false)
	})

	t.Run("PVC expansion is scoped to one template", func(t *testing.T) {
		claim := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			}},
		}
		its := its2InstanceStatusSet(2)
		its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{claim}
		its, tree, _ := newITS2InstanceStatusFixtureFromSet(t, its)

		its.Generation++
		expanded := claim.DeepCopy()
		expanded.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
		its.Spec.Instances[0].VolumeClaimTemplates = []corev1.PersistentVolumeClaim{*expanded}
		if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		assertITS2UpToDate(t, its, "demo-0", false)
		assertITS2UpToDate(t, its, "demo-1", true)
	})
}

func TestITS2AllocationChangesStayInDesiredAndCurrentState(t *testing.T) {
	its := its2InstanceStatusSet(2)
	its.Spec.FlatInstanceOrdinal = false
	its.Spec.Instances = nil
	its, tree, _ := newITS2InstanceStatusFixtureFromSet(t, its)

	its.Generation++
	its.Spec.Replicas = ptr.To[int32](3)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	added := its.FindInstanceStatus("demo-2")
	if added == nil || added.DesiredState != workloads.InstanceDesiredStateActive || added.CurrentState != workloads.InstanceCurrentStateAbsent || added.UpToDate {
		t.Fatalf("added identity was not expressed by desired/current state: %#v", added)
	}

	its.Generation++
	its.Spec.Replicas = ptr.To[int32](1)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	released := its.FindInstanceStatus("demo-1")
	if released == nil || released.DesiredState != workloads.InstanceDesiredStateReleased || released.CurrentState != workloads.InstanceCurrentStatePresent || released.UpToDate {
		t.Fatalf("removed identity was not expressed as Released/Present: %#v", released)
	}

	its.Generation++
	its.Spec.OfflineInstances = []string{"demo-0"}
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	offline := its.FindInstanceStatus("demo-0")
	if offline == nil || offline.DesiredState != workloads.InstanceDesiredStateOffline || offline.CurrentState != workloads.InstanceCurrentStatePresent || offline.UpToDate {
		t.Fatalf("offline identity was not expressed as Offline/Present: %#v", offline)
	}
}

func newITS2InstanceStatusFixture(t *testing.T, configs []workloads.ConfigTemplate) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*workloads.Instance) {
	t.Helper()
	its := its2InstanceStatusSet(2)
	its.Spec.Configs = configs
	return newITS2InstanceStatusFixtureFromSet(t, its)
}

func newITS2InstanceStatusFixtureFromSet(t *testing.T, its *workloads.InstanceSet) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*workloads.Instance) {
	t.Helper()
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	desired, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		t.Fatal(err)
	}
	instances := make(map[string]*workloads.Instance, len(desired))
	for name, target := range desired {
		inst := target.DeepCopy()
		inst.Generation = 1
		inst.Status = workloads.InstanceStatus2{
			ObservedGeneration: 1,
			CurrentState:       workloads.InstanceCurrentStatePresent,
			CurrentRevision:    "pod-revision",
			UpdateRevision:     "pod-revision",
			UpToDate:           true,
		}
		for _, config := range inst.Spec.Configs {
			inst.Status.Configs = append(inst.Status.Configs, workloads.InstanceConfigStatus{Name: config.Name, ConfigHash: config.ConfigHash})
		}
		if err := tree.Add(inst); err != nil {
			t.Fatal(err)
		}
		instances[name] = inst
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	assertITS2UpToDate(t, its, "demo-0", true)
	assertITS2UpToDate(t, its, "demo-1", true)
	return its, tree, instances
}

func its2InstanceStatusSet(replicas int32) *workloads.InstanceSet {
	one := int32(1)
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1},
		Spec: workloads.InstanceSetSpec{
			Replicas:            ptr.To(replicas),
			FlatInstanceOrdinal: true,
			Selector:            &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "db", Image: "mysql:old",
			}}}},
			Instances: []workloads.InstanceTemplate{
				{Name: "a", Replicas: &one, Ordinals: workloads.Ordinals{Discrete: []int32{0}}},
				{Name: "b", Replicas: &one, Ordinals: workloads.Ordinals{Discrete: []int32{1}}},
			},
		},
	}
}

func assertITS2UpToDate(t *testing.T, its *workloads.InstanceSet, name string, want bool) {
	t.Helper()
	status := its.FindInstanceStatus(name)
	if status == nil || status.UpToDate != want {
		t.Fatalf("instance %s UpToDate = %#v, want %v", name, status, want)
	}
}
