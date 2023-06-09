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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

type initTransformer struct {
	cluster       *appsv1alpha1.Cluster
	originCluster *appsv1alpha1.Cluster
}

var _ graph.Transformer = &initTransformer{}

func (t *initTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// put the cluster object first, it will be root vertex of DAG
	rootVertex := &ictrltypes.LifecycleVertex{Obj: t.cluster, ObjCopy: t.originCluster, Action: ictrltypes.ActionStatusPtr()}
	dag.AddVertex(rootVertex)

	if !t.cluster.IsDeleting() {
		t.handleLatestOpsRequestProcessingCondition()
	}
	if t.cluster.IsUpdating() {
		t.handleClusterPhase()
	}
	return nil
}

func (t *initTransformer) handleClusterPhase() {
	clusterPhase := t.cluster.Status.Phase
	if clusterPhase == "" {
		t.cluster.Status.Phase = appsv1alpha1.CreatingClusterPhase
	} else if clusterPhase != appsv1alpha1.CreatingClusterPhase {
		t.cluster.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase
	}
}

// updateLatestOpsRequestProcessingCondition handles the latest opsRequest processing condition.
func (t *initTransformer) handleLatestOpsRequestProcessingCondition() {
	opsRecords, _ := opsutil.GetOpsRequestSliceFromCluster(t.cluster)
	if len(opsRecords) == 0 {
		return
	}
	ops := opsRecords[0]
	opsBehaviour, ok := appsv1alpha1.OpsRequestBehaviourMapper[ops.Type]
	if !ok {
		return
	}
	opsCondition := newOpsRequestProcessingCondition(ops.Name, string(ops.Type), opsBehaviour.ProcessingReasonInClusterCondition)
	oldCondition := meta.FindStatusCondition(t.cluster.Status.Conditions, opsCondition.Type)
	if oldCondition == nil {
		// if this condition not exists, insert it to the first position.
		opsCondition.LastTransitionTime = metav1.Now()
		t.cluster.Status.Conditions = append([]metav1.Condition{opsCondition}, t.cluster.Status.Conditions...)
	} else {
		meta.SetStatusCondition(&t.cluster.Status.Conditions, opsCondition)
	}
}
