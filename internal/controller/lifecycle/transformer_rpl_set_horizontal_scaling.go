/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type RplSetHorizontalScalingTransformer struct {
	client.Client
}

func (t *RplSetHorizontalScalingTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	origCluster := transCtx.OrigCluster
	cluster := transCtx.Cluster

	if isClusterDeleting(*origCluster) {
		return nil
	}

	hasScaling, err := t.hasReplicationSetHScaling(transCtx, *cluster)
	if err != nil {
		return err
	}
	if !hasScaling {
		return nil
	}
	vertices := findAll[*appsv1.StatefulSet](dag)
	// stsList is used to handle statefulSets horizontal scaling when workloadType is replication
	var stsList []*appsv1.StatefulSet
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		stsList = append(stsList, v.obj.(*appsv1.StatefulSet))
	}
	if err := replicationset.HandleReplicationSet(transCtx.Context, t.Client, cluster, stsList); err != nil {
		return err
	}

	return nil
}

// TODO: fix stale cache problem
// TODO: if sts created in last reconcile-loop not present in cache, hasReplicationSetHScaling return false positive
func (t *RplSetHorizontalScalingTransformer) hasReplicationSetHScaling(transCtx *ClusterTransformContext, cluster appsv1alpha1.Cluster) (bool, error) {
	stsList, err := t.listAllStsOwnedByCluster(transCtx, cluster)
	if err != nil {
		return false, err
	}
	if len(stsList) == 0 {
		return false, err
	}

	for _, compDef := range transCtx.ClusterDef.Spec.ComponentDefs {
		if compDef.WorkloadType == appsv1alpha1.Replication {
			return true, nil
		}
	}

	return false, nil
}

func (t *RplSetHorizontalScalingTransformer) listAllStsOwnedByCluster(transCtx *ClusterTransformContext, cluster appsv1alpha1.Cluster) ([]appsv1.StatefulSet, error) {
	stsList := &appsv1.StatefulSetList{}
	if err := transCtx.Client.List(transCtx.Context, stsList,
		client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name},
		client.InNamespace(cluster.Namespace)); err != nil {
		return nil, err
	}
	allSts := make([]appsv1.StatefulSet, 0)
	allSts = append(allSts, stsList.Items...)
	return allSts, nil
}
