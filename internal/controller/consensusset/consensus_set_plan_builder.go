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

package consensusset

import (
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type csSetPlanBuilder struct {
	req          ctrl.Request
	cli          client.Client
	transCtx     *CSSetTransformContext
	transformers graph.TransformerChain
}

type csSetPlan struct {
	dag      *graph.DAG
	walkFunc graph.WalkFunc
	cli      client.Client
	transCtx *CSSetTransformContext
}

func init() {
	model.AddScheme(workloads.AddToScheme)
}

// PlanBuilder implementation

func (b *csSetPlanBuilder) Init() error {
	csSet := &workloads.ConsensusSet{}
	if err := b.cli.Get(b.transCtx.Context, b.req.NamespacedName, csSet); err != nil {
		return err
	}
	b.AddTransformer(&initTransformer{ConsensusSet: csSet})
	return nil
}

func (b *csSetPlanBuilder) AddTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	b.transformers = append(b.transformers, transformer...)
	return b
}

func (b *csSetPlanBuilder) AddParallelTransformer(transformer ...graph.Transformer) graph.PlanBuilder {
	b.transformers = append(b.transformers, &model.ParallelTransformer{Transformers: transformer})
	return b
}

func (b *csSetPlanBuilder) Build() (graph.Plan, error) {
	var err error
	// new a DAG and apply chain on it, after that we should get the final Plan
	dag := graph.NewDAG()
	err = b.transformers.ApplyTo(b.transCtx, dag)
	// log for debug
	b.transCtx.Logger.Info(fmt.Sprintf("DAG: %s", dag))

	// we got the execution plan
	plan := &csSetPlan{
		dag:      dag,
		walkFunc: b.csSetWalkFunc,
		cli:      b.cli,
		transCtx: b.transCtx,
	}
	return plan, err
}

// Plan implementation

func (p *csSetPlan) Execute() error {
	return p.dag.WalkReverseTopoOrder(p.walkFunc)
}

// Do the real works

func (b *csSetPlanBuilder) csSetWalkFunc(v graph.Vertex) error {
	vertex, ok := v.(*model.ObjectVertex)
	if !ok {
		return fmt.Errorf("wrong vertex type %v", v)
	}
	if vertex.Action == nil {
		return errors.New("node action can't be nil")
	}
	if vertex.Immutable {
		return nil
	}
	switch *vertex.Action {
	case model.CREATE:
		err := b.cli.Create(b.transCtx.Context, vertex.Obj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	case model.UPDATE:
		if vertex.Immutable {
			return nil
		}
		o, err := b.buildUpdateObj(vertex)
		if err != nil {
			return err
		}
		err = b.cli.Update(b.transCtx.Context, o)
		if err != nil && !apierrors.IsNotFound(err) {
			b.transCtx.Logger.Error(err, fmt.Sprintf("update %T error: %s", o, vertex.OriObj.GetName()))
			return err
		}
	case model.DELETE:
		if controllerutil.RemoveFinalizer(vertex.Obj, CSSetFinalizerName) {
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
		patch := client.MergeFrom(vertex.OriObj)
		if err := b.cli.Status().Patch(b.transCtx.Context, vertex.Obj, patch); err != nil {
			return err
		}
	}
	return nil
}

func (b *csSetPlanBuilder) buildUpdateObj(vertex *model.ObjectVertex) (client.Object, error) {
	handleSts := func(origObj, targetObj *appsv1.StatefulSet) (client.Object, error) {
		origObj.Spec.Template = targetObj.Spec.Template
		origObj.Spec.Replicas = targetObj.Spec.Replicas
		origObj.Spec.UpdateStrategy = targetObj.Spec.UpdateStrategy
		return origObj, nil
	}

	handleDeploy := func(origObj, targetObj *appsv1.Deployment) (client.Object, error) {
		origObj.Spec = targetObj.Spec
		return origObj, nil
	}

	handleSvc := func(origObj, targetObj *corev1.Service) (client.Object, error) {
		origObj.Spec = targetObj.Spec
		return origObj, nil
	}

	handlePVC := func(origObj, targetObj *corev1.PersistentVolumeClaim) (client.Object, error) {
		if origObj.Spec.Resources.Requests[corev1.ResourceStorage] == targetObj.Spec.Resources.Requests[corev1.ResourceStorage] {
			return origObj, nil
		}
		origObj.Spec.Resources.Requests[corev1.ResourceStorage] = targetObj.Spec.Resources.Requests[corev1.ResourceStorage]
		return origObj, nil
	}

	origObj := vertex.OriObj.DeepCopyObject()
	switch v := vertex.Obj.(type) {
	case *appsv1.StatefulSet:
		return handleSts(origObj.(*appsv1.StatefulSet), v)
	case *appsv1.Deployment:
		return handleDeploy(origObj.(*appsv1.Deployment), v)
	case *corev1.Service:
		return handleSvc(origObj.(*corev1.Service), v)
	case *corev1.PersistentVolumeClaim:
		return handlePVC(origObj.(*corev1.PersistentVolumeClaim), v)
	case *corev1.Secret, *corev1.ConfigMap:
		return v, nil
	}

	return vertex.Obj, nil
}

// NewCSSetPlanBuilder returns a csSetPlanBuilder powered PlanBuilder
func NewCSSetPlanBuilder(ctx intctrlutil.RequestCtx, cli client.Client, req ctrl.Request) graph.PlanBuilder {
	return &csSetPlanBuilder{
		req: req,
		cli: cli,
		transCtx: &CSSetTransformContext{
			Context:       ctx.Ctx,
			Client:        cli,
			EventRecorder: ctx.Recorder,
			Logger:        ctx.Log,
		},
	}
}

var _ graph.PlanBuilder = &csSetPlanBuilder{}
var _ graph.Plan = &csSetPlan{}
