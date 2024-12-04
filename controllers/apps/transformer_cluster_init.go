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
	"github.com/pkg/errors"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type clusterInitTransformer struct {
	cluster *appsv1.Cluster
}

var _ graph.Transformer = &clusterInitTransformer{}

func (t *clusterInitTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	transCtx.Cluster, transCtx.OrigCluster = t.cluster, t.cluster.DeepCopy()
	graphCli, _ := transCtx.Client.(model.GraphClient)

	supported, err := intctrlutil.APIVersionPredicate(t.cluster)
	if err != nil {
		return errors.Wrap(err, "API version predicate failed")
	}
	if !supported {
		return graph.ErrPrematureStop
	}

	// init dag
	graphCli.Root(dag, transCtx.OrigCluster, transCtx.Cluster, model.ActionStatusPtr())
	return nil
}
