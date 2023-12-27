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
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

var _ = Describe("graph client test.", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	Context("GraphWriter", func() {
		It("should work well", func() {
			graphCli := NewGraphClient(nil)
			dag := graph.NewDAG()
			dagExpected := graph.NewDAG()
			root := builder.NewStatefulSetBuilder(namespace, name).GetObject()

			By("init root vertex")
			graphCli.Root(dag, root.DeepCopy(), root, ActionStatusPtr())
			dagExpected.AddVertex(&ObjectVertex{Obj: root, OriObj: root, Action: ActionStatusPtr()})
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("create object")
			obj0 := builder.NewPodBuilder(namespace, name+"0").GetObject()
			obj1 := builder.NewPodBuilder(namespace, name+"1").GetObject()
			obj2 := builder.NewPodBuilder(namespace, name+"2").GetObject()
			graphCli.Create(dag, obj0)
			graphCli.Create(dag, obj1)
			graphCli.Create(dag, obj2)
			graphCli.DependOn(dag, obj1, obj2)
			v0 := &ObjectVertex{Obj: obj0, Action: ActionCreatePtr()}
			v1 := &ObjectVertex{Obj: obj1, Action: ActionCreatePtr()}
			v2 := &ObjectVertex{Obj: obj2, Action: ActionCreatePtr()}
			dagExpected.AddConnectRoot(v0)
			dagExpected.AddConnectRoot(v1)
			dagExpected.AddConnectRoot(v2)
			dagExpected.Connect(v1, v2)
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("update&delete&status object")
			graphCli.Status(dag, obj0, obj0.DeepCopy())
			graphCli.Update(dag, obj1, obj1.DeepCopy())
			graphCli.Delete(dag, obj2)
			v0.Action = ActionStatusPtr()
			v1.Action = ActionUpdatePtr()
			v2.Action = ActionDeletePtr()
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("replace an existing object")
			newObj1 := builder.NewPodBuilder(namespace, name+"1").GetObject()
			graphCli.Update(dag, nil, newObj1, &ReplaceIfExistingOption{})
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())
			podList := graphCli.FindAll(dag, &corev1.Pod{})
			Expect(podList).Should(HaveLen(3))
			Expect(slices.IndexFunc(podList, func(obj client.Object) bool {
				return obj == newObj1
			})).Should(BeNumerically(">=", 0))

			By("noop")
			graphCli.Noop(dag, obj0)
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeFalse())
			v0.Action = ActionNoopPtr()
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("patch")
			graphCli.Patch(dag, obj0.DeepCopy(), obj0)
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeFalse())
			v0.Action = ActionPatchPtr()
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("find objects exist")
			podList = graphCli.FindAll(dag, &corev1.Pod{})
			Expect(podList).Should(HaveLen(3))
			for _, object := range []client.Object{obj0, newObj1, obj2} {
				Expect(slices.IndexFunc(podList, func(obj client.Object) bool {
					return obj == object
				})).Should(BeNumerically(">=", 0))
			}
			Expect(slices.IndexFunc(podList, func(obj client.Object) bool {
				return obj == obj1
			})).Should(BeNumerically("<", 0))

			By("find objects not existing")
			Expect(graphCli.FindAll(dag, &appsv1.Deployment{})).Should(HaveLen(0))

			By("find objects different with the given type")
			newPodList := graphCli.FindAll(dag, &appsv1.StatefulSet{}, &HaveDifferentTypeWithOption{})
			Expect(newPodList).Should(HaveLen(3))
			// should have same result as podList
			for _, object := range podList {
				Expect(slices.IndexFunc(newPodList, func(obj client.Object) bool {
					return obj == object
				})).Should(BeNumerically(">=", 0))
			}

			By("find nil should return empty list")
			Expect(graphCli.FindAll(dag, nil)).Should(HaveLen(0))

			By("find all objects")
			objectList := graphCli.FindAll(dag, nil, &HaveDifferentTypeWithOption{})
			Expect(objectList).Should(HaveLen(4))
			allObjects := podList
			allObjects = append(allObjects, root)
			for _, object := range allObjects {
				Expect(slices.IndexFunc(objectList, func(obj client.Object) bool {
					return obj == object
				})).Should(BeNumerically(">=", 0))
			}

		})

		It("post init root vertex", func() {
			graphCli := NewGraphClient(nil)
			dag := graph.NewDAG()
			dagExpected := graph.NewDAG()

			By("create none root vertex first")
			obj := builder.NewPodBuilder(namespace, name+"0").GetObject()
			graphCli.Root(dag, obj, obj, ActionCreatePtr())
			v := &ObjectVertex{OriObj: obj, Obj: obj, Action: ActionCreatePtr()}
			dagExpected.AddVertex(v)
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())

			By("post create root vertex")
			root := builder.NewStatefulSetBuilder(namespace, name).GetObject()
			graphCli.Root(dag, root.DeepCopy(), root, ActionStatusPtr())
			rootVertex := &ObjectVertex{Obj: root, OriObj: root, Action: ActionStatusPtr()}
			dagExpected.AddVertex(rootVertex)
			dagExpected.Connect(rootVertex, v)
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())
		})

		It("IsAction should work", func() {
			graphCli := NewGraphClient(nil)
			dag := graph.NewDAG()

			By("create root vertex")
			obj := builder.NewPodBuilder(namespace, name+"0").GetObject()
			graphCli.Root(dag, obj, obj, ActionStatusPtr())
			Expect(graphCli.IsAction(dag, obj, ActionStatusPtr())).Should(BeTrue())
			Expect(graphCli.IsAction(dag, obj, ActionCreatePtr())).Should(BeFalse())

			By("vertex not existing")
			Expect(graphCli.IsAction(dag, &corev1.Pod{}, ActionStatusPtr())).Should(BeFalse())
			Expect(graphCli.IsAction(dag, &corev1.Pod{}, ActionCreatePtr())).Should(BeFalse())

			By("nil action")
			graphCli.Root(dag, obj, obj, nil)
			Expect(graphCli.IsAction(dag, obj, nil)).Should(BeTrue())
			Expect(graphCli.IsAction(dag, obj, ActionCreatePtr())).Should(BeFalse())
		})
	})
})
