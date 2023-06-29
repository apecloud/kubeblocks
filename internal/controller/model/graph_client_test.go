/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package model

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

var _ = Describe("graph client test.", func() {
	Context("GraphWriter", func() {
		It("should work well", func() {
			graphCli := NewGraphClient(nil)
			dag := graph.NewDAG()

			By("create without root vertex")
			namespace := "foo"
			name := "bar"
			root := builder.NewStatefulSetBuilder(namespace, name).GetObject()
			graphCli.Create(dag, root)
			dagExpected := graph.NewDAG()
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("init root vertex")
			graphCli.Root(dag, root, root.DeepCopy())
			dagExpected.AddVertex(&ObjectVertex{Obj: root, OriObj: root, Action: ActionPtr(STATUS)})
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("create object")
			obj0 := builder.NewPodBuilder(namespace, name+"0").GetObject()
			obj1 := builder.NewPodBuilder(namespace, name+"1").GetObject()
			obj2 := builder.NewPodBuilder(namespace, name+"2").GetObject()
			graphCli.Create(dag, obj0)
			graphCli.Create(dag, obj1)
			graphCli.Create(dag, obj2)
			graphCli.DependOn(dag, obj1, obj2)
			v0 := &ObjectVertex{Obj: obj0, Action: ActionPtr(CREATE)}
			v1 := &ObjectVertex{Obj: obj1, Action: ActionPtr(CREATE)}
			v2 := &ObjectVertex{Obj: obj2, Action: ActionPtr(CREATE)}
			dagExpected.AddConnectRoot(v0)
			dagExpected.AddConnectRoot(v1)
			dagExpected.AddConnectRoot(v2)
			dagExpected.Connect(v1, v2)
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("update&delete&status object")
			graphCli.Status(dag, obj0, obj0.DeepCopy())
			graphCli.Update(dag, obj1, obj1.DeepCopy())
			graphCli.Delete(dag, obj2)
			v0.Action = ActionPtr(STATUS)
			v1.Action = ActionPtr(UPDATE)
			v2.Action = ActionPtr(DELETE)
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())
		})
	})
})
