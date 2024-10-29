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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterOwnershipTransformer adds finalizer to all none cluster objects
type clusterOwnershipTransformer struct{}

var _ graph.Transformer = &clusterOwnershipTransformer{}

func (f *clusterOwnershipTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	graphCli, _ := transCtx.Client.(model.GraphClient)
	cluster := transCtx.Cluster

	objects := graphCli.FindAll(dag, &appsv1.Cluster{}, &model.HaveDifferentTypeWithOption{})

	controllerutil.AddFinalizer(cluster, constant.DBClusterFinalizerName)
	for _, object := range objects {
		if err := intctrlutil.SetOwnership(cluster, object, rscheme, constant.DBClusterFinalizerName); err != nil {
			if _, ok := err.(*controllerutil.AlreadyOwnedError); ok {
				continue
			}
			return err
		}
	}
	return nil
}
