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

package instanceset

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestRevisionUpdateInvalidatesOnlyAffectedLegacyInstances(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*workloads.InstanceSet)
		check  func(*testing.T, *workloads.InstanceSet, map[string]string)
	}{
		{
			name: "pod revision changes for one template",
			mutate: func(its *workloads.InstanceSet) {
				its.Spec.Instances[0].Env = []corev1.EnvVar{{Name: "REVISION_CHANGE", Value: "true"}}
			},
			check: func(t *testing.T, its *workloads.InstanceSet, old map[string]string) {
				updated, err := GetRevisions(its.Status.UpdateRevisions)
				if err != nil {
					t.Fatal(err)
				}
				if updated["demo-0"] == old["demo-0"] || updated["demo-1"] != old["demo-1"] {
					t.Fatalf("revision change was not scoped to demo-0: old=%v new=%v", old, updated)
				}
			},
		},
		{
			name: "revision-excluded resources change for one template",
			mutate: func(its *workloads.InstanceSet) {
				its.Spec.Instances[0].Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				}
			},
			check: func(t *testing.T, its *workloads.InstanceSet, old map[string]string) {
				updated, err := GetRevisions(its.Status.UpdateRevisions)
				if err != nil {
					t.Fatal(err)
				}
				if updated["demo-0"] != old["demo-0"] || updated["demo-1"] != old["demo-1"] {
					t.Fatalf("in-place resources unexpectedly changed revisions: old=%v new=%v", old, updated)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			its, tree, _ := newLegacyStatusContractFixture(t, nil)
			oldRevisions, err := GetRevisions(its.Status.UpdateRevisions)
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
			tt.check(t, its, oldRevisions)
			assertLegacyUpToDate(t, its, "demo-0", false)
			assertLegacyUpToDate(t, its, "demo-1", true)

			// The next status pass consumes the new revisions. The affected instance remains stale and
			// the unaffected observation remains true across the two-reconcile handoff.
			if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			assertLegacyUpToDate(t, its, "demo-0", false)
			assertLegacyUpToDate(t, its, "demo-1", true)
		})
	}
}

func TestLegacyInstanceStatusTracksConfigAndPVCConvergence(t *testing.T) {
	t.Run("dynamic config", func(t *testing.T) {
		configs := []workloads.ConfigTemplate{{Name: "mysql", ConfigHash: ptr.To("old")}}
		its, tree, pods := newLegacyStatusContractFixture(t, configs)
		its.Generation++
		its.Spec.Configs[0].ConfigHash = ptr.To("new")
		if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		assertLegacyUpToDate(t, its, "demo-0", false)
		assertLegacyUpToDate(t, its, "demo-1", false)

		for _, pod := range pods {
			if err := configsToPod(its.Spec.Configs, pod); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		assertLegacyUpToDate(t, its, "demo-0", true)
		assertLegacyUpToDate(t, its, "demo-1", true)
	})

	t.Run("PVC expansion is scoped to one template", func(t *testing.T) {
		claim := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			}},
		}
		its, tree, _ := newLegacyStatusContractFixture(t, nil)
		its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{claim}
		addLegacyPVCs(t, tree, its, resource.MustParse("1Gi"))
		if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}

		its.Generation++
		expanded := claim.DeepCopy()
		expanded.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
		its.Spec.Instances[0].VolumeClaimTemplates = []corev1.PersistentVolumeClaim{*expanded}
		oldRevisions, _ := GetRevisions(its.Status.UpdateRevisions)
		if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		newRevisions, _ := GetRevisions(its.Status.UpdateRevisions)
		if oldRevisions["demo-0"] != newRevisions["demo-0"] {
			t.Fatal("PVC expansion must remain outside the legacy Pod revision hash")
		}
		assertLegacyUpToDate(t, its, "demo-0", false)
		assertLegacyUpToDate(t, its, "demo-1", true)

		pvc := legacyPVCForInstance(tree, "demo-0")
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
		if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		status := its.FindInstanceStatus("demo-0")
		if status.UpToDate || !status.VolumeExpansion {
			t.Fatalf("running expansion was not reflected: %#v", status)
		}

		pvc.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("2Gi")
		if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
			t.Fatal(err)
		}
		status = its.FindInstanceStatus("demo-0")
		if !status.UpToDate || status.VolumeExpansion {
			t.Fatalf("completed expansion did not converge: %#v", status)
		}
	})
}

