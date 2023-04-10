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

package consensusset

import (
	"context"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type consensusSetPlanBuilder struct {
	context.Context
	client.Client
	ctrl.Request
	logr.Logger
	record.EventRecorder
	*workloads.ConsensusSet
}

const consensusSetFinalizerName = "cs.workloads.kubeblocks.io/finalizer"

func init() {
	model.AddScheme(workloads.AddToScheme)
}

func (p *consensusSetPlanBuilder) Init() error {
	csSet := &workloads.ConsensusSet{}
	if err := p.Client.Get(p.Context, p.Request.NamespacedName, csSet); err != nil {
		return err
	}
	p.ConsensusSet = csSet
	return nil
}

func (p *consensusSetPlanBuilder) Validate() error {
	return nil
}

func (p *consensusSetPlanBuilder) Build() (graph.Plan, error) {
	chain := &graph.TransformerChain{
		// add root vertex
		&initTransformer{ConsensusSet: p.ConsensusSet},
		// generate objects and actions
		&objectGenerationTransformer{ctx: p.Context, cli: p.Client},
	}

	dag := graph.NewDAG()
	if err := chain.ApplyTo(dag); err != nil {
		return nil, err
	}

	return &consensusSetPlan{
		DAG: dag,
		WalkFunc: p.consensusSetWalkFunc,
	}, nil
}

func (p *consensusSetPlanBuilder) consensusSetWalkFunc(vertex graph.Vertex) error {
	return nil
}

type consensusSetPlan struct {
	*graph.DAG
	graph.WalkFunc
}

func (p *consensusSetPlan) Execute() error {
	return p.DAG.WalkReverseTopoOrder(p.WalkFunc)
}

func NewPlanBuilder(ctx context.Context,
	cli client.Client,
	request ctrl.Request,
	logger logr.Logger,
	recorder record.EventRecorder) graph.PlanBuilder {
	return &consensusSetPlanBuilder{
		Context:       ctx,
		Client:        cli,
		Request:       request,
		Logger:        logger,
		EventRecorder: recorder,
	}
}

var _ graph.PlanBuilder = &consensusSetPlanBuilder{}
var _ graph.Plan = &consensusSetPlan{}