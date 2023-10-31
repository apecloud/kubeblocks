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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type initTransformer struct {
	*workloads.ReplicatedStateMachine
}

var _ graph.Transformer = &initTransformer{}

func (t *initTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// init context
	transCtx, _ := ctx.(*rsmTransformContext)
	transCtx.rsm, transCtx.rsmOrig = t.ReplicatedStateMachine, t.ReplicatedStateMachine.DeepCopy()
	graphCli, _ := transCtx.Client.(model.GraphClient)

	// stop reconciliation if paused=true
	if t.ReplicatedStateMachine.Spec.Paused {
		graphCli.Root(dag, transCtx.rsmOrig, transCtx.rsm, model.ActionNoopPtr())
		return graph.ErrPrematureStop
	}

	// init dag
	graphCli.Root(dag, transCtx.rsmOrig, transCtx.rsm, model.ActionStatusPtr())

	return nil
}
