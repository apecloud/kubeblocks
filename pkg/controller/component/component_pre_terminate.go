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
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	dag *graph.DAG) error {
	ctx := reqCtx.Ctx

	// TODO(xingran): check if preTerminate action is needed for the component when cluster id deleting
	//if !cluster.DeletionTimestamp.IsZero() {
	//	reqCtx.Log.Info("cluster is deleting, skip reconciling component preTerminate", "cluster", cluster.Name)
	//	return nil
	//}

	compKey := types.NamespacedName{
		Namespace: comp.Namespace,
		Name:      comp.Spec.CompDef,
	}
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := cli.Get(ctx, compKey, compDef); err != nil {
		return err
	}

	synthesizedComp, err := BuildSynthesizedComponent(reqCtx, cli, cluster, compDef, comp)
	if err != nil {
		return err
	}

	return reconcileCompPreTerminate(ctx, cli, cluster, comp, synthesizedComp, dag)
}

func reconcileCompPreTerminate(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	synthesizedComp *SynthesizedComponent,
	dag *graph.DAG) error {
	needPreTerminate, err := needDoPreTerminate(ctx, cli, cluster, comp, synthesizedComp)
	if err != nil {
		return err
	}
	if !needPreTerminate {
		return nil
	}

	job, err := createActionJobIfNotExist(ctx, cli, cluster, comp, synthesizedComp, PreTerminateAction)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}

	err = CheckJobSucceed(ctx, cli, cluster, job.Name)
	if err != nil {
		return err
	}

	// job executed successfully, add the annotation to indicate that the PreTerminate has been executed and delete the job
	compOrig := comp.DeepCopy()
	if err := setActionDoneAnnotation(cli, comp, dag, PreTerminateAction); err != nil {
		return err
	}

	if err := cleanActionJob(ctx, cli, cluster, *compOrig, *synthesizedComp, PreTerminateAction, job.Name); err != nil {
		return err
	}

	return nil
}

func needDoPreTerminate(ctx context.Context, cli client.Client,
	cluster *appsv1alpha1.Cluster, comp *appsv1alpha1.Component, synthesizeComp *SynthesizedComponent) (bool, error) {
	// if the component does not have a custom PreTerminate action, skip it
	actionExist, _ := checkLifeCycleAction(synthesizeComp, PreTerminateAction)
	if !actionExist {
		return false, nil
	}

	return needDoActionByCheckingJobNAnnotation(ctx, cli, cluster, comp, synthesizeComp, PreTerminateAction)
}