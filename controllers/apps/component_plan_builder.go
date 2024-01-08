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

package apps

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentTransformContext a graph.TransformContext implementation for Cluster reconciliation
type componentTransformContext struct {
	context.Context
	Client client.Reader
	record.EventRecorder
	logr.Logger
	Cluster             *appsv1alpha1.Cluster
	CompDef             *appsv1alpha1.ComponentDefinition
	Component           *appsv1alpha1.Component
	ComponentOrig       *appsv1alpha1.Component
	SynthesizeComponent *component.SynthesizedComponent
	RunningWorkload     client.Object
	ProtoWorkload       client.Object
}

func (c *componentTransformContext) GetContext() context.Context {
	return c.Context
}

func (c *componentTransformContext) GetClient() client.Reader {
	return c.Client
}

func (c *componentTransformContext) GetRecorder() record.EventRecorder {
	return c.EventRecorder
}

func (c *componentTransformContext) GetLogger() logr.Logger {
	return c.Logger
}

// componentPlanBuilder a graph.PlanBuilder implementation for Component reconciliation
type componentPlanBuilder struct {
	req          ctrl.Request
	cli          client.Client
	transCtx     *componentTransformContext
	transformers graph.TransformerChain
}

// clusterPlan a graph.Plan implementation for Cluster reconciliation
type componentPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cli      client.Client
	transCtx *componentTransformContext
}

var _ graph.TransformContext = &componentTransformContext{}
var _ graph.PlanBuilder = &componentPlanBuilder{}
var _ graph.Plan = &componentPlan{}

func (c *componentPlanBuilder) Init() error {
	comp := &appsv1alpha1.Component{}
	if err := c.cli.Get(c.transCtx.Context, c.req.NamespacedName, comp); err != nil {
		return err
	}

	c.transCtx.Component = comp
	c.transCtx.ComponentOrig = comp.DeepCopy()
	c.transformers = append(c.transformers, &componentInitTransformer{
		Component:     c.transCtx.Component,
		ComponentOrig: c.transCtx.ComponentOrig,
	})
	return nil
}

func (c *componentPlanBuilder) AddTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	c.transformers = append(c.transformers, transformer...)
	return c
}

func (c *componentPlanBuilder) AddParallelTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	c.transformers = append(c.transformers, &ParallelTransformers{transformers: transformer})
	return c
}

// Build runs all transformers to generate a plan
func (c *componentPlanBuilder) Build() (graph.Plan, error) {
	dag := graph.NewDAG()
	err := c.transformers.ApplyTo(c.transCtx, dag)
	if err != nil {
		c.transCtx.Logger.V(1).Info(fmt.Sprintf("build error: %s", err.Error()))
	}
	c.transCtx.Logger.V(1).Info(fmt.Sprintf("DAG: %s", dag))

	plan := &componentPlan{
		dag:      dag,
		walkFunc: c.defaultWalkFuncWithLogging,
		cli:      c.cli,
		transCtx: c.transCtx,
	}
	return plan, err
}

func (p *componentPlan) Execute() error {
	err := p.dag.WalkReverseTopoOrder(p.walkFunc, nil)
	if err != nil {
		p.transCtx.Logger.V(1).Info(fmt.Sprintf("execute error: %s", err.Error()))
	}
	return err
}

// newComponentPlanBuilder returns a componentPlanBuilder powered PlanBuilder
func newComponentPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, req ctrl.Request) graph.PlanBuilder {
	return &componentPlanBuilder{
		req: req,
		cli: cli,
		transCtx: &componentTransformContext{
			Context:       ctx.Ctx,
			Client:        model.NewGraphClient(cli),
			EventRecorder: ctx.Recorder,
			Logger:        ctx.Log,
		},
	}
}

func (c *componentPlanBuilder) defaultWalkFuncWithLogging(vertex graph.Vertex) error {
	node, ok := vertex.(*model.ObjectVertex)
	err := c.defaultWalkFunc(vertex)
	switch {
	case err == nil:
		c.transCtx.Logger.V(1).Info(fmt.Sprintf("reconcile object %T with action %s OK", node.Obj, *node.Action))
		return err
	case !ok:
		c.transCtx.Logger.Error(err, "")
	case node.Action == nil:
		c.transCtx.Logger.Error(err, fmt.Sprintf("%T", node))
	case apierrors.IsConflict(err):
		c.transCtx.Logger.V(1).Info(fmt.Sprintf("reconcile object %T with action %s error: %s", node.Obj, *node.Action, err.Error()))
		return err
	default:
		c.transCtx.Logger.Error(err, fmt.Sprintf("%s %T error", *node.Action, node.Obj))
	}
	return err
}

func (c *componentPlanBuilder) defaultWalkFunc(v graph.Vertex) error {
	vertex, ok := v.(*model.ObjectVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", v)
	}
	if vertex.Action == nil {
		return fmt.Errorf("vertex action can't be nil")
	}
	ctx := c.transCtx.Context
	switch *vertex.Action {
	case model.CREATE:
		return c.reconcileCreateObject(ctx, vertex)
	case model.UPDATE:
		return c.reconcileUpdateObject(ctx, vertex)
	case model.PATCH:
		return c.reconcilePatchObject(ctx, vertex)
	case model.DELETE:
		return c.reconcileDeleteObject(ctx, vertex)
	case model.STATUS:
		return c.reconcileStatusObject(ctx, vertex)
	}
	return nil
}

func (c *componentPlanBuilder) reconcileCreateObject(ctx context.Context, vertex *model.ObjectVertex) error {
	err := c.cli.Create(ctx, vertex.Obj, clientOption(vertex))
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (c *componentPlanBuilder) reconcileUpdateObject(ctx context.Context, vertex *model.ObjectVertex) error {
	err := c.cli.Update(ctx, vertex.Obj, clientOption(vertex))
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *componentPlanBuilder) reconcilePatchObject(ctx context.Context, vertex *model.ObjectVertex) error {
	patch := client.MergeFrom(vertex.OriObj)
	err := c.cli.Patch(ctx, vertex.Obj, patch, clientOption(vertex))
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *componentPlanBuilder) reconcileDeleteObject(ctx context.Context, vertex *model.ObjectVertex) error {
	if controllerutil.RemoveFinalizer(vertex.Obj, constant.DBClusterFinalizerName) {
		err := c.cli.Update(ctx, vertex.Obj, clientOption(vertex))
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	if !model.IsObjectDeleting(vertex.Obj) {
		err := c.cli.Delete(ctx, vertex.Obj, clientOption(vertex))
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *componentPlanBuilder) reconcileStatusObject(ctx context.Context, vertex *model.ObjectVertex) error {
	return c.cli.Status().Update(ctx, vertex.Obj, clientOption(vertex))
}
