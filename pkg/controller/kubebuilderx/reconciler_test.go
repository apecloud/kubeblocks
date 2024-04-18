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

package kubebuilderx

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("reconciler test", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	Context("ObjectTree methods", func() {
		It("should work well", func() {
			By("NewObjectTree")
			expectedTree := &ObjectTree{children: make(model.ObjectSnapshot)}
			tree := NewObjectTree()
			Expect(tree).Should(Equal(expectedTree))

			By("SetRoot & GetRoot")
			root := builder.NewReplicatedStateMachineBuilder(namespace, name).GetObject()
			tree.SetRoot(root)
			expectedTree.root = root
			Expect(tree).Should(Equal(expectedTree))
			Expect(tree.GetRoot()).Should(Equal(root))

			By("DeleteRoot")
			tree.DeleteRoot()
			Expect(tree.GetRoot()).Should(BeNil())

			By("CRUD secondary objects")
			obj0 := builder.NewPodBuilder(namespace, name+"0").GetObject()
			obj1 := builder.NewPodBuilder(namespace, name+"1").GetObject()
			tree.SetRoot(root)
			Expect(tree.Add(obj0)).Should(Succeed())
			Expect(tree.Add(obj1)).Should(Succeed())
			Expect(tree.List(&corev1.Pod{})).Should(HaveLen(2))
			Expect(tree.Delete(obj1)).Should(Succeed())
			Expect(tree.List(&corev1.Pod{})).Should(HaveLen(1))
			Expect(tree.List(&corev1.Pod{})[0]).Should(Equal(obj0))
			obj0Update := builder.NewPodBuilder(namespace, name+"0").AddLabels("hello", "world").GetObject()
			Expect(tree.Update(obj0Update)).Should(Succeed())
			Expect(tree.List(&corev1.Pod{})[0]).Should(Equal(obj0Update))
			Expect(tree.GetSecondaryObjects()).Should(HaveLen(1))
			tree.DeleteSecondaryObjects()
			Expect(tree.GetSecondaryObjects()).Should(HaveLen(0))

			By("DeepCopy")
			tree.SetRoot(root)
			Expect(tree.Add(obj0, obj1)).Should(Succeed())
			treeCopied, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			Expect(treeCopied).Should(Equal(tree))

			By("Set&Get Finalizer")
			finalizer := "test"
			tree.SetFinalizer(finalizer)
			Expect(tree.GetFinalizer()).Should(Equal(finalizer))
		})
	})
})
