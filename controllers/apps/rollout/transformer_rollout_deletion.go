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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type rolloutDeletionTransformer struct{}

var _ graph.Transformer = &rolloutDeletionTransformer{}

func (t *rolloutDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rolloutTransformContext)
	if !model.IsObjectDeleting(transCtx.RolloutOrig) {
		return nil
	}

	var (
		graphCli, _ = transCtx.Client.(model.GraphClient)
		rollout     = transCtx.Rollout
	)

	// delete the rollout label from the cluster
	clusterKey := types.NamespacedName{
		Namespace: rollout.Namespace,
		Name:      rollout.Spec.ClusterName,
	}
	cluster := &appsv1.Cluster{}
	err := transCtx.Client.Get(transCtx.Context, clusterKey, cluster)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err == nil {
		clusterCopy := cluster.DeepCopy()
		delete(cluster.Labels, rolloutNameClusterLabel)
		if !reflect.DeepEqual(clusterCopy.Labels, cluster.Labels) {
			graphCli.Update(dag, clusterCopy, cluster)
		}
	}

	// TODO: impl
	graphCli.Delete(dag, rollout)

	return nil
}
