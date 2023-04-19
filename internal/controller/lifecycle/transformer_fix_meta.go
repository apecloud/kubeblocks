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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type FixMetaTransformer struct{}

func (t *FixMetaTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster

	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	if !controllerutil.ContainsFinalizer(cluster, dbClusterFinalizerName) {
		controllerutil.AddFinalizer(cluster, dbClusterFinalizerName)
	}

	// patch the label to prevent the label from being modified by the user.
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

var _ graph.Transformer = &FixMetaTransformer{}
