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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

type OpsAction interface {
	// Execute executes the action.
	Execute(actionCtx ActionContext) (*ActionStatus, error)

	// CheckStatus checks the action status.
	CheckStatus(actionCtx ActionContext) (*ActionStatus, error)
}

type ActionStatus struct {
	IsCompleted  bool
	ExistFailure bool
	// return the action tasks(required).
	ActionTasks []opsv1alpha1.ActionTask
}

func NewActiontatus() *ActionStatus {
	return &ActionStatus{
		ActionTasks: []opsv1alpha1.ActionTask{},
	}
}

type ActionContext struct {
	ReqCtx intctrlutil.RequestCtx
	Client client.Client
	Action *opsv1alpha1.OpsAction
}

func (actionCtx ActionContext) createActionK8sWorkload(
	opsRequest *opsv1alpha1.OpsRequest,
	workload client.Object,
	targetPodName string) (*opsv1alpha1.ActionTask, error) {
	if workload.GetNamespace() == opsRequest.Namespace {
		scheme, _ := opsv1alpha1.SchemeBuilder.Build()
		if err := utils.SetControllerReference(opsRequest, workload, scheme); err != nil {
			return nil, err
		}
	}
	objectKey := fmt.Sprintf("%s/%s", workload.GetObjectKind().GroupVersionKind().Kind, workload.GetName())
	if err := actionCtx.Client.Create(actionCtx.ReqCtx.Ctx, workload); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}
	return &opsv1alpha1.ActionTask{
		Namespace:     workload.GetNamespace(),
		ObjectKey:     objectKey,
		TargetPodName: targetPodName,
		Status:        opsv1alpha1.ProcessingActionTaskStatus,
	}, nil
}

func (actionCtx ActionContext) checkActionStatus(progressDetail opsv1alpha1.ProgressStatusDetail,
	checkTaskStatus func(ActionContext, *opsv1alpha1.ActionTask, int) (bool, bool, error)) (*ActionStatus, error) {
	var (
		tasks          = progressDetail.ActionTasks
		completedCount int
		existFailed    bool
	)
	for i := range tasks {
		completed, failed, err := checkTaskStatus(actionCtx, &tasks[i], i)
		if err != nil {
			return nil, err
		}
		if !completed {
			continue
		}
		// sync the task status.
		completedCount += 1
		if failed {
			existFailed = true
			tasks[i].Status = opsv1alpha1.FailedActionTaskStatus
		} else {
			tasks[i].Status = opsv1alpha1.SucceedActionTaskStatus
		}
	}
	return &ActionStatus{
		ActionTasks:  tasks,
		IsCompleted:  len(tasks) == completedCount,
		ExistFailure: existFailed,
	}, nil
}

func (actionCtx ActionContext) checkPodTaskStatus(
	task *opsv1alpha1.ActionTask,
	backOffLimit int32,
	createPod func() error) (bool, bool, error) {
	var (
		completed    bool
		existFailure bool
		err          error
	)
	switch task.Status {
	case opsv1alpha1.FailedActionTaskStatus:
		completed = true
		existFailure = true
	case opsv1alpha1.SucceedActionTaskStatus:
		completed = true
	default:
		pod := &corev1.Pod{}
		err = actionCtx.Client.Get(actionCtx.ReqCtx.Ctx,
			client.ObjectKey{Name: getNameFromObjectKey(task.ObjectKey), Namespace: task.Namespace}, pod)
		if err != nil {
			// if the pod has been deleted during running task, re-create it.
			if apierrors.IsNotFound(err) {
				return false, false, createPod()
			}
			return false, false, err
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			completed = true
		case corev1.PodFailed:
			if task.Retries < backOffLimit {
				task.Retries += 1
				return false, false, createPod()
			} else {
				completed = true
				existFailure = true
			}
		}
	}
	return completed, existFailure, nil
}
