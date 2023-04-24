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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
)

type PITRTransformer struct {
	client.Client
}

var _ graph.Transformer = &PITRTransformer{}

func (t *PITRTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	// handle PITR only when cluster is in status reconciliation stage
	if !cluster.IsStatusUpdating() {
		return nil
	}
	// TODO: (free6om) refactor: remove client.Client
	// TODO(refactor): PITR will update cluster annotations and sts spec, create and delete pvc & job resources, resolve them later.
	if shouldRequeue, err := plan.DoPITRIfNeed(transCtx.Context, t.Client, cluster); err != nil {
		return err
	} else if shouldRequeue {
		return &realRequeueError{reason: "waiting pitr job", requeueAfter: requeueDuration}
	}
	if err := plan.DoPITRCleanup(transCtx.Context, t.Client, cluster); err != nil {
		return err
	}
	return nil
}
