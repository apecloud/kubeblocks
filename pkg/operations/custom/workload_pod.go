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
	corev1 "k8s.io/api/core/v1"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

// createJob creates the job workload.
func (w *WorkloadAction) createPod(actionCtx ActionContext,
	podSpec *corev1.PodSpec,
	targetPodName string,
	targetPodIndex,
	retries int) (*opsv1alpha1.ActionTask, error) {
	podName := buildActionPodName(w.OpsRequest, w.Comp.Name, actionCtx.Action.Name, targetPodIndex, retries)
	pod := builder.NewPodBuilder(w.OpsRequest.Namespace, podName).
		AddLabelsInMap(buildLabels(w.OpsRequest.Name, actionCtx.Action.Name)).
		AddLabels(constant.OpsRequestNamespaceLabelKey, w.OpsRequest.Namespace).
		SetPodSpec(*podSpec).
		SetRestartPolicy(corev1.RestartPolicyNever).
		GetObject()
	pod.Kind = constant.PodKind
	return actionCtx.createActionK8sWorkload(w.OpsRequest, pod, targetPodName)
}

// checkPodStatus checks if the pod is finished and failed by status.
func (w *WorkloadAction) checkPodStatus(actionCtx ActionContext,
	task *opsv1alpha1.ActionTask,
	taskIndex int) (bool, bool, error) {
	return actionCtx.checkPodTaskStatus(task, actionCtx.Action.Workload.BackoffLimit, func() error {
		targetPodTemplate, targetPod, err := getTargetTemplateAndPod(actionCtx.ReqCtx.Ctx, actionCtx.Client,
			w.OpsDef, actionCtx.Action.Workload.PodInfoExtractorName, task.TargetPodName, w.OpsRequest.Namespace)
		if err != nil {
			return err
		}
		podSpec, err := w.buildPodSpec(actionCtx, targetPodTemplate, targetPod)
		if err != nil {
			return err
		}
		actionTask, err := w.createPod(actionCtx, podSpec, task.TargetPodName, taskIndex, int(task.Retries))
		if err != nil {
			return err
		}
		task.ObjectKey = actionTask.ObjectKey
		return err
	})
}
