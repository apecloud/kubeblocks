/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package rollout

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type rolloutMetaTransformer struct{}

var _ graph.Transformer = &rolloutMetaTransformer{}

func (t *rolloutMetaTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx := ctx.(*rolloutTransformContext)
	rollout := transCtx.Rollout

	controllerutil.AddFinalizer(rollout, constant.RolloutFinalizerName)

	if reflect.DeepEqual(transCtx.RolloutOrig.Finalizers, rollout.Finalizers) {
		return nil
	}

	graphCli := transCtx.Client.(model.GraphClient)
	graphCli.Update(dag, transCtx.RolloutOrig, rollout)

	return graph.ErrPrematureStop
}
