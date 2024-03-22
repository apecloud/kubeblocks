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

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var (
	rsm         *workloads.ReplicatedStateMachine
	transCtx    *transformContext
	graphCli    model.GraphClient
	transformer *objectTree2DAGTransformer
	currentTree *ObjectTree
	desiredTree *ObjectTree
)

const (
	namespace = "foo"
	name      = "bar"
)

func less(v1, v2 graph.Vertex) bool {
	return model.DefaultLess(v1, v2)
}

var _ = Describe("object tree to dag transformer test.", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			AddLabels(constant.AppComponentLabelKey, name).
			SetReplicas(3).
			GetObject()
		transCtx = &transformContext{
			ctx:      ctx,
			cli:      model.NewGraphClient(k8sMock),
			recorder: nil,
			logger:   logger,
		}
		graphCli = model.NewGraphClient(k8sMock)
		currentTree = NewObjectTree()
		desiredTree = NewObjectTree()
		transformer = &objectTree2DAGTransformer{
			current: currentTree,
			desired: desiredTree,
		}
	})

	Context("Transform function", func() {
		It("should work well", func() {
			pod := builder.NewPodBuilder(namespace, name).GetObject()
			headlessSvc := builder.NewHeadlessServiceBuilder(namespace, name).GetObject()
			svc := builder.NewServiceBuilder(namespace, name).GetObject()
			env := builder.NewConfigMapBuilder(namespace, name+"-rsm-env").GetObject()

			dagExpected := graph.NewDAG()
			graphCli.Root(dagExpected, rsm, rsm.DeepCopy(), model.ActionStatusPtr())
			graphCli.Create(dagExpected, pod)
			graphCli.Create(dagExpected, headlessSvc)
			graphCli.Create(dagExpected, svc)
			graphCli.Create(dagExpected, env)
			graphCli.DependOn(dagExpected, pod, headlessSvc, svc, env)

			// do Transform
			currentTree.SetRoot(rsm)
			desiredTree.SetRoot(rsm)
			Expect(desiredTree.Add(pod, headlessSvc, svc, env)).Should(Succeed())
			dag := graph.NewDAG()
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())

			// compare DAGs
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})
})
