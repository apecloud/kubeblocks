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
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
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
	actionCtx *ActionContext,
	dag *graph.DAG) error {
	needPostProvision, err := NeedDoPostProvision(ctx, cli, actionCtx)
	if err != nil {
		return err
	}
	if !needPostProvision {
		return nil
	}

	actionJob, err := createActionJobIfNotExist(ctx, cli, graphCli, dag, actionCtx)
	if err != nil {
		return err
	}
	if actionJob == nil {
		return nil
	}

	err = job.CheckJobSucceed(ctx, cli, actionCtx.cluster, actionJob.Name)
	if err != nil {
		return err
	}

	// job executed successfully, add the annotation to indicate that the postProvision has been executed and delete the job
	if err := setActionDoneAnnotation(graphCli, actionCtx, dag); err != nil {
		return err
	}

	if err := cleanActionJob(ctx, cli, dag, actionCtx, actionJob.Name); err != nil {
		return err
	}

	return nil
}

func NeedDoPostProvision(ctx context.Context, cli client.Reader, actionCtx *ActionContext) (bool, error) {
	// if the component does not have a custom postProvision, skip it
	actionExist, _ := checkLifeCycleAction(actionCtx)
	if !actionExist {
		return false, nil
	}

	actionPreCondition := actionCtx.lifecycleActions.PostProvision.PreCondition
	if actionPreCondition != nil {
		switch *actionPreCondition {
		case appsv1alpha1.ImmediatelyPreConditionType:
			return needDoActionByCheckingJobNAnnotation(ctx, cli, actionCtx)
		case appsv1alpha1.RuntimeReadyPreConditionType:
			if actionCtx.workload == nil {
				return false, intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "runtime is nil when checking RuntimeReady preCondition in postProvision action")
			}
			runningITS, _ := actionCtx.workload.(*workloads.InstanceSet)
			if !instanceset.IsInstancesReady(runningITS) {
				return false, intctrlutil.NewErrorf(intctrlutil.ErrorTypeExpectedInProcess, "runtime is not ready when checking RuntimeReady preCondition in postProvision action")
			}
		case appsv1alpha1.ComponentReadyPreConditionType:
			if actionCtx.component.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
				return false, nil
			}
		case appsv1alpha1.ClusterReadyPreConditionType:
			if actionCtx.cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
				return false, nil
			}
		default:
			return false, errors.New("unsupported postProvision preCondition type")
		}
	} else if actionCtx.component.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
		// if the PreCondition is not set, the default preCondition is ComponentReady
		return false, nil
	}

	return needDoActionByCheckingJobNAnnotation(ctx, cli, actionCtx)
}
