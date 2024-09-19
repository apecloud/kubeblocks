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
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type clusterHaltTransformer struct{}

var _ graph.Transformer = &clusterHaltTransformer{}

func (t *clusterHaltTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.OrigCluster
	if !cluster.IsDeleting() || cluster.Spec.TerminationPolicy != appsv1.Halt {
		return nil
	}

	var (
		graphCli, _     = transCtx.Client.(model.GraphClient)
		ml              = getAppInstanceML(*cluster)
		toPreserveKinds = haltPreserveKinds()
	)
	return preserveClusterObjects(transCtx.Context, transCtx.Client, graphCli, dag, cluster, ml, toPreserveKinds)
}

func haltPreserveKinds() []client.ObjectList {
	return []client.ObjectList{
		&corev1.PersistentVolumeClaimList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
	}
}

// preserveClusterObjects preserves the objects owned by the cluster when the cluster is being deleted
func preserveClusterObjects(ctx context.Context, cli client.Reader, graphCli model.GraphClient, dag *graph.DAG,
	cluster *appsv1.Cluster, ml client.MatchingLabels, toPreserveKinds []client.ObjectList) error {
	return preserveObjects(ctx, cli, graphCli, dag, cluster, ml, toPreserveKinds, constant.DBClusterFinalizerName, constant.LastAppliedClusterAnnotationKey)
}
