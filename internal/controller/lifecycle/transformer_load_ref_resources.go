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
	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// LoadRefResourcesTransformer injects cd & cv into the shared transCtx, a little bit hacky
// TODO: a better design to load cd & cv into the transformer chain
type LoadRefResourcesTransformer struct {}

func (t *LoadRefResourcesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	if isClusterDeleting(*transCtx.OrigCluster) {
		return nil
	}
	cluster := transCtx.Cluster
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := transCtx.Client.Get(transCtx.Context, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name: cluster.Spec.ClusterDefRef}, cd); err != nil {
		return err
	}
	var cv *appsv1alpha1.ClusterVersion
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		cv = &appsv1alpha1.ClusterVersion{}
		if err := transCtx.Client.Get(transCtx.Context, types.NamespacedName{
			Namespace: cluster.Namespace,
			Name: cluster.Spec.ClusterVersionRef,
		}, cv); err != nil {
			return err
		}
	}

	// inject cd & cv into the shared ctx
	transCtx.ClusterDef = cd
	transCtx.ClusterVer = cv

	return nil
}

var _ graph.Transformer = &LoadRefResourcesTransformer{}