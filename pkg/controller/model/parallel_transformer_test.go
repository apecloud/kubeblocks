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

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

var _ = Describe("parallel transformer test", func() {
	Context("Transform function", func() {
		It("should work well", func() {
			id1 := 1
			transformer := &ParallelTransformer{
				Transformers: []graph.Transformer{
					&testTransformer{id: id1},
				},
			}
			dag := graph.NewDAG()
			// TODO(free6om): DAG is not thread-safe currently, so parallel transformer has concurrent map writes issue.
			// parallel more transformers when DAG is ready.
			Expect(transformer.Transform(nil, dag)).Should(Succeed())
			dagExpected := graph.NewDAG()
			dagExpected.AddVertex(id1)
			Expect(dag.Equals(dagExpected, DefaultLess)).Should(BeTrue())
		})
	})
})

type testTransformer struct {
	id int
}

var _ graph.Transformer = &testTransformer{}

func (t *testTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	dag.AddVertex(t.id)
	return nil
}
