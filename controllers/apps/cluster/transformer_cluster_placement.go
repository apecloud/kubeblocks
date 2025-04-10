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

package cluster

import (
	"math/rand"
	"slices"
	"strings"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

// clusterPlacementTransformer handles replicas placement.
type clusterPlacementTransformer struct {
	multiClusterMgr multicluster.Manager
}

var _ graph.Transformer = &clusterPlacementTransformer{}

func (t *clusterPlacementTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if transCtx.OrigCluster.IsDeleting() {
		return nil
	}

	if t.multiClusterMgr == nil {
		return nil // do nothing
	}

	if t.assigned(transCtx) {
		transCtx.Context = appsutil.IntoContext(transCtx.Context, appsutil.Placement(transCtx.OrigCluster))
		return nil
	}

	p := t.assign(transCtx)

	cluster := transCtx.Cluster
	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string)
	}
	cluster.Annotations[constant.KBAppMultiClusterPlacementKey] = strings.Join(p, ",")
	transCtx.Context = appsutil.IntoContext(transCtx.Context, appsutil.Placement(cluster))

	return nil
}

func (t *clusterPlacementTransformer) assigned(transCtx *clusterTransformContext) bool {
	cluster := transCtx.OrigCluster
	if cluster.Annotations == nil {
		return false
	}

	p, ok := cluster.Annotations[constant.KBAppMultiClusterPlacementKey]
	return ok && len(strings.TrimSpace(p)) > 0
}

func (t *clusterPlacementTransformer) assign(transCtx *clusterTransformContext) []string {
	replicas := t.maxReplicas(transCtx)
	contexts := t.multiClusterMgr.GetContexts()
	if replicas >= len(contexts) {
		return contexts
	}

	slices.Sort(contexts)
	for k := 0; k < len(contexts); k++ {
		rand.Shuffle(len(contexts), func(i, j int) {
			contexts[i], contexts[j] = contexts[j], contexts[i]
		})
	}
	return contexts[:replicas]
}

func (t *clusterPlacementTransformer) maxReplicas(transCtx *clusterTransformContext) int {
	replicas := 0
	transCtx.traverse(func(spec *appsv1.ClusterComponentSpec) {
		replicas = max(replicas, int(spec.Replicas))
	})
	return replicas
}
