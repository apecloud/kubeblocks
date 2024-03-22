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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("deletion reconciler test", func() {
	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			rsm := builder.NewReplicatedStateMachineBuilder(namespace, name).GetObject()
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(rsm)
			reconciler := NewDeletionReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultUnsatisfied))
			t := metav1.NewTime(time.Now())
			rsm.SetDeletionTimestamp(&t)
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultSatisfied))

			By("Reconcile")
			pod := builder.NewPodBuilder(namespace, name+"0").GetObject()
			Expect(tree.Add(pod)).Should(Succeed())
			newTree, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(newTree.GetRoot()).Should(Equal(rsm))
			newTree, err = reconciler.Reconcile(newTree)
			Expect(err).Should(BeNil())
			Expect(newTree.GetRoot()).Should(BeNil())
		})
	})
})
