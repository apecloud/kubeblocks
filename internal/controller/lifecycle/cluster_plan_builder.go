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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/dag"
)


type Action string
const (
	CREATE = "CREATE"
	UPDATE = "UPDATE"
	DELETE = "DELETE"
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

type planObject struct {
	obj client.Object
	immutable bool
	action *Action
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

	walkFunc := func(node dag.Vertex) error {
		obj, ok := node.(*planObject)
		if !ok {
			return fmt.Errorf("wrong node type %v", node)
		}
		if obj.action == nil {
			return errors.New("node action can't be nil")
		}
		switch *obj.action {
		case CREATE:
			return b.Cli.Create(b.Ctx, obj.obj)
		case UPDATE:
			return b.Cli.Update(b.Ctx, obj.obj)
		case DELETE:
			return b.Cli.Delete(b.Ctx, obj.obj)
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
	return p.dag.WalkDepthFirst(p.walkFunc)
}

func NewClusterPlanBuilder(
	ctx context.Context, cli client.Client, cluster v1alpha1.Cluster, clusterDef v1alpha1.ClusterDefinition, version v1alpha1.ClusterVersion) ClusterPlanBuilder {
	return ClusterPlanBuilder{
		Ctx: ctx,
		Cli: cli,
		Cluster: cluster,
		ClusterDef: clusterDef,
		ClusterVersion: version,
	}
}