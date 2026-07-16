/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
)

func (w *WorkloadAction) buildJob(actionCtx ActionContext,
	podSpec *corev1.PodSpec,
	taskIndex int) *batchv1.Job {
	buildJobName := func() string {
		componentName := w.Comp.Name
		if actionCtx.Action.Workload.Type == opsv1alpha1.ManagedJobWorkload {
			componentName = w.CustomCompOps.ComponentName
		}
		jobName := fmt.Sprintf("%s-%s-%s-%s", common.CutString(string(w.OpsRequest.UID), 8), common.CutString(w.OpsRequest.Name, 18),
			common.CutString(componentName, 18), actionCtx.Action.Name)
		return fmt.Sprintf("%s-%d", common.CutString(jobName, 57), taskIndex)
	}
	job := builder.NewJobBuilder(w.OpsRequest.Namespace, buildJobName()).
		SetBackoffLimit(actionCtx.Action.Workload.BackoffLimit).
		AddLabelsInMap(buildLabels(w.OpsRequest.Name, actionCtx.Action.Name)).
		SetPodTemplateSpec(corev1.PodTemplateSpec{Spec: *podSpec}).
		GetObject()
	job.APIVersion = batchv1.SchemeGroupVersion.String()
	job.Kind = constant.JobKind
	if actionCtx.Action.Workload.Type == opsv1alpha1.ManagedJobWorkload {
		selectorValue := opsutil.ManagedJobSelectorValue(string(w.OpsRequest.UID), taskIndex)
		job.Spec.ManualSelector = ptr.To(true)
		job.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{
			constant.ManagedJobSelectorLabelKey: selectorValue,
		}}
		if job.Spec.Template.Labels == nil {
			job.Spec.Template.Labels = map[string]string{}
		}
		job.Spec.Template.Labels[constant.ManagedJobSelectorLabelKey] = selectorValue
	}
	return job
}

// createJob creates the legacy Job workload.
func (w *WorkloadAction) createJob(actionCtx ActionContext,
	podSpec *corev1.PodSpec,
	targetPodName string,
	taskIndex int) (*opsv1alpha1.ActionTask, error) {
	job := w.buildJob(actionCtx, podSpec, taskIndex)
	return actionCtx.createActionK8sWorkload(w.OpsRequest, job, targetPodName)
}

