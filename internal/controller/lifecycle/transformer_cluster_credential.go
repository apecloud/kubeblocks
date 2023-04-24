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
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

// ClusterCredentialTransformer creates the connection credential secret
type ClusterCredentialTransformer struct{}

var _ graph.Transformer = &ClusterCredentialTransformer{}

func (c *ClusterCredentialTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if cluster.IsDeleting() {
		return nil
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	var secret *corev1.Secret
	for _, compDef := range transCtx.ClusterDef.Spec.ComponentDefs {
		if compDef.Service == nil {
			continue
		}

		component := &component.SynthesizedComponent{
			Services: []corev1.Service{
				{Spec: compDef.Service.ToSVCSpec()},
			},
		}
		if secret, err = builder.BuildConnCredentialLow(transCtx.ClusterDef, cluster, component); err != nil {
			return err
		}
		break
	}

	if secret != nil {
		ictrltypes.LifecycleObjectCreate(dag, secret, root)
	}
	return nil
}
