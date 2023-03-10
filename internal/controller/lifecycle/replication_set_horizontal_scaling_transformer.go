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

	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type rplSetHorizontalScalingTransformer struct {
	cc  compoundCluster
	cli client.Client
	ctx intctrlutil.RequestCtx
}

func (r *rplSetHorizontalScalingTransformer) Transform(dag *graph.DAG) error {
	vertices, err := findAll[*appsv1.StatefulSet](dag)
	if err != nil {
		return err
	}
	// stsList is used to handle statefulSets horizontal scaling when workloadType is replication
	var stsList []*appsv1.StatefulSet
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		stsList = append(stsList, v.obj.(*appsv1.StatefulSet))
	}
	if err := replicationset.HandleReplicationSet(r.ctx.Ctx, r.cli, r.cc.cluster, stsList); err != nil {
		return err
	}

	return nil
}
