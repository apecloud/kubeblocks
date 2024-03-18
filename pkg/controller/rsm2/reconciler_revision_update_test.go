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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("revision update reconciler test", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetService(&corev1.Service{}).
			SetReplicas(3).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetRoles(roles).
			GetObject()
	})

	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			rsm.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(rsm)
			reconciler := NewRevisionUpdateReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultSatisfied))

			By("Reconcile")
			newTree, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			newRsm, ok := newTree.GetRoot().(*workloads.ReplicatedStateMachine)
			Expect(ok).Should(BeTrue())
			Expect(newRsm.Status.ObservedGeneration).Should(Equal(rsm.Generation))
			Expect(newRsm.Status.UpdateRevisions).Should(HaveLen(3))
			Expect(newRsm.Status.UpdateRevisions).Should(HaveKey(rsm.Name + "-0"))
			Expect(newRsm.Status.UpdateRevisions).Should(HaveKey(rsm.Name + "-1"))
			Expect(newRsm.Status.UpdateRevisions).Should(HaveKey(rsm.Name + "-2"))
			Expect(newRsm.Status.UpdateRevision).Should(Equal(newRsm.Status.UpdateRevisions[rsm.Name+"-2"]))
		})
	})
})
