/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"time"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
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
	if comp.GetDeletionTimestamp().IsZero() || comp.Status.Phase != appsv1alpha1.DeletingClusterCompPhase {
		return nil
	}

	synthesizedComp := transCtx.SynthesizeComponent
	if synthesizedComp == nil || synthesizedComp.LifecycleActions == nil || synthesizedComp.LifecycleActions.PreTerminate == nil {
		return nil
	}

	// TODO: force skip the pre-terminate action?

	if t.checkPreTerminateDone(transCtx, dag) {
		return nil
	}
	err := t.preTerminate(transCtx)
	if err != nil {
		return lifecycle.IgnoreNotDefined(err)
	}
	return t.markPreTerminateDone(transCtx, dag)
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

func (t *componentPreTerminateTransformer) preTerminate(transCtx *componentTransformContext) error {
	lfa, err := lifecycleAction4Component(transCtx)
	if err != nil {
		return err
	}
	return lfa.PreTerminate(transCtx.Context, transCtx.Client, nil)
}
