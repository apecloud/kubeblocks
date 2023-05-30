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

package lifecycle

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
)

type PITRTransformer struct {
	client.Client
}

var _ graph.Transformer = &PITRTransformer{}

func (t *PITRTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	// handle PITR only when cluster is in status reconciliation stage
	if !cluster.IsStatusUpdating() {
		return nil
	}
	// TODO: (free6om) refactor: remove client.Client
	// TODO(refactor): PITR will update cluster annotations and sts spec, create and delete pvc & job resources, resolve them later.
	if shouldRequeue, err := plan.DoPITRIfNeed(transCtx.Context, t.Client, cluster); err != nil {
		return err
	} else if shouldRequeue {
		return newRequeueError(requeueDuration, "waiting pitr job")
	}
	if err := plan.DoPITRCleanup(transCtx.Context, t.Client, cluster); err != nil {
		return err
	}
	return nil
}
