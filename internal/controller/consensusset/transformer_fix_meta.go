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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

type FixMetaTransformer struct{}

func (t *FixMetaTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	csSet := transCtx.CSSet
	if model.IsObjectDeleting(csSet) {
		return nil
	}

	root, err := model.FindRootVertex(dag)
	if err != nil {
		return err
	}

	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	if controllerutil.ContainsFinalizer(csSet, CSSetFinalizerName) {
		return nil
	}
	csSetCopy := csSet.DeepCopy()
	controllerutil.AddFinalizer(csSetCopy, CSSetFinalizerName)
	vertex := &model.ObjectVertex{Obj: csSetCopy, OriObj: transCtx.OrigCSSet, Action: model.ActionPtr(model.UPDATE)}
	dag.AddConnect(root, vertex)

	return nil
}

var _ graph.Transformer = &FixMetaTransformer{}
