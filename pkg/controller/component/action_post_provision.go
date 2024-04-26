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

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/job"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// post-provision constants
const (
	kbPostProvisionJobLabelKey   = "kubeblocks.io/post-provision-job"
	kbPostProvisionJobLabelValue = "kb-post-provision-job"
	kbPostProvisionJobNamePrefix = "kb-post-provision-job"

	// kbCompPostStartDoneKeyPattern will be deprecated after KubeBlocks v0.8.0 and use kbCompPostProvisionDoneKey instead
	kbCompPostStartDoneKeyPattern = "kubeblocks.io/%s-poststart-done"
	// kbCompPostProvisionDoneKey is used to mark the component postProvision job is done
	kbCompPostProvisionDoneKey = "kubeblocks.io/post-provision-done"
)

// ReconcileCompPostProvision reconciles the component-level postProvision command.
func ReconcileCompPostProvision(ctx context.Context,
	cli client.Reader,
	graphCli model.GraphClient,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	synthesizeComp *SynthesizedComponent,
	dag *graph.DAG) error {
	needPostProvision, err := NeedDoPostProvision(ctx, cli, cluster, comp, synthesizeComp)
	if err != nil {
		return err
	}
	if !needPostProvision {
		return nil
	}

	actionJob, err := createActionJobIfNotExist(ctx, cli, graphCli, dag, cluster, comp, synthesizeComp, PostProvisionAction)
	if err != nil {
		return err
	}
	if actionJob == nil {
		return nil
	}

	err = job.CheckJobSucceed(ctx, cli, cluster, actionJob.Name)
	if err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeExpectedInProcess) {
			return nil
		}
		return err
	}

	// job executed successfully, add the annotation to indicate that the postProvision has been executed and delete the job
	compOrig := comp.DeepCopy()
	if err := setActionDoneAnnotation(graphCli, comp, dag, PostProvisionAction); err != nil {
		return err
	}

	if err := cleanActionJob(ctx, cli, dag, cluster, *compOrig, *synthesizeComp, PostProvisionAction, actionJob.Name); err != nil {
		return err
	}

	return nil
}

func NeedDoPostProvision(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster, comp *appsv1alpha1.Component, synthesizeComp *SynthesizedComponent) (bool, error) {
	// if the component does not have a custom postProvision, skip it
	actionExist, _ := checkLifeCycleAction(synthesizeComp, PostProvisionAction)
	if !actionExist {
		return false, nil
	}

	// TODO(xingran): The PostProvision handling for the ComponentReady & ClusterReady condition has been implemented. The PostProvision for other conditions is currently pending implementation.
	actionPreCondition := synthesizeComp.LifecycleActions.PostProvision.CustomHandler.PreCondition
	if actionPreCondition != nil {
		switch *actionPreCondition {
		case appsv1alpha1.ComponentReadyPreConditionType:
			if comp.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
				return false, nil
			}
		case appsv1alpha1.ClusterReadyPreConditionType:
			if cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
				return false, nil
			}
		default:
			return false, nil
		}
	} else if comp.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
		// if the PreCondition is not set, the default preCondition is ComponentReady
		return false, nil
	}

	return needDoActionByCheckingJobNAnnotation(ctx, cli, cluster, comp, synthesizeComp, PostProvisionAction)
}
