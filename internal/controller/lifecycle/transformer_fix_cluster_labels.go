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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/types"
)

// fixClusterLabelsTransformer should patch the label first to prevent the label from being modified by the user.
type fixClusterLabelsTransformer struct{}

func (f *fixClusterLabelsTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := types.FindRootVertex(dag)
	if err != nil {
		return err
	}
	cluster, _ := rootVertex.Obj.(*appsv1alpha1.Cluster)
	labels := cluster.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	cdLabelName := labels[clusterDefLabelKey]
	cvLabelName := labels[clusterVersionLabelKey]
	cdName, cvName := cluster.Spec.ClusterDefRef, cluster.Spec.ClusterVersionRef
	if cdLabelName == cdName && cvLabelName == cvName {
		return nil
	}
	labels[clusterDefLabelKey] = cdName
	labels[clusterVersionLabelKey] = cvName
	cluster.Labels = labels
	return nil
}
