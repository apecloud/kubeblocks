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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type ExecAction struct {
	OpsRequest     *appsv1alpha1.OpsRequest
	Cluster        *appsv1alpha1.Cluster
	OpsDef         *appsv1alpha1.OpsDefinition
	CustomCompOps  *appsv1alpha1.CustomOpsComponent
	Comp           *appsv1alpha1.ClusterComponentSpec
	progressDetail appsv1alpha1.ProgressStatusDetail
}

func NewExecAction(opsRequest *appsv1alpha1.OpsRequest,
	cluster *appsv1alpha1.Cluster,
	opsDef *appsv1alpha1.OpsDefinition,
	customCompOps *appsv1alpha1.CustomOpsComponent,
	comp *appsv1alpha1.ClusterComponentSpec,
	progressDetail appsv1alpha1.ProgressStatusDetail) *ExecAction {
	return &ExecAction{
		OpsRequest:     opsRequest,
		Cluster:        cluster,
		OpsDef:         opsDef,
		CustomCompOps:  customCompOps,
		Comp:           comp,
		progressDetail: progressDetail,
	}
}

func (e *ExecAction) Execute(actionCtx ActionContext) (*ActionStatus, error) {
	if actionCtx.Action.Exec == nil {
		return nil, nil
	}
	var (
		podInfoExtractorName = actionCtx.Action.Exec.PodInfoExtractorName
		podInfoExtractor     *appsv1alpha1.PodInfoExtractor
		actionStatus         = NewActiontatus()
	)
	// get target pods
	podInfoExtractor = getTargetPodInfoExtractor(e.OpsDef, podInfoExtractorName)
	if podInfoExtractor == nil {
		return nil, intctrlutil.NewFatalError("can not found the podInfoExtractor: " + podInfoExtractorName)
	}
	targetPods, err := getTargetPods(actionCtx.ReqCtx.Ctx, actionCtx.Client, e.Cluster, podInfoExtractor.PodSelector, e.CustomCompOps.ComponentName)
	if err != nil {
		return nil, err
	}
	for i := range targetPods {
		podSpec, err := e.buildExecPodSpec(actionCtx, podInfoExtractor, targetPods[i])
		if err != nil {
			return nil, err
		}
		actionTask, err := e.createExecPod(actionCtx, podSpec, targetPods[i].Name, i, 0)
		if err != nil {
			return nil, err
		}
		if actionTask != nil {
			actionStatus.ActionTasks = append(actionStatus.ActionTasks, *actionTask)
		}
	}
	return actionStatus, nil
}

func (e *ExecAction) CheckStatus(actionCtx ActionContext) (*ActionStatus, error) {
	return actionCtx.checkActionStatus(e.progressDetail, e.checkActionPodStatus)
}

func (e *ExecAction) checkActionPodStatus(actionCtx ActionContext,
	task *appsv1alpha1.ActionTask,
	taskIndex int) (bool, bool, error) {
	return actionCtx.checkPodTaskStatus(task, actionCtx.Action.Exec.BackoffLimit, func() error {
		targetPodTemplate, targetPod, err := getTargetTemplateAndPod(actionCtx.ReqCtx.Ctx, actionCtx.Client,
			e.OpsDef, actionCtx.Action.Exec.PodInfoExtractorName, task.TargetPodName, e.OpsRequest.Namespace)
		if err != nil {
			return err
		}
		podSpec, err := e.buildExecPodSpec(actionCtx, targetPodTemplate, targetPod)
		if err != nil {
			return err
		}
		actionTask, err := e.createExecPod(actionCtx, podSpec, task.TargetPodName, taskIndex, int(task.Retries))
		if err != nil {
			return err
		}
		task.ObjectKey = actionTask.ObjectKey
		return nil
	})
}

func (e *ExecAction) createExecPod(actionCtx ActionContext,
	podSpec *corev1.PodSpec,
	targetPodName string,
	targetPodIndex,
	retries int) (*appsv1alpha1.ActionTask, error) {
	podName := buildActionPodName(e.OpsRequest, e.Comp.Name, actionCtx.Action.Name, targetPodIndex, retries)
	namespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	serviceAccountName := viper.GetString(constant.KBServiceAccountName)
	if e.OpsRequest.Spec.CustomOps.ServiceAccountName != nil {
		serviceAccountName = *e.OpsRequest.Spec.CustomOps.ServiceAccountName
		namespace = e.OpsRequest.Namespace
	}
	pod := builder.NewPodBuilder(namespace, podName).
		AddLabelsInMap(buildLabels(e.OpsRequest.Name, actionCtx.Action.Name)).
		AddLabels(constant.OpsRequestNamespaceLabelKey, e.OpsRequest.Namespace).
		SetPodSpec(*podSpec).
		AddServiceAccount(serviceAccountName).
		SetRestartPolicy(corev1.RestartPolicyNever).
		GetObject()
	pod.Kind = constant.PodKind
	return actionCtx.createActionK8sWorkload(e.OpsRequest, pod, targetPodName)
}

func (e *ExecAction) buildExecPodSpec(actionCtx ActionContext,
	podInfoExtractor *appsv1alpha1.PodInfoExtractor,
	targetPod *corev1.Pod) (*corev1.PodSpec, error) {
	// inject component and componentDef envs
	env, err := buildActionPodEnv(actionCtx.ReqCtx, actionCtx.Client, e.Cluster, e.OpsDef,
		e.OpsRequest, e.Comp, e.CustomCompOps, podInfoExtractor, targetPod)
	if err != nil {
		return nil, err
	}
	execAction := actionCtx.Action.Exec
	containerName := execAction.ContainerName
	if containerName == "" {
		containerName = targetPod.Spec.Containers[0].Name
	}
	container := &corev1.Container{
		Name:            actionCtx.Action.Name,
		Image:           viper.GetString(constant.KBToolsImage),
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		Command:         []string{"kubectl"},
		Env:             env,
		Args: append([]string{
			"-n",
			targetPod.Namespace,
			"exec",
			targetPod.Name,
			"-c",
			containerName,
			"--",
		}, execAction.Command...),
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
	return &corev1.PodSpec{
		Containers: []corev1.Container{*container},
		// tolerate all taints
		Tolerations:      e.Comp.Tolerations,
		ImagePullSecrets: intctrlutil.BuildImagePullSecrets(),
	}, nil
}
