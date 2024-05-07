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

package experimental

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	experimentalv1alpha1 "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("update status reconciler test", func() {
	BeforeEach(func() {
		tree = mockTestTree()
	})

	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			var reconciler kubebuilderx.Reconciler

			By("scale target cluster")
			reconciler = scaleTargetCluster()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultSatisfied))
			newTree, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())

			By("mock the workload to scale ready")
			nodes := tree.List(&corev1.Node{})
			desiredReplicas := int32(len(nodes))
			itsList := tree.List(&workloads.InstanceSet{})
			for _, object := range itsList {
				its, ok := object.(*workloads.InstanceSet)
				Expect(ok).Should(BeTrue())
				its.Status.CurrentReplicas = desiredReplicas
				its.Status.ReadyReplicas = desiredReplicas
				its.Status.AvailableReplicas = desiredReplicas
			}

			By("update status")
			reconciler = updateStatus()
			Expect(reconciler.PreCondition(newTree)).Should(Equal(kubebuilderx.ResultSatisfied))
			newTree, err = reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			newNAS, ok := newTree.GetRoot().(*experimentalv1alpha1.NodeAwareScaler)
			Expect(ok).Should(BeTrue())
			Expect(newNAS.Status.ComponentStatuses).Should(HaveLen(2))
			Expect(newNAS.Status.ComponentStatuses[0].CurrentReplicas).Should(Equal(desiredReplicas))
			Expect(newNAS.Status.ComponentStatuses[0].ReadyReplicas).Should(Equal(desiredReplicas))
			Expect(newNAS.Status.ComponentStatuses[0].AvailableReplicas).Should(Equal(desiredReplicas))
			Expect(newNAS.Status.ComponentStatuses[0].DesiredReplicas).Should(Equal(desiredReplicas))
			Expect(newNAS.Status.ComponentStatuses[1].CurrentReplicas).Should(Equal(desiredReplicas))
			Expect(newNAS.Status.ComponentStatuses[1].ReadyReplicas).Should(Equal(desiredReplicas))
			Expect(newNAS.Status.ComponentStatuses[1].AvailableReplicas).Should(Equal(desiredReplicas))
			Expect(newNAS.Status.ComponentStatuses[1].DesiredReplicas).Should(Equal(desiredReplicas))
			Expect(newNAS.Status.Conditions).Should(HaveLen(1))
			Expect(newNAS.Status.Conditions[0].Type).Should(BeEquivalentTo(experimentalv1alpha1.ScaleReady))
			Expect(newNAS.Status.Conditions[0].Status).Should(Equal(metav1.ConditionTrue))
			Expect(newNAS.Status.Conditions[0].Reason).Should(Equal(experimentalv1alpha1.ReasonReady))
			Expect(newNAS.Status.Conditions[0].Message).Should(Equal("scale ready"))
		})
	})
})
