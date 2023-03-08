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
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type clusterPlanBuilder struct {
	ctx     intctrlutil.RequestCtx
	cli     client.Client
	cluster *appsv1alpha1.Cluster
}

type clusterPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
}

func (b *clusterPlanBuilder) getCompoundCluster() (*compoundCluster, error) {
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := b.cli.Get(b.ctx.Ctx, types.NamespacedName{
		Name: b.cluster.Spec.ClusterDefRef,
	}, cd); err != nil {
		return nil, err
	}
	cv := &appsv1alpha1.ClusterVersion{}
	if err := b.cli.Get(b.ctx.Ctx, types.NamespacedName{
		Name: b.cluster.Spec.ClusterVersionRef,
	}, cv); err != nil {
		return nil, err
	}

	cc := &compoundCluster{
		cluster: b.cluster,
		cd:      *cd,
		cv:      *cv,
	}
	return cc, nil
}

// Build only cluster Creation, Update and Deletion supported.
// TODO: Validations and Corrections (cluster labels correction, primaryIndex spec validation etc.)
func (b *clusterPlanBuilder) Build() (graph.Plan, error) {
	cc, err := b.getCompoundCluster()
	if err != nil {
		return nil, err
	}

	// build transformer chain
	chain := &graph.TransformerChain{
		// cluster to K8s objects and put them into dag
		&clusterTransformer{cc: *cc, cli: b.cli, ctx: b.ctx},
		// add our finalizer to all objects
		&finalizerSetterTransformer{finalizer: dbClusterFinalizerName},
		// make all workload objects depending on credential secret
		&credentialTransformer{},
		// make config configmap immutable
		&configTransformer{},
		// read old snapshot from cache, and generate diff plan
		&cacheDiffTransformer{cc: *cc, cli: b.cli, ctx: b.ctx},
		// finally, update cluster status
		&clusterStatusTransformer{*cc},
	}

	// new a DAG and apply chain on it, after that we should get the final Plan
	dag := graph.NewDAG()
	if err := chain.WalkThrough(dag); err != nil {
		return nil, err
	}

	// prepare the action func we want to apply to each DAG vertex
	walkFunc := func(node graph.Vertex) error {
		obj, ok := node.(*lifecycleVertex)
		if !ok {
			return fmt.Errorf("wrong node type %v", node)
		}
		if obj.action == nil {
			return errors.New("node action can't be nil")
		}
		switch *obj.action {
		case CREATE:
			return b.cli.Create(b.ctx.Ctx, obj.obj)
		case UPDATE:
			return b.cli.Update(b.ctx.Ctx, obj.obj)
		case DELETE:
			return b.cli.Delete(b.ctx.Ctx, obj.obj)
		case STATUS:
			return b.cli.Patch(b.ctx.Ctx, obj.obj, obj.patch)
		}
		return nil
	}

	// we got the execution plan
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: walkFunc,
	}
	return plan, nil
}

// NewClusterPlanBuilder returns a clusterPlanBuilder powered PlanBuilder
// TODO: change ctx to context.Context
func NewClusterPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster) graph.PlanBuilder {
	return &clusterPlanBuilder{
		ctx:     ctx,
		cli:     cli,
		cluster: cluster,
	}
}

func (p *clusterPlan) Execute() error {
	return p.dag.WalkReverseTopoOrder(p.walkFunc)
}
