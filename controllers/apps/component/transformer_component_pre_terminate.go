/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	kbCompPreTerminateDoneKey = "kubeblocks.io/pre-terminate-done"
)

type componentPreTerminateTransformer struct{}

var _ graph.Transformer = &componentPreTerminateTransformer{}

func (t *componentPreTerminateTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	comp := transCtx.Component
	if comp.GetDeletionTimestamp().IsZero() || comp.Status.Phase != appsv1.DeletingComponentPhase {
		return nil
	}

	if len(comp.Spec.CompDef) == 0 {
		return nil
	}
	compDefKey := types.NamespacedName{
		Name: comp.Spec.CompDef,
	}
	compDef := &appsv1.ComponentDefinition{}
	if err := transCtx.Client.Get(transCtx.Context, compDefKey, compDef); err != nil {
		return err
	}
	if compDef.Spec.LifecycleActions == nil || compDef.Spec.LifecycleActions.PreTerminate == nil {
		return nil
	}

	if t.skipPreTerminate(transCtx) {
		return nil
	}

	provisioned, err := t.provisioned(transCtx)
	if err != nil {
		return err
	}
	if !provisioned {
		return nil
	}

	if t.checkPreTerminateDone(transCtx, dag) {
		return nil
	}
	if err = t.preTerminate(transCtx, compDef); err != nil {
		return lifecycle.IgnoreNotDefined(err)
	}
	return t.markPreTerminateDone(transCtx, dag)
}

func (t *componentPreTerminateTransformer) skipPreTerminate(transCtx *componentTransformContext) bool {
	comp := transCtx.Component
	if comp.Annotations == nil {
		return false
	}
	skip, ok := comp.Annotations[constant.SkipPreTerminateAnnotationKey]
	return ok && strings.ToLower(skip) == "true"
}

func (t *componentPreTerminateTransformer) provisioned(transCtx *componentTransformContext) (bool, error) {
	its := &workloads.InstanceSet{}
	itsKey := types.NamespacedName{
		Namespace: transCtx.Component.Namespace,
		Name:      transCtx.Component.Name,
	}
	if err := transCtx.Client.Get(transCtx.Context, itsKey, its); err != nil {
		return false, client.IgnoreNotFound(err)
	}

	for _, inst := range its.Status.InstanceStatus {
		if inst.Provisioned {
			return true, nil
		}
	}
	return false, nil
}

func (t *componentPreTerminateTransformer) checkPreTerminateDone(transCtx *componentTransformContext, dag *graph.DAG) bool {
	comp := transCtx.Component
	if comp.Annotations == nil {
		return false
	}
	// TODO: condition
	_, ok := comp.Annotations[kbCompPreTerminateDoneKey]
	return ok
}

func (t *componentPreTerminateTransformer) markPreTerminateDone(transCtx *componentTransformContext, dag *graph.DAG) error {
	comp := transCtx.Component
	if comp.Annotations == nil {
		comp.Annotations = make(map[string]string)
	}
	_, ok := comp.Annotations[kbCompPreTerminateDoneKey]
	if ok {
		return nil
	}
	compObj := comp.DeepCopy()
	timeStr := time.Now().Format(time.RFC3339Nano)
	comp.Annotations[kbCompPreTerminateDoneKey] = timeStr

	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Update(dag, compObj, comp, &model.ReplaceIfExistingOption{})
	return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for pre-terminate annotation to be set")
}

func (t *componentPreTerminateTransformer) preTerminate(transCtx *componentTransformContext, compDef *appsv1.ComponentDefinition) error {
	lfa, err := t.lifecycleAction4Component(transCtx, compDef)
	if err != nil {
		return err
	}
	return lfa.PreTerminate(transCtx.Context, transCtx.Client, nil)
}

func (t *componentPreTerminateTransformer) lifecycleAction4Component(transCtx *componentTransformContext, compDef *appsv1.ComponentDefinition) (lifecycle.Lifecycle, error) {
	synthesizedComp, err1 := t.synthesizedComponent(transCtx, compDef)
	if err1 != nil {
		return nil, err1
	}
	pods, err2 := component.ListOwnedPods(transCtx.Context, transCtx.Client,
		synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
	if err2 != nil {
		return nil, err2
	}
	if len(pods) == 0 {
		// TODO: (good-first-issue) we should handle the case that the component has no pods
		return nil, fmt.Errorf("has no pods to running the pre-terminate action")
	}
	return lifecycle.New(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name,
		synthesizedComp.LifecycleActions, synthesizedComp.TemplateVars, nil, pods...)
}

func (t *componentPreTerminateTransformer) synthesizedComponent(transCtx *componentTransformContext, compDef *appsv1.ComponentDefinition) (*component.SynthesizedComponent, error) {
	var (
		ctx  = transCtx.Context
		cli  = transCtx.Client
		comp = transCtx.Component
	)
	synthesizedComp, err := component.BuildSynthesizedComponent(ctx, cli, compDef, comp)
	if err != nil {
		return nil, intctrlutil.NewRequeueError(appsutil.RequeueDuration,
			fmt.Sprintf("build synthesized component failed at pre-terminate: %s", err.Error()))
	}
	synthesizedComp.TemplateVars, _, err = component.ResolveTemplateNEnvVars(ctx, cli, synthesizedComp, compDef.Spec.Vars)
	if err != nil {
		return nil, err
	}
	return synthesizedComp, nil
}
