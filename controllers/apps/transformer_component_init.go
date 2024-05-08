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

package apps

import (
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type componentInitTransformer struct{}

var _ graph.Transformer = &componentInitTransformer{}

func (t *componentInitTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)

	// init dag
	rootVertex := &model.ObjectVertex{Obj: transCtx.Component, OriObj: transCtx.ComponentOrig, Action: model.ActionStatusPtr()}
	dag.AddVertex(rootVertex)

	// init placement
	transCtx.Context = intoContext(transCtx.Context, placement(transCtx.Component))

	return nil
}
