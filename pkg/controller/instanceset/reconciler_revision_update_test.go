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

package instanceset

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("revision update reconciler test", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetReplicas(3).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetRoles(roles).
			GetObject()
	})

	Context("PreCondition & Reconcile", func() {
		It("keeps instance status behind revision publication", func() {
			its.Generation = 2
			its.Status.ObservedGeneration = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			Expect(NewStatusReconciler().PreCondition(tree)).Should(Equal(kubebuilderx.ConditionUnsatisfied))
			Expect(NewRevisionUpdateReconciler().PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))

			its.Status.ObservedGeneration = its.Generation
			Expect(NewStatusReconciler().PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))
		})

		It("should work well", func() {
			By("PreCondition")
			its.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			reconciler := NewRevisionUpdateReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))

			By("Reconcile")
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			newITS, ok := tree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			Expect(newITS.Status.ObservedGeneration).Should(Equal(its.Generation))
			updateRevisions, err := GetRevisions(newITS.Status.UpdateRevisions)
			Expect(err).Should(BeNil())
			Expect(updateRevisions).Should(HaveLen(3))
			Expect(updateRevisions).Should(HaveKey(its.Name + "-0"))
			Expect(updateRevisions).Should(HaveKey(its.Name + "-1"))
			Expect(updateRevisions).Should(HaveKey(its.Name + "-2"))
			Expect(newITS.Status.UpdateRevision).Should(Equal(updateRevisions[its.Name+"-2"]))
			Expect(newITS.Status.InstanceStatus).Should(BeEmpty())
		})

		It("preserves the published view and allows alignment during a transient flat ordinal reassignment", func() {
			its = transientFlatReassignmentInstanceSet()
			previous := its.DeepCopy().Status.InstanceStatus
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			for ordinal := 0; ordinal < 2; ordinal++ {
				pod := builder.NewPodBuilder(its.Namespace, its.Name+fmt.Sprintf("-%d", ordinal)).GetObject()
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
				Expect(tree.Add(pod)).Should(Succeed())
			}

			res, err := NewStatusReconciler().Reconcile(tree)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			Expect(its.Status.InstanceStatus).Should(Equal(previous))

			res, err = NewRevisionUpdateReconciler().Reconcile(tree)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			Expect(its.Status.ObservedGeneration).Should(Equal(its.Generation))
			Expect(its.Status.InstanceStatus).Should(Equal(previous))

			res, err = NewReplicasAlignmentReconciler().Reconcile(tree)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			Expect(tree.List(&corev1.Pod{})).Should(HaveLen(1))
		})
	})
})

func transientFlatReassignmentInstanceSet() *workloads.InstanceSet {
	replicas, templateReplicas := int32(2), int32(1)
	templateA, templateB := "a", "b"
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 2},
		Spec: workloads.InstanceSetSpec{
			Replicas:            &replicas,
			FlatInstanceOrdinal: true,
			Template:            corev1.PodTemplateSpec{},
			Instances: []workloads.InstanceTemplate{
				{Name: templateA, Replicas: &templateReplicas, Ordinals: workloads.Ordinals{Discrete: []int32{1}}},
				{Name: templateB, Replicas: &templateReplicas, Ordinals: workloads.Ordinals{Discrete: []int32{0}}},
			},
		},
		Status: workloads.InstanceSetStatus{
			ObservedGeneration: 1,
			AssignedOrdinals: map[string]workloads.Ordinals{
				templateA: {Discrete: []int32{0}},
				templateB: {Discrete: []int32{1}},
			},
			InstanceStatus: []workloads.InstanceStatus{
				{PodName: "demo-0", TemplateName: &templateA, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent},
				{PodName: "demo-1", TemplateName: &templateB, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent},
			},
		},
	}
}

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
			its, tree, _ := newLegacyInstanceStatusFixture(t, nil)
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

func TestRevisionUpdateInvalidatesMissingLegacyPod(t *testing.T) {
	its, tree, pods := newLegacyInstanceStatusFixture(t, nil)
	missing := its.FindInstanceStatus("demo-0")
	missing.Ready = true
	missing.Available = true
	if err := tree.Delete(pods[missing.PodName]); err != nil {
		t.Fatal(err)
	}

	its.Generation++
	its.Spec.Instances[0].Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
	}
	if NewStatusReconciler().PreCondition(tree) != kubebuilderx.ConditionUnsatisfied {
		t.Fatal("status reconciler must wait for revision publication")
	}
	if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
		t.Fatal(err)
	}

	if its.Status.ObservedGeneration != its.Generation {
		t.Fatalf("ObservedGeneration = %d, want %d", its.Status.ObservedGeneration, its.Generation)
	}
	assertLegacyUpToDate(t, its, "demo-0", false)
	assertLegacyUpToDate(t, its, "demo-1", true)
	if !missing.Ready || !missing.Available {
		t.Fatalf("revision updater changed runtime observations: %#v", missing)
	}
}
