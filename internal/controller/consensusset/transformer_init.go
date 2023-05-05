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

package consensusset

import (
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

type initTransformer struct {
	*workloads.ConsensusSet
}

func (t *initTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	obj, origObj := t.ConsensusSet, t.ConsensusSet.DeepCopy()
	// init context
	transCtx, _ := ctx.(*CSSetTransformContext)
	transCtx.CSSet, transCtx.OrigCSSet = obj, origObj

	// init dag
	vertex := &model.ObjectVertex{Obj: transCtx.CSSet, OriObj: transCtx.OrigCSSet}
	dag.AddVertex(vertex)
	return nil
}

var _ graph.Transformer = &initTransformer{}
