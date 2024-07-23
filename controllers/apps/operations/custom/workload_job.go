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

package custom

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

// createJob creates the job workload.
func (w *WorkloadAction) createJob(actionCtx ActionContext,
	podSpec *corev1.PodSpec,
	targetPodName string,
	taskIndex int) (*appsv1alpha1.ActionTask, error) {
	buildJobName := func() string {
		jobName := fmt.Sprintf("%s-%s-%s-%s", w.OpsRequest.UID[:8], common.CutString(w.OpsRequest.Name, 18),
			common.CutString(w.Comp.Name, 18), actionCtx.Action.Name)
		return fmt.Sprintf("%s-%d", common.CutString(jobName, 57), taskIndex)
	}
	job := builder.NewJobBuilder(w.OpsRequest.Namespace, buildJobName()).
		SetBackoffLimit(actionCtx.Action.Workload.BackoffLimit).
		AddLabelsInMap(buildLabels(w.OpsRequest.Name, actionCtx.Action.Name)).
		SetPodTemplateSpec(corev1.PodTemplateSpec{Spec: *podSpec}).
		GetObject()
	job.Kind = constant.JobKind
	return actionCtx.createActionK8sWorkload(w.OpsRequest, job, targetPodName)
}

// checkJobStatus checks if the job is finished and failed by status.
func (w *WorkloadAction) checkJobStatus(actionCtx ActionContext, task *appsv1alpha1.ActionTask, taskIndex int) (bool, bool, error) {
	var (
		completed    bool
		existFailure bool
		err          error
	)
	createIfNotExist := func() error {
		targetPodTemplate, targetPod, err := getTargetTemplateAndPod(actionCtx.ReqCtx.Ctx, actionCtx.Client,
			w.OpsDef, actionCtx.Action.Workload.PodInfoExtractorName, task.TargetPodName, w.OpsRequest.Namespace)
		if err != nil {
			return err
		}
		podSpec, err := w.buildPodSpec(actionCtx, targetPodTemplate, targetPod)
		if err != nil {
			return err
		}
		_, err = w.createJob(actionCtx, podSpec, task.TargetPodName, taskIndex)
		return err
	}
	switch task.Status {
	case appsv1alpha1.FailedActionTaskStatus:
		completed = true
		existFailure = true
	case appsv1alpha1.SucceedActionTaskStatus:
		completed = true
	default:
		job := &batchv1.Job{}
		err = actionCtx.Client.Get(actionCtx.ReqCtx.Ctx,
			client.ObjectKey{Name: getNameFromObjectKey(task.ObjectKey), Namespace: task.Namespace}, job)
		if err != nil {
			// if the job has been deleted, re-create it.
			if apierrors.IsNotFound(err) {
				return false, false, createIfNotExist()
			}
			return false, false, err
		}
		for _, c := range job.Status.Conditions {
			if c.Status != corev1.ConditionTrue {
				continue
			}
			if c.Type == batchv1.JobComplete {
				completed = true
				break
			}
			if c.Type == batchv1.JobFailed {
				completed = true
				existFailure = true
			}
		}
	}
	return completed, existFailure, nil
}
