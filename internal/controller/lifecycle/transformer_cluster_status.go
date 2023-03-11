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
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type clusterStatusTransformer struct {
	compoundCluster
}

func (c *clusterStatusTransformer) Transform(dag *graph.DAG) error {
	if !c.cluster.DeletionTimestamp.IsZero() {
		return nil
	}

	// get root(cluster) vertex
	rootVertex := dag.Root()
	if rootVertex == nil {
		return fmt.Errorf("root vertex not found: %v", dag)
	}
	root, _ := rootVertex.(*lifecycleVertex)
	cluster, _ := root.obj.(*appsv1alpha1.Cluster)
	// apply resources succeed, record the condition and event
	applyResourcesCondition := newApplyResourcesCondition()
	cluster.SetStatusCondition(applyResourcesCondition)
	// if cluster status is ConditionsError, do it before updated the observedGeneration.
	updateClusterPhaseWhenConditionsError(cluster)
	// update observed generation
	cluster.Status.ObservedGeneration = cluster.Generation
	cluster.Status.ClusterDefGeneration = c.cd.Generation
	// TODO: emit event
	//r.Recorder.Event(cluster, corev1.EventTypeNormal, applyResourcesCondition.Reason, applyResourcesCondition.Message)
	root.action = actionPtr(STATUS)
	return nil
}
