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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type FixMetaTransformer struct{}

var _ graph.Transformer = &FixMetaTransformer{}

func (t *FixMetaTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rsmTransformContext)
	obj := transCtx.rsm
	if model.IsObjectDeleting(obj) {
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	finalizer := getFinalizer(obj)
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		return nil
	}
	controllerutil.AddFinalizer(obj, finalizer)
	graphCli.Update(dag, transCtx.rsmOrig, obj)

	return graph.ErrPrematureStop
}
