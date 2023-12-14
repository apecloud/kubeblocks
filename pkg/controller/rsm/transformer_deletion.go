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
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// ObjectDeletionTransformer handles object and its secondary resources' deletion
type ObjectDeletionTransformer struct{}

var _ graph.Transformer = &ObjectDeletionTransformer{}

func (t *ObjectDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rsmTransformContext)
	obj := transCtx.rsm
	if !model.IsObjectDeleting(obj) {
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	// list all objects owned by this primary obj in cache, and delete them all
	// there is chance that objects leak occurs because of cache stale
	// ignore the problem currently
	// TODO: GC the leaked objects
	ml := getLabels(obj)
	snapshot, err := model.ReadCacheSnapshot(transCtx, obj, ml, deletionKinds(transCtx.rsm.Spec.RsmTransformPolicy)...)
	if err != nil {
		return err
	}
	for _, object := range snapshot {
		// don't delete cm that not created by rsm
		if IsOwnedByRsm(object) {
			graphCli.Delete(dag, object)
		}
	}
	graphCli.Delete(dag, obj)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}
