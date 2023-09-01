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

package rsm

import (
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	model2 "github.com/apecloud/kubeblocks/pkg/controller/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("init transformer test.", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			SetReplicas(3).
			GetObject()

		transCtx = &rsmTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
		}

		dag = graph.NewDAG()
		transformer = &initTransformer{ReplicatedStateMachine: rsm}
	})

	Context("dag init", func() {
		It("should work well", func() {
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			dagExpected := graph.NewDAG()
			root := &model2.ObjectVertex{
				Obj:    rsm,
				OriObj: rsm.DeepCopy(),
				Action: model2.ActionPtr(model2.STATUS),
			}
			dagExpected.AddVertex(root)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})
})
