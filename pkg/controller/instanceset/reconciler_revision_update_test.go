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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		})
	})
})

var _ = Describe("instance template images in reconciliation", func() {
	It("creates overridden Pods and replaces them after template image updates", func() {
		podTemplate := corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers:     []corev1.Container{{Name: "db", Image: "db:1"}, {Name: "sidecar", Image: "sidecar:fixed"}},
			InitContainers: []corev1.Container{{Name: "setup", Image: "setup:1"}},
		}}
		its := builder.NewInstanceSetBuilder(namespace, "template-images").SetUID(uid).SetReplicas(2).
			SetTemplate(podTemplate).SetPodManagementPolicy(appsv1.ParallelPodManagement).
			SetPodUpdatePolicy(kbappsv1.ReCreatePodUpdatePolicyType).
			SetPodUpgradePolicy(kbappsv1.ReCreatePodUpdatePolicyType).
			SetInstances([]workloads.InstanceTemplate{{Name: "explicit", Images: map[string]string{"db": "db:2", "setup": "setup:2", "unknown": "ignored"}}}).GetObject()
		its.Generation = 1
		tree := kubebuilderx.NewObjectTree()
		tree.Logger = logger
		tree.SetRoot(its)
		revisions := NewRevisionUpdateReconciler()
		alignment := NewReplicasAlignmentReconciler()
		Expect(revisions.PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))
		_, err := revisions.Reconcile(tree)
		Expect(err).Should(Succeed())
		_, err = alignment.Reconcile(tree)
		Expect(err).Should(Succeed())
		Expect(tree.List(&corev1.Pod{})).Should(HaveLen(2))
		getPod := func(name string) *corev1.Pod {
			obj, err := tree.Get(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
			Expect(err).Should(Succeed())
			return obj.(*corev1.Pod)
		}
		explicitName := its.Name + "-explicit-0"
		defaultName := its.Name + "-0"
		explicit := getPod(explicitName)
		Expect(explicit.Spec.Containers[0].Image).Should(Equal("db:2"))
		Expect(explicit.Spec.Containers[1].Image).Should(Equal("sidecar:fixed"))
		Expect(explicit.Spec.InitContainers[0].Image).Should(Equal("setup:2"))
		Expect(getPod(defaultName).Spec.Containers[0].Image).Should(Equal("db:1"))
		Expect(its.Spec.Template).Should(Equal(podTemplate))
		revision := getPodRevision(explicit)

		for _, images := range []map[string]string{{"db": "db:3", "setup": "setup:3"}, nil} {
			By("observing a template image update through revision and update reconciliation")
			for _, obj := range tree.List(&corev1.Pod{}) {
				pod := obj.(*corev1.Pod)
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			}
			its.Generation++
			its.Spec.Instances[0].Images = images
			Expect(revisions.PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))
			_, err = revisions.Reconcile(tree)
			Expect(err).Should(Succeed())
			Expect(its.Status.ObservedGeneration).Should(Equal(its.Generation))
			desiredRevisions, err := GetRevisions(its.Status.UpdateRevisions)
			Expect(err).Should(Succeed())
			// Image-only updates retain the existing in-place-compatible revision hash.
			Expect(desiredRevisions[explicitName]).Should(Equal(revision))
			_, err = NewUpdateReconciler().Reconcile(tree)
			Expect(err).Should(Succeed())
			Expect(tree.List(&corev1.Pod{})).Should(HaveLen(1))
			Expect(getPod(defaultName).Spec.Containers[0].Image).Should(Equal("db:1"))
			By("resuming from the remaining Pod and creating the latest replacement")
			_, err = alignment.Reconcile(tree)
			Expect(err).Should(Succeed())
			explicit = getPod(explicitName)
			if images == nil {
				Expect(explicit.Spec.Containers[0].Image).Should(Equal("db:1"))
				Expect(explicit.Spec.InitContainers[0].Image).Should(Equal("setup:1"))
			} else {
				Expect(explicit.Spec.Containers[0].Image).Should(Equal("db:3"))
				Expect(explicit.Spec.InitContainers[0].Image).Should(Equal("setup:3"))
			}
			Expect(getPodRevision(explicit)).Should(Equal(revision))
		}
	})
})
