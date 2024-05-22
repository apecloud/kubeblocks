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

package component

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// pre-terminate constants
const (
	kbPreTerminateJobLabelKey   = "kubeblocks.io/pre-terminate-job"
	kbPreTerminateJobLabelValue = "kb-pre-terminate-job"
	kbPreTerminateJobNamePrefix = "kb-pre-terminate-job"

	// kbCompPreTerminateDoneKey is used to mark the component PreTerminate job is done
	kbCompPreTerminateDoneKey = "kubeblocks.io/pre-terminate-done"
)

// ReconcileCompPreTerminate reconciles the component-level preTerminate command.
func ReconcileCompPreTerminate(reqCtx intctrlutil.RequestCtx,
	cli client.Reader,
	graphCli model.GraphClient,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	dag *graph.DAG) error {
	ctx := reqCtx.Ctx
	if comp == nil || len(comp.Spec.CompDef) == 0 {
		reqCtx.Log.Info("comp is nil or compDef is empty, skip reconciling component preTerminate")
		return nil
	}
	compKey := types.NamespacedName{
		Namespace: comp.Namespace,
		Name:      comp.Spec.CompDef,
	}
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := cli.Get(ctx, compKey, compDef); err != nil {
		reqCtx.Log.Error(err, "reconcile pre terminate failed to get compDef", "comp", comp.Name, "compDef", compKey)
		return err
	}

	actionCtx, err := NewActionContext(cluster, comp, nil, compDef.Spec.LifecycleActions, compDef.Spec.Scripts, PreTerminateAction)
	if err != nil {
		return err
	}

	return reconcileCompPreTerminate(ctx, cli, graphCli, actionCtx, dag)
}

func reconcileCompPreTerminate(ctx context.Context,
	cli client.Reader,
	graphCli model.GraphClient,
	actionCtx *ActionContext,
	dag *graph.DAG) error {
	needPreTerminate, err := needDoPreTerminate(ctx, cli, actionCtx)
	if err != nil {
		return err
	}
	if !needPreTerminate {
		return nil
	}

	actionJob, err := createActionJobIfNotExist(ctx, cli, graphCli, dag, actionCtx)
	if err != nil {
		return err
	}
	if actionJob == nil {
		return nil
	}

	err = CheckJobSucceed(ctx, cli, actionCtx.cluster, actionJob.Name)
	if err != nil {
		return err
	}

	// job executed successfully, add the annotation to indicate that the PreTerminate has been executed and delete the job
	if err := setActionDoneAnnotation(graphCli, actionCtx, dag); err != nil {
		return err
	}

	if err := cleanActionJob(ctx, cli, dag, actionCtx, actionJob.Name); err != nil {
		return err
	}

	return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for job %s to be cleaned.", actionJob.Name)
}

func needDoPreTerminate(ctx context.Context, cli client.Reader, actionCtx *ActionContext) (bool, error) {
	// if the component does not have a custom PreTerminate action, skip it
	actionExist, _ := checkLifeCycleAction(actionCtx)
	if !actionExist {
		return false, nil
	}

	return needDoActionByCheckingJobNAnnotation(ctx, cli, actionCtx)
}
