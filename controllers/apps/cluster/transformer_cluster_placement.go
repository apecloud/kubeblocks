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
	"fmt"
	"math/rand"
	"slices"
	"strings"

	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
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

	if !t.enabled(transCtx) {
		return nil
	}

	if err := t.precheck(transCtx); err != nil {
		return err
	}

	if t.assigned(transCtx) {
		return nil
	}

	contexts := t.assign(transCtx)
	cluster := transCtx.Cluster
	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string)
	}
	cluster.Annotations[constant.KBAppMultiClusterPlacementKey] = strings.Join(contexts, ",")

	return nil
}

func (t *clusterPlacementTransformer) enabled(transCtx *clusterTransformContext) bool {
	cluster := transCtx.OrigCluster
	_, ok := cluster.Annotations[constant.KBAppMultiClusterPlacementKey]
	return ok
}

func (t *clusterPlacementTransformer) precheck(transCtx *clusterTransformContext) error {
	if t.multiClusterMgr == nil {
		return fmt.Errorf("intend to create a multi-cluster object, but the multi-cluster manager is not set up properly")
	}

	var components []string
	for _, spec := range transCtx.components {
		if !ptr.Deref(spec.EnableInstanceAPI, false) {
			components = append(components, spec.Name)

		}
	}
	if len(components) > 0 {
		return fmt.Errorf("the multi-cluster object is only supported for components that enable the instance API: %s", strings.Join(components, ","))
	}

	var shardings []string
	for _, spec := range transCtx.shardings {
		if !ptr.Deref(spec.Template.EnableInstanceAPI, false) {
			shardings = append(shardings, spec.Name)
		}
	}
	if len(shardings) > 0 {
		return fmt.Errorf("the multi-cluster object is only supported for shardings that enable the instance API: %s", strings.Join(shardings, ","))
	}

	return nil
}

func (t *clusterPlacementTransformer) assigned(transCtx *clusterTransformContext) bool {
	cluster := transCtx.OrigCluster
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
