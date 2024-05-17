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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentPostProvisionTransformer handles component postProvision lifecycle action.
type componentPostProvisionTransformer struct{}

var _ graph.Transformer = &componentPostProvisionTransformer{}

func (t *componentPostProvisionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	cluster := transCtx.Cluster
	compOrig := transCtx.ComponentOrig
	synthesizeComp := transCtx.SynthesizeComponent
	runningWorkload := transCtx.RunningWorkload

	if model.IsObjectDeleting(compOrig) {
		return nil
	}

	actionCtx, err := component.NewActionContext(cluster, comp, runningWorkload,
		synthesizeComp.LifecycleActions, synthesizeComp.ScriptTemplates, component.PostProvisionAction)
	if err != nil {
		return err
	}

	if err := component.ReconcileCompPostProvision(reqCtx.Ctx, transCtx.Client, graphCli, actionCtx, dag); err != nil {
		// When postProvision action's preCondition is Immediately or RuntimeReady, only the task success will be component status become Ready.
		if postProvisionPrematureStopCondition(synthesizeComp.LifecycleActions) {
			comp.Status.Phase = appsv1alpha1.CreatingClusterCompPhase
			graphCli.Status(dag, comp, transCtx.Component)
			reqCtx.Log.Info("Component postProvision prematurely stopped", "component", comp.Name, "err", err)
			return graph.ErrPrematureStop
		}
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeExpectedInProcess) {
			return nil
		}
		return err
	}
	return nil
}

// postProvisionPrematureStopCondition checks if the component postProvision action should stop prematurely.
func postProvisionPrematureStopCondition(lifecycleActions *appsv1alpha1.ComponentLifecycleActions) bool {
	if lifecycleActions == nil || lifecycleActions.PostProvision == nil ||
		lifecycleActions.PostProvision.CustomHandler == nil {
		return false
	}
	if component.IsImmediatelyOrRuntimeReadyPreCondition(lifecycleActions.PostProvision.CustomHandler) {
		return true
	}
	return false
}
