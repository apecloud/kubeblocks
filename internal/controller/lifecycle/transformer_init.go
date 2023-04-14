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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type initTransformer struct {
	cluster       *appsv1alpha1.Cluster
	originCluster *appsv1alpha1.Cluster
}

func (t *initTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// put the cluster object first, it will be root vertex of DAG
	rootVertex := &lifecycleVertex{obj: t.cluster, oriObj: t.originCluster, action: actionPtr(STATUS)}
	dag.AddVertex(rootVertex)

	if !isClusterDeleting(*t.cluster) {
		t.handleLatestOpsRequestProcessingCondition()
	}
	return nil
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

var _ graph.Transformer = &initTransformer{}
