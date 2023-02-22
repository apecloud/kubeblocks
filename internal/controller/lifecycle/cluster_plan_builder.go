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
	"context"
	"errors"
	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/dag"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterPlanBuilder struct {
	Ctx context.Context
	Cli client.Client
	Cluster v1alpha1.Cluster
	ClusterDef v1alpha1.ClusterDefinition
	ClusterVersion v1alpha1.ClusterVersion
}

type ClusterPlan struct {
	dag *dag.DAG
	walkFunc dag.WalkFunc
}

func (b *ClusterPlanBuilder) Build() (dag.Plan, error) {
	graph := dag.New()
	transformers := []dag.GraphTransformer{
		&ClusterTransformer{},
		&CredentialTransformer{},
		&ConfigTransformer{},
		&CacheDiffTransformer{},
	}
	for _, transformer := range transformers {
		if err := transformer.Transform(graph);  err != nil {
			return nil, err
		}
	}

	walkFunc := func(node dag.Node) error {
		if node.Action == nil {
			return errors.New("node action can't be nil")
		}
		obj, ok := node.Obj.(client.Object)
		if !ok {
			return errors.New("node.Obj should be client.Object")
		}
		switch *node.Action {
		case dag.CREATE:
			return b.Cli.Create(b.Ctx, obj)
		case dag.UPDATE:
			return b.Cli.Update(b.Ctx, obj)
		case dag.DELETE:
			return b.Cli.Delete(b.Ctx, obj)
		}
		return nil
	}
	plan := &ClusterPlan{
		dag: graph,
		walkFunc: walkFunc,
	}
	return plan, nil
}

func (p *ClusterPlan) Execute() error {
	return p.dag.WalkBreadthFirst(p.walkFunc)
}