func (w *WorkloadAction) buildExpectedManagedJob(actionCtx ActionContext, taskIndex int) (*batchv1.Job, error) {
	podSpec, err := w.buildPodSpec(actionCtx, nil, nil)
	if err != nil {
		return nil, err
	}
	job := w.buildJob(actionCtx, podSpec, taskIndex)
	if err := actionCtx.prepareActionK8sWorkload(w.OpsRequest, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (w *WorkloadAction) defaultManagedJob(actionCtx ActionContext, job *batchv1.Job) (*batchv1.Job, error) {
	defaulted := job.DeepCopy()
	if err := actionCtx.Client.Create(actionCtx.ReqCtx.Ctx, defaulted, client.DryRunAll); err != nil {
		return nil, err
	}
	return defaulted, nil
}

func managedJobSpecHash(job *batchv1.Job) (string, error) {
	return opsutil.ManagedJobSpecHash(job.Spec)
}

func (w *WorkloadAction) planManagedJob(actionCtx ActionContext) (*opsv1alpha1.ActionTask, error) {
	var taskIndex int32
	job, err := w.buildExpectedManagedJob(actionCtx, int(taskIndex))
	if err != nil {
		return nil, err
	}
	live := &batchv1.Job{}
	err = actionCtx.directReader().Get(actionCtx.ReqCtx.Ctx, client.ObjectKeyFromObject(job), live)
	if err == nil {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("managed Job %s/%s exists before its execution plan was persisted", job.Namespace, job.Name))
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	defaulted, err := w.defaultManagedJob(actionCtx, job)
	if err != nil {
		return nil, err
	}
	hash, err := managedJobSpecHash(defaulted)
	if err != nil {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("failed to hash managed Job %s/%s: %v", job.Namespace, job.Name, err))
	}
	return &opsv1alpha1.ActionTask{
		ObjectKey:        fmt.Sprintf("%s/%s", constant.JobKind, job.Name),
		Namespace:        job.Namespace,
		Status:           opsv1alpha1.ProcessingActionTaskStatus,
		TaskIndex:        &taskIndex,
		DispatchState:    opsv1alpha1.PlannedActionTaskDispatchState,
		WorkloadSpecHash: hash,
	}, nil
}

func (w *WorkloadAction) validateManagedJob(task *opsv1alpha1.ActionTask, job *batchv1.Job, expectedName string) error {
	if job.Name != expectedName || job.Namespace != task.Namespace {
		return intctrlutil.NewFatalError(fmt.Sprintf("managed Job identity changed from %s/%s to %s/%s",
			task.Namespace, expectedName, job.Namespace, job.Name))
	}
	if !job.DeletionTimestamp.IsZero() {
		return intctrlutil.NewFatalError(fmt.Sprintf("managed Job %s/%s is terminating", job.Namespace, job.Name))
	}
	owner := metav1.GetControllerOf(job)
	if owner == nil || owner.APIVersion != opsv1alpha1.GroupVersion.String() || owner.Kind != "OpsRequest" ||
		owner.Name != w.OpsRequest.Name || owner.UID != w.OpsRequest.UID {
		return intctrlutil.NewFatalError(fmt.Sprintf("managed Job %s/%s is not controlled by the exact OpsRequest UID %s",
			job.Namespace, job.Name, w.OpsRequest.UID))
	}
	if job.UID == "" {
		return intctrlutil.NewFatalError(fmt.Sprintf("managed Job %s/%s has no UID", job.Namespace, job.Name))
	}
	hash, err := managedJobSpecHash(job)
	if err != nil {
		return intctrlutil.NewFatalError(fmt.Sprintf("failed to hash managed Job %s/%s: %v", job.Namespace, job.Name, err))
	}
	if hash != task.WorkloadSpecHash {
		return intctrlutil.NewFatalError(fmt.Sprintf("managed Job %s/%s spec no longer matches the persisted execution plan", job.Namespace, job.Name))
	}
	if task.WorkloadUID != "" && string(job.UID) != task.WorkloadUID {
		return intctrlutil.NewFatalError(fmt.Sprintf("managed Job %s/%s UID changed from %s to %s",
			job.Namespace, job.Name, task.WorkloadUID, job.UID))
	}
	return nil
}

func (w *WorkloadAction) dispatchManagedJob(actionCtx ActionContext, task *opsv1alpha1.ActionTask, taskIndex int) error {
	if task.TaskIndex == nil || *task.TaskIndex != int32(taskIndex) || task.WorkloadSpecHash == "" {
		return intctrlutil.NewFatalError("managed Job task is missing its persisted index or spec hash")
	}
	expected, err := w.buildExpectedManagedJob(actionCtx, taskIndex)
	if err != nil {
		return err
	}
	if task.ObjectKey != fmt.Sprintf("%s/%s", constant.JobKind, expected.Name) || task.Namespace != expected.Namespace {
		return intctrlutil.NewFatalError("managed Job task identity no longer matches the deterministic Job name")
	}
	live := &batchv1.Job{}
	key := client.ObjectKeyFromObject(expected)
	err = actionCtx.directReader().Get(actionCtx.ReqCtx.Ctx, key, live)
	if err == nil {
		if err := w.validateManagedJob(task, live, expected.Name); err != nil {
			return err
		}
		task.DispatchState = opsv1alpha1.CreatedActionTaskDispatchState
		task.WorkloadUID = string(live.UID)
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	defaulted, err := w.defaultManagedJob(actionCtx, expected)
	if err != nil {
		return err
	}
	hash, err := managedJobSpecHash(defaulted)
	if err != nil {
		return intctrlutil.NewFatalError(fmt.Sprintf("failed to hash managed Job %s/%s: %v", expected.Namespace, expected.Name, err))
	}
	if hash != task.WorkloadSpecHash {
		return intctrlutil.NewFatalError(fmt.Sprintf("managed Job %s/%s inputs changed after the execution plan was persisted", expected.Namespace, expected.Name))
	}
	if err := actionCtx.Client.Create(actionCtx.ReqCtx.Ctx, expected); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	live = &batchv1.Job{}
	if err := actionCtx.directReader().Get(actionCtx.ReqCtx.Ctx, key, live); err != nil {
		return err
	}
	if err := w.validateManagedJob(task, live, expected.Name); err != nil {
		return err
	}
	task.DispatchState = opsv1alpha1.CreatedActionTaskDispatchState
	task.WorkloadUID = string(live.UID)
	return nil
}

func (w *WorkloadAction) checkManagedJobStatus(actionCtx ActionContext,
	task *opsv1alpha1.ActionTask, taskIndex int) (bool, bool, error) {
	if task.DispatchState == opsv1alpha1.PlannedActionTaskDispatchState {
		if task.Status != opsv1alpha1.ProcessingActionTaskStatus {
			return false, false, intctrlutil.NewFatalError("planned managed Job task has a terminal status before dispatch")
		}
		if err := w.dispatchManagedJob(actionCtx, task, taskIndex); err != nil {
			return false, false, err
		}
		return false, false, nil
	}
	if task.DispatchState != opsv1alpha1.CreatedActionTaskDispatchState || task.WorkloadUID == "" {
		return false, false, intctrlutil.NewFatalError("managed Job task is not bound to an exact live UID")
	}
	job := &batchv1.Job{}
	name := getNameFromObjectKey(task.ObjectKey)
	if err := actionCtx.directReader().Get(actionCtx.ReqCtx.Ctx,
		client.ObjectKey{Name: name, Namespace: task.Namespace}, job); err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, intctrlutil.NewFatalError(fmt.Sprintf("managed Job %s/%s disappeared after dispatch; automatic recreation is forbidden", task.Namespace, name))
		}
		return false, false, err
	}
	if err := w.validateManagedJob(task, job, name); err != nil {
		return false, false, err
	}
	complete := false
	failed := false
	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case batchv1.JobComplete:
			complete = true
		case batchv1.JobFailed:
			failed = true
		}
	}
	switch task.Status {
	case opsv1alpha1.FailedActionTaskStatus:
		if !failed {
			return false, false, intctrlutil.NewFatalError("persisted managed Job failure no longer matches the exact live Job")
		}
		return true, true, nil
	case opsv1alpha1.SucceedActionTaskStatus:
		if failed || !complete {
			return false, false, intctrlutil.NewFatalError("persisted managed Job success no longer matches the exact live Job")
		}
		return true, false, nil
	case opsv1alpha1.ProcessingActionTaskStatus:
		if failed {
			return true, true, nil
		}
		return complete, false, nil
	default:
		return false, false, intctrlutil.NewFatalError("managed Job task has an unsupported status")
	}
}

// checkJobStatus checks if the job is finished and failed by status.
func (w *WorkloadAction) checkJobStatus(actionCtx ActionContext, task *opsv1alpha1.ActionTask, taskIndex int) (bool, bool, error) {
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
	case opsv1alpha1.FailedActionTaskStatus:
		completed = true
		existFailure = true
	case opsv1alpha1.SucceedActionTaskStatus:
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
