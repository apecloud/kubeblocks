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

package rsm2

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

var _ = Describe("status reconciler test", func() {
	const (
		namespace = "foo"
		name      = "bar"

		minReadySeconds = 10
	)

	var (
		rsm        *workloads.ReplicatedStateMachine
		reconciler kubebuilderx.Reconciler

		uid = types.UID("rsm-mock-uid")

		selectors = map[string]string{
			constant.AppInstanceLabelKey:    name,
			rsm1.WorkloadsManagedByLabelKey: rsm1.KindReplicatedStateMachine,
		}

		roles = []workloads.ReplicaRole{
			{
				Name:       "leader",
				IsLeader:   true,
				CanVote:    true,
				AccessMode: workloads.ReadWriteMode,
			},
			{
				Name:       "follower",
				IsLeader:   false,
				CanVote:    true,
				AccessMode: workloads.ReadonlyMode,
			},
			{
				Name:       "logger",
				IsLeader:   false,
				CanVote:    true,
				AccessMode: workloads.NoneMode,
			},
			{
				Name:       "learner",
				IsLeader:   false,
				CanVote:    false,
				AccessMode: workloads.ReadonlyMode,
			},
		}
		pod = builder.NewPodBuilder("", "").
			AddContainer(corev1.Container{
				Name:  "foo",
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          "my-svc",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: 12345,
					},
				},
			}).GetObject()
		template = corev1.PodTemplateSpec{
			ObjectMeta: pod.ObjectMeta,
			Spec:       pod.Spec,
		}

		volumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceStorage: resource.MustParse("2G"),
						},
					},
				},
			},
		}
	)

	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
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
			rsm.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(rsm)

			By("prepare current tree")
			// desired: bar-0, bar-1, bar-2, bar-3, foo-0, foo-1, hello
			replicas := int32(7)
			rsm.Spec.Replicas = &replicas
			rsm.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			nameHello := "hello"
			instanceHello := workloads.InstanceTemplate{
				Name: &nameHello,
			}
			rsm.Spec.Instances = append(rsm.Spec.Instances, instanceHello)
			generateNameFoo := "foo"
			replicasFoo := int32(2)
			instanceFoo := workloads.InstanceTemplate{
				GenerateName: &generateNameFoo,
				Replicas:     &replicasFoo,
			}
			rsm.Spec.Instances = append(rsm.Spec.Instances, instanceFoo)

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

			By("all pods are not ready")
			reconciler = NewStatusReconciler()
			Expect(reconciler.PreCondition(newTree)).Should(Equal(kubebuilderx.ResultSatisfied))
			_, err = reconciler.Reconcile(newTree)
			Expect(err).Should(BeNil())
			Expect(rsm.Status.Replicas).Should(BeEquivalentTo(0))
			Expect(rsm.Status.ReadyReplicas).Should(BeEquivalentTo(0))
			Expect(rsm.Status.AvailableReplicas).Should(BeEquivalentTo(0))
			Expect(rsm.Status.UpdatedReplicas).Should(BeEquivalentTo(0))
			Expect(rsm.Status.CurrentReplicas).Should(BeEquivalentTo(0))
			Expect(rsm.Status.CurrentRevisions).Should(HaveLen(0))
			Expect(rsm.Status.CurrentGeneration).Should(BeEquivalentTo(rsm.Generation))

			By("make all pods ready with old revision")
			condition := corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			}
			makePodAvailableWithOldRevision := func(pod *corev1.Pod, revision string) {
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = revision
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = append(pod.Status.Conditions, condition)
			}
			pods := newTree.List(&corev1.Pod{})
			for _, object := range pods {
				pod, ok := object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				makePodAvailableWithOldRevision(pod, "old-revision")
			}
			_, err = reconciler.Reconcile(newTree)
			Expect(rsm.Status.Replicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.ReadyReplicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.AvailableReplicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.UpdatedReplicas).Should(BeEquivalentTo(0))
			Expect(rsm.Status.CurrentReplicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.CurrentRevisions).Should(HaveLen(0))
			Expect(rsm.Status.CurrentGeneration).Should(BeEquivalentTo(rsm.Generation))

			By("make all pods available with latest revision")
			for _, object := range pods {
				pod, ok := object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				makePodAvailableWithOldRevision(pod, rsm.Status.UpdateRevisions[pod.Name])
			}
			_, err = reconciler.Reconcile(newTree)
			Expect(rsm.Status.Replicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.ReadyReplicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.AvailableReplicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.UpdatedReplicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.CurrentReplicas).Should(BeEquivalentTo(replicas))
			Expect(rsm.Status.CurrentRevisions).Should(Equal(rsm.Status.UpdateRevisions))
			Expect(rsm.Status.CurrentGeneration).Should(BeEquivalentTo(rsm.Generation))
		})
	})
})
