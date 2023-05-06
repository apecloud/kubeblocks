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
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

// CSSetDeletionTransformer handles ConsensusSet deletion
type CSSetDeletionTransformer struct{}

func (t *CSSetDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	obj := transCtx.CSSet
	if !model.IsObjectDeleting(obj) {
		return nil
	}

	// list all objects owned by this primary obj in cache, and delete them all
	// there is chance that objects leak occurs because of cache stale
	// ignore the problem currently
	// TODO: GC the leaked objects
	snapshot, err := model.ReadCacheSnapshot(transCtx, obj, ownedKinds()...)
	if err != nil {
		return err
	}
	root, err := model.FindRootVertex(dag)
	if err != nil {
		return err
	}
	for _, object := range snapshot {
		vertex := &model.ObjectVertex{Obj: object, Action: model.ActionPtr(model.DELETE)}
		dag.AddConnect(root, vertex)
	}
	root.Action = model.ActionPtr(model.DELETE)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrFastReturn
}

var _ graph.Transformer = &CSSetDeletionTransformer{}
