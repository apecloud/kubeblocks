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

package apps

import (
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

// ComponentStatusTransformer computes the current status: read the underlying rsm status and update the component status
type ComponentStatusTransformer struct{}

var _ graph.Transformer = &ComponentStatusTransformer{}

func (t *ComponentStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ComponentTransformContext)
	comp := transCtx.Component
	compOrig := transCtx.ComponentOrig

	// fast return
	if model.IsObjectDeleting(compOrig) {
		return nil
	}

	switch {
	case model.IsObjectUpdating(compOrig):
		comp.Status.ObservedGeneration = comp.Generation
	case model.IsObjectStatusUpdating(compOrig):
		// TODO: read the underlying rsm status and update the component status
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Status(dag, compOrig, comp)

	return nil
}
