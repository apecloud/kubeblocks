/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("update reconciler test", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetUID(uid).
			SetReplicas(3).
			AddMatchLabelsInMap(selectors).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetMinReadySeconds(minReadySeconds).
			GetObject()
	})

	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			its.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			reconciler = NewUpdateReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))

			By("prepare current tree")
			// desired: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			replicas := int32(7)
			its.Spec.Replicas = &replicas
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			nameHello := "hello"
			instanceHello := workloads.InstanceTemplate{
				Name: nameHello,
			}
			its.Spec.Instances = append(its.Spec.Instances, instanceHello)
			generateNameFoo := "foo"
			replicasFoo := int32(2)
			instanceFoo := workloads.InstanceTemplate{
				Name:     generateNameFoo,
				Replicas: &replicasFoo,
			}
			its.Spec.Instances = append(its.Spec.Instances, instanceFoo)

			// prepare for update
			By("fix meta")
			reconciler = NewFixMetaReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Commit))

			By("update revisions")
			reconciler = NewRevisionUpdateReconciler()
			res, err = reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))

			By("assistant object")
			reconciler = NewAssistantObjectReconciler()
			res, err = reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))

			By("replicas alignment")
			reconciler = NewReplicasAlignmentReconciler()
			res, err = reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))

			By("update all pods to ready with outdated revision")
			pods := tree.List(&corev1.Pod{})
			containersReadyCondition := corev1.PodCondition{
				Type:               corev1.ContainersReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			}
			readyCondition := corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			}
			makePodAvailableWithOldRevision := func(pod *corev1.Pod) {
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "old-revision"
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = append(pod.Status.Conditions, readyCondition, containersReadyCondition)
			}
			for _, object := range pods {
				pod, ok := object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				makePodAvailableWithOldRevision(pod)
			}

			expectUpdatedPods := func(tree *kubebuilderx.ObjectTree, names []string) {
				pods = tree.List(&corev1.Pod{})
				Expect(pods).Should(HaveLen(int(replicas) - len(names)))
				for _, name := range names {
					Expect(slices.IndexFunc(pods, func(object client.Object) bool {
						return object.GetName() == name
					})).Should(BeNumerically("<", 0))
				}
			}
			makePodLatestRevision := func(pod *corev1.Pod) {
				labels := pod.Labels
				if labels == nil {
					labels = make(map[string]string)
				}
				updateRevisions, err := GetRevisions(its.Status.UpdateRevisions)
				Expect(err).Should(BeNil())
				labels[appsv1.ControllerRevisionHashLabelKey] = updateRevisions[pod.Name]
			}
			reconciler = NewUpdateReconciler()

			By("reconcile with default UpdateStrategy(RollingUpdate, no partition, MaxUnavailable=1)")
			// order: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			// expected: bar-hello-0 being deleted
			defaultTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			res, err = reconciler.Reconcile(defaultTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(defaultTree, []string{"bar-hello-0"})

			By("reconcile with Partition=50% and MaxUnavailable=2")
			partitionTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok := partitionTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			updateReplicas := intstr.FromInt32(3)
			maxUnavailable := intstr.FromInt32(2)
			root.Spec.UpdateStrategy = &workloads.UpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       &updateReplicas,
					MaxUnavailable: &maxUnavailable,
				},
			}
			// order: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			// expected: bar-hello-0, bar-foo-1 being deleted
			res, err = reconciler.Reconcile(partitionTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(partitionTree, []string{"bar-hello-0", "bar-foo-1"})

			By("update revisions to the updated value")
			partitionTree, err = tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = partitionTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			root.Spec.UpdateStrategy = &workloads.UpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       &updateReplicas,
					MaxUnavailable: &maxUnavailable,
				},
			}
			for _, name := range []string{"bar-hello-0", "bar-foo-1"} {
				pod := builder.NewPodBuilder(namespace, name).GetObject()
				object, err := partitionTree.Get(pod)
				Expect(err).Should(BeNil())
				pod, ok = object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				makePodLatestRevision(pod)
			}
			res, err = reconciler.Reconcile(partitionTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(partitionTree, []string{"bar-foo-0"})

			By("reconcile with UpdateStrategy='OnDelete'")
			onDeleteTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = onDeleteTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			root.Spec.UpdateStrategy = &workloads.UpdateStrategy{
				Type: workloads.OnDeleteStrategyType,
			}
			res, err = reconciler.Reconcile(onDeleteTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(onDeleteTree, []string{})

			// order: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			// expected: bar-hello-0 being deleted
			By("reconcile with PodUpdatePolicy='PreferInPlace'")
			preferInPlaceTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = preferInPlaceTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			instanceUpdatePolicy := workloads.PreferInPlaceInstanceUpdatePolicyType
			root.Spec.UpdateStrategy = &workloads.UpdateStrategy{
				InstanceUpdatePolicy: &instanceUpdatePolicy,
			}
			// try to add env to instanceHello to trigger the recreation
			root.Spec.Instances[0].Env = []corev1.EnvVar{
				{
					Name:  "foo",
					Value: "bar",
				},
			}
			res, err = reconciler.Reconcile(preferInPlaceTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(preferInPlaceTree, []string{"bar-hello-0"})

			By("reconcile with PodUpdatePolicy='StrictInPlace'")
			strictInPlaceTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = strictInPlaceTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			instanceUpdatePolicy = workloads.StrictInPlaceInstanceUpdatePolicyType
			root.Spec.UpdateStrategy = &workloads.UpdateStrategy{
				InstanceUpdatePolicy: &instanceUpdatePolicy,
			}
			// try to add env to instanceHello to trigger the recreation
			root.Spec.Instances[0].Env = []corev1.EnvVar{
				{
					Name:  "foo",
					Value: "bar",
				},
			}
			res, err = reconciler.Reconcile(strictInPlaceTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(strictInPlaceTree, []string{})
		})
	})
})
