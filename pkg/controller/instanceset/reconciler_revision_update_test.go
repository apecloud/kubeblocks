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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
