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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	experimentalv1alpha1 "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("scale target cluster reconciler test", func() {
	BeforeEach(func() {
		tree = mockTestTree()
	})

	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			reconciler := scaleTargetCluster()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultSatisfied))

			By("Reconcile")
			beforeReconcile := metav1.Now()
			newTree, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			newNAS, ok := newTree.GetRoot().(*experimentalv1alpha1.NodeAwareScaler)
			Expect(ok).Should(BeTrue())
			Expect(newNAS.Status.LastScaleTime.Compare(beforeReconcile.Time)).Should(BeNumerically(">=", 0))
			object, err := newTree.Get(builder.NewClusterBuilder(newNAS.Namespace, newNAS.Spec.TargetClusterName).GetObject())
			Expect(err).Should(BeNil())
			newCluster, ok := object.(*appsv1alpha1.Cluster)
			Expect(ok).Should(BeTrue())
			nodes := newTree.List(&corev1.Node{})
			desiredReplicas := int32(len(nodes))
			Expect(newCluster.Spec.ComponentSpecs).Should(HaveLen(2))
			Expect(newCluster.Spec.ComponentSpecs[0].Replicas).Should(Equal(desiredReplicas))
			Expect(newCluster.Spec.ComponentSpecs[1].Replicas).Should(Equal(desiredReplicas))
		})
	})
})
