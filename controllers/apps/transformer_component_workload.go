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
	"github.com/apecloud/kubeblocks/internal/controller/factory"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

// ComponentWorkloadTransformer handles component rsm workload generation
type ComponentWorkloadTransformer struct{}

var _ graph.Transformer = &ComponentWorkloadTransformer{}

func (t *ComponentWorkloadTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// TODO: build or update rsm workload
	transCtx, _ := ctx.(*ComponentTransformContext)
	compOrig := transCtx.ComponentOrig

	if model.IsObjectDeleting(compOrig) {
		return nil
	}

	cluster := transCtx.Cluster
	synthesizeComp := transCtx.SynthesizeComponent

	// build rsm workload
	// TODO(xingran): BuildRSM relies on the deprecated fields of the component, for example component.WorkloadType, which should be removed in the future
	_, err := factory.BuildRSM(cluster, synthesizeComp)
	if err != nil {
		return err
	}

	return nil
}
