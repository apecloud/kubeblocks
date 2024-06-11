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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
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
			SetRoles(roles).
			GetObject()
	})

	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			its.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			reconciler = NewUpdateReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultSatisfied))

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
			newTree, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())

			By("update revisions")
			reconciler = NewRevisionUpdateReconciler()
			newTree, err = reconciler.Reconcile(newTree)
			Expect(err).Should(BeNil())

			By("assistant object")
			reconciler = NewAssistantObjectReconciler()
			newTree, err = reconciler.Reconcile(newTree)
			Expect(err).Should(BeNil())

			By("replicas alignment")
			reconciler = NewReplicasAlignmentReconciler()
			newTree, err = reconciler.Reconcile(newTree)
			Expect(err).Should(BeNil())

			By("update all pods to ready with outdated revision")
			pods := newTree.List(&corev1.Pod{})
			condition := corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			}
			makePodAvailableWithOldRevision := func(pod *corev1.Pod) {
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "old-revision"
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = append(pod.Status.Conditions, condition)
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
			defaultTree, err := newTree.DeepCopy()
			Expect(err).Should(BeNil())
			_, err = reconciler.Reconcile(defaultTree)
			Expect(err).Should(BeNil())
			expectUpdatedPods(defaultTree, []string{"bar-hello-0"})

			By("reconcile with Partition=50% and MaxUnavailable=2")
			partitionTree, err := newTree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok := partitionTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			partition := int32(3)
			maxUnavailable := intstr.FromInt32(2)
			root.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition:      &partition,
					MaxUnavailable: &maxUnavailable,
				},
			}
			// order: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			// expected: bar-hello-0, bar-foo-1 being deleted
			_, err = reconciler.Reconcile(partitionTree)
			Expect(err).Should(BeNil())
			expectUpdatedPods(partitionTree, []string{"bar-hello-0", "bar-foo-1"})

			By("update revisions to the updated value")
			partitionTree, err = newTree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = partitionTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			root.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition:      &partition,
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
			_, err = reconciler.Reconcile(partitionTree)
			Expect(err).Should(BeNil())
			expectUpdatedPods(partitionTree, []string{"bar-foo-0"})

			By("reconcile with UpdateStrategy='OnDelete'")
			onDeleteTree, err := newTree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = onDeleteTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			root.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
			_, err = reconciler.Reconcile(onDeleteTree)
			Expect(err).Should(BeNil())
			expectUpdatedPods(onDeleteTree, []string{})
		})
	})
})