func TestLegacyAllocationChangesStayInDesiredAndCurrentState(t *testing.T) {
	its, tree, _ := newLegacyDefaultStatusContractFixture(t, 2)

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

func newLegacyStatusContractFixture(t *testing.T, configs []workloads.ConfigTemplate) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*corev1.Pod) {
	t.Helper()
	its := legacyContractInstanceSet(2)
	its.Spec.Configs = configs
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	templates, err := desiredInstanceTemplates(its, tree)
	if err != nil {
		t.Fatal(err)
	}
	pods := make(map[string]*corev1.Pod, len(templates))
	for name, template := range templates {
		pod, err := buildInstancePodByTemplate(name, template, its, "")
		if err != nil {
			t.Fatal(err)
		}
		pod.Status.Phase = corev1.PodRunning
		if err := tree.Add(pod); err != nil {
			t.Fatal(err)
		}
		pods[name] = pod
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	assertLegacyUpToDate(t, its, "demo-0", true)
	assertLegacyUpToDate(t, its, "demo-1", true)
	return its, tree, pods
}

func newLegacyDefaultStatusContractFixture(t *testing.T, replicas int32) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*corev1.Pod) {
	t.Helper()
	its := legacyContractInstanceSet(replicas)
	its.Spec.FlatInstanceOrdinal = false
	its.Spec.Instances = nil
	return newLegacyStatusContractFixtureFromInstanceSet(t, its)
}

func newLegacyStatusContractFixtureFromInstanceSet(t *testing.T, its *workloads.InstanceSet) (*workloads.InstanceSet, *kubebuilderx.ObjectTree, map[string]*corev1.Pod) {
	t.Helper()
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	templates, err := desiredInstanceTemplates(its, tree)
	if err != nil {
		t.Fatal(err)
	}
	pods := make(map[string]*corev1.Pod, len(templates))
	for name, template := range templates {
		pod, err := buildInstancePodByTemplate(name, template, its, "")
		if err != nil {
			t.Fatal(err)
		}
		pod.Status.Phase = corev1.PodRunning
		if err := tree.Add(pod); err != nil {
			t.Fatal(err)
		}
		pods[name] = pod
	}
	if _, err := NewStatusReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}
	return its, tree, pods
}

func legacyContractInstanceSet(replicas int32) *workloads.InstanceSet {
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

func addLegacyPVCs(t *testing.T, tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, capacity resource.Quantity) {
	t.Helper()
	templates, err := desiredInstanceTemplates(its, tree)
	if err != nil {
		t.Fatal(err)
	}
	for name, template := range templates {
		for _, claim := range template.VolumeClaimTemplates {
			pvc := claim.DeepCopy()
			pvc.Name = intctrlutil.ComposePVCName(claim, its.Name, name)
			pvc.Namespace = its.Namespace
			pvc.Labels = map[string]string{constant.KBAppPodNameLabelKey: name}
			pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: capacity.DeepCopy()}
			if err := tree.Add(pvc); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func legacyPVCForInstance(tree *kubebuilderx.ObjectTree, name string) *corev1.PersistentVolumeClaim {
	for _, obj := range tree.List(&corev1.PersistentVolumeClaim{}) {
		pvc := obj.(*corev1.PersistentVolumeClaim)
		if pvc.Labels[constant.KBAppPodNameLabelKey] == name {
			return pvc
		}
	}
	return nil
}

func assertLegacyUpToDate(t *testing.T, its *workloads.InstanceSet, name string, want bool) {
	t.Helper()
	status := its.FindInstanceStatus(name)
	if status == nil || status.UpToDate != want {
		t.Fatalf("instance %s UpToDate = %#v, want %v", name, status, want)
	}
}
