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

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("fix meta reconciler test", func() {
	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			its := builder.NewInstanceSetBuilder(namespace, name).GetObject()
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			reconciler := NewFixMetaReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultSatisfied))

			By("Reconcile without finalizer")
			newTree, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(newTree.GetRoot().GetFinalizers()).Should(HaveLen(1))
			Expect(newTree.GetRoot().GetFinalizers()[0]).Should(Equal(finalizer))

			By("Reconcile with finalizer")
			Expect(reconciler.PreCondition(newTree)).Should(Equal(kubebuilderx.ResultUnsatisfied))
		})
	})
})
