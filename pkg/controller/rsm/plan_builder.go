/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package rsm

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type PlanBuilder struct {
	req          ctrl.Request
	cli          client.Client
	transCtx     *rsmTransformContext
	transformers graph.TransformerChain
}

var _ graph.PlanBuilder = &PlanBuilder{}

type Plan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cli      client.Client
	transCtx *rsmTransformContext
}

var _ graph.Plan = &Plan{}

func init() {
	model.AddScheme(workloads.AddToScheme)
}

// PlanBuilder implementation

func (b *PlanBuilder) Init() error {
	rsm := &workloads.ReplicatedStateMachine{}
	if err := b.cli.Get(b.transCtx.Context, b.req.NamespacedName, rsm); err != nil {
		return err
	}
	b.AddTransformer(&initTransformer{ReplicatedStateMachine: rsm})
	return nil
}

func (b *PlanBuilder) AddTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	b.transformers = append(b.transformers, transformer...)
	return b
}

func (b *PlanBuilder) AddParallelTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	b.transformers = append(b.transformers, &model.ParallelTransformer{Transformers: transformer})
	return b
}

func (b *PlanBuilder) Build() (graph.Plan, error) {
	var err error
	// new a DAG and apply chain on it, after that we should get the final Plan
	dag := graph.NewDAG()
	err = b.transformers.ApplyTo(b.transCtx, dag)
	// log for debug
	b.transCtx.Logger.Info(fmt.Sprintf("DAG: %s", dag))

	// we got the execution plan
	plan := &Plan{
		dag:      dag,
		walkFunc: b.rsmWalkFunc,
		cli:      b.cli,
		transCtx: b.transCtx,
	}
	return plan, err
}

// Plan implementation

func (p *Plan) Execute() error {
	return p.dag.WalkReverseTopoOrder(p.walkFunc, nil)
}

// Do the real works

func (b *PlanBuilder) rsmWalkFunc(v graph.Vertex) error {
	vertex, ok := v.(*model.ObjectVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", v)
	}
	if vertex.Action == nil {
		return errors.New("vertex action can't be nil")
	}
	switch *vertex.Action {
	case model.CREATE:
		err := b.cli.Create(b.transCtx.Context, vertex.Obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	case model.UPDATE:
		err := b.cli.Update(b.transCtx.Context, vertex.Obj)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	case model.DELETE:
		finalizer := getFinalizer(vertex.Obj)
		if controllerutil.RemoveFinalizer(vertex.Obj, finalizer) {
			err := b.cli.Update(b.transCtx.Context, vertex.Obj)
			if err != nil && !apierrors.IsNotFound(err) {
				b.transCtx.Logger.Error(err, fmt.Sprintf("delete %T error: %s", vertex.Obj, vertex.Obj.GetName()))
				return err
			}
		}
		if !model.IsObjectDeleting(vertex.Obj) {
			err := b.cli.Delete(b.transCtx.Context, vertex.Obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	case model.STATUS:
		if err := b.cli.Status().Update(b.transCtx.Context, vertex.Obj); err != nil {
			return err
		}
	}
	return nil
}

// NewRSMPlanBuilder returns a RSMPlanBuilder powered PlanBuilder
func NewRSMPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, req ctrl.Request) graph.PlanBuilder {
	return &PlanBuilder{
		req: req,
		cli: cli,
		transCtx: &rsmTransformContext{
			Context:       ctx.Ctx,
			Client:        model.NewGraphClient(cli),
			EventRecorder: ctx.Recorder,
			Logger:        ctx.Log,
		},
	}
}
