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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type WorkloadAction struct {
	OpsRequest     *opsv1alpha1.OpsRequest
	Cluster        *appsv1.Cluster
	OpsDef         *opsv1alpha1.OpsDefinition
	CustomCompOps  *opsv1alpha1.CustomOpsComponent
	Comp           *appsv1.ClusterComponentSpec
	progressDetail opsv1alpha1.ProgressStatusDetail
}

func NewWorkloadAction(opsRequest *opsv1alpha1.OpsRequest,
	cluster *appsv1.Cluster,
	opsDef *opsv1alpha1.OpsDefinition,
	customCompOps *opsv1alpha1.CustomOpsComponent,
	comp *appsv1.ClusterComponentSpec,
	progressDetail opsv1alpha1.ProgressStatusDetail) *WorkloadAction {
	return &WorkloadAction{
		OpsRequest:     opsRequest,
		Cluster:        cluster,
		OpsDef:         opsDef,
		CustomCompOps:  customCompOps,
		Comp:           comp,
		progressDetail: progressDetail,
	}
}

func (w *WorkloadAction) Execute(actionCtx ActionContext) (*ActionStatus, error) {
	if actionCtx.Action.Workload == nil {
		return nil, nil
	}
	var (
		podInfoExtractorName = actionCtx.Action.Workload.PodInfoExtractorName
		targetPods           []*corev1.Pod
		err                  error
		podInfoExtractor     *opsv1alpha1.PodInfoExtractor
		actionStatus         = NewActiontatus()
	)
	if podInfoExtractorName != "" {
		// get target pods
		podInfoExtractor = getTargetPodInfoExtractor(w.OpsDef, podInfoExtractorName)
		if podInfoExtractor == nil {
			return nil, intctrlutil.NewFatalError("can not found the podInfoExtractor: " + podInfoExtractorName)
		}
		targetPods, err = getTargetPods(actionCtx.ReqCtx.Ctx, actionCtx.Client, w.Cluster, podInfoExtractor.PodSelector, w.CustomCompOps.ComponentName)
		if err != nil {
			return nil, err
		}
	}
	executeAction := func(podTemplate *opsv1alpha1.PodInfoExtractor, targetPod *corev1.Pod, index int) (*opsv1alpha1.ActionTask, error) {
		podSpec, err := w.buildPodSpec(actionCtx, podTemplate, targetPod)
		if err != nil {
			return nil, err
		}
		var targetPodName string
		if targetPod != nil {
			targetPodName = ""
		}
		return w.createWorkload(actionCtx, podSpec, targetPodName, index)
	}
	if len(targetPods) == 0 {
		// if no target pods, only create the workload by pod spec.
		actionTask, err := executeAction(nil, nil, 0)
		if actionTask != nil {
			actionStatus.ActionTasks = append(actionStatus.ActionTasks, *actionTask)
		}
		return actionStatus, err
	}
	actionPodSpecCopy := actionCtx.Action.Workload.PodSpec.DeepCopy()
	for i := range targetPods {
		actionCtx.Action.Workload.PodSpec = *actionPodSpecCopy
		actionTask, err := executeAction(podInfoExtractor, targetPods[i], i)
		if err != nil {
			return nil, err
		}
		if actionTask != nil {
			actionStatus.ActionTasks = append(actionStatus.ActionTasks, *actionTask)
		}
	}
	return actionStatus, nil
}

func (w *WorkloadAction) CheckStatus(actionCtx ActionContext) (*ActionStatus, error) {
	switch actionCtx.Action.Workload.Type {
	case opsv1alpha1.JobWorkload:
		return actionCtx.checkActionStatus(w.progressDetail, w.checkJobStatus)
	case opsv1alpha1.PodWorkload:
		return actionCtx.checkActionStatus(w.progressDetail, w.checkPodStatus)
	default:
		return nil, intctrlutil.NewFatalError("not implement for workload " + string(actionCtx.Action.Workload.Type))
	}
}

func (w *WorkloadAction) createWorkload(actionCtx ActionContext,
	podSpec *corev1.PodSpec,
	targetPodName string,
	taskIndex int) (*opsv1alpha1.ActionTask, error) {
	switch actionCtx.Action.Workload.Type {
	case opsv1alpha1.JobWorkload:
		return w.createJob(actionCtx, podSpec, targetPodName, taskIndex)
	case opsv1alpha1.PodWorkload:
		return w.createPod(actionCtx, podSpec, targetPodName, taskIndex, 0)
	default:
		return nil, intctrlutil.NewFatalError("not implement for workload " + string(actionCtx.Action.Workload.Type))
	}
}

func (w *WorkloadAction) buildPodSpec(actionCtx ActionContext,
	podInfoExtractor *opsv1alpha1.PodInfoExtractor,
	targetPod *corev1.Pod) (*corev1.PodSpec, error) {

	var (
		workloadAction = actionCtx.Action.Workload
		podSpec        = workloadAction.PodSpec.DeepCopy()
		volumeMounts   []corev1.VolumeMount
		env            []corev1.EnvVar
	)

	env, err := buildActionPodEnv(actionCtx.ReqCtx, actionCtx.Client, w.Cluster, w.OpsDef, w.OpsRequest,
		w.Comp, w.CustomCompOps, podInfoExtractor, targetPod)
	if err != nil {
		return nil, err
	}
	if podInfoExtractor != nil {
		// mount the target pod's volumes.
		for _, volumeMount := range podInfoExtractor.VolumeMounts {
			for _, volume := range targetPod.Spec.Volumes {
				if volume.Name != volumeMount.Name {
					continue
				}
				podSpec.Volumes = append(podSpec.Volumes, volume)
				volumeMounts = append(volumeMounts, volumeMount)
			}
		}
		if len(podInfoExtractor.VolumeMounts) > 0 {
			podSpec.NodeSelector = map[string]string{
				corev1.LabelHostname: targetPod.Spec.NodeName,
			}
		}
	}
	// inject container envs, volumeMounts and resource.
	for i := range podSpec.Containers {
		podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, env...)
		podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, volumeMounts...)
		intctrlutil.InjectZeroResourcesLimitsIfEmpty(&podSpec.Containers[i])
	}
	// inject extras script.
	w.injectOpsUtils(podSpec)
	for i := range podSpec.InitContainers {
		intctrlutil.InjectZeroResourcesLimitsIfEmpty(&podSpec.InitContainers[i])
	}
	if podSpec.RestartPolicy == "" {
		podSpec.RestartPolicy = corev1.RestartPolicyNever
	}
	if len(podSpec.Tolerations) == 0 && w.Comp.SchedulingPolicy != nil {
		podSpec.Tolerations = w.Comp.SchedulingPolicy.Tolerations
	}
	switch {
	case w.OpsRequest.Spec.CustomOps.ServiceAccountName != nil:
		// prioritize using the input sa.
		podSpec.ServiceAccountName = *w.OpsRequest.Spec.CustomOps.ServiceAccountName
	case w.Comp.ServiceAccountName != "":
		// using the component sa.
		podSpec.ServiceAccountName = w.Comp.ServiceAccountName
	default:
		saKey := client.ObjectKey{Namespace: w.Cluster.Namespace,
			Name: constant.GenerateDefaultServiceAccountName(w.Cluster.Name, w.Comp.Name)}
		if exists, _ := intctrlutil.CheckResourceExists(actionCtx.ReqCtx.Ctx, actionCtx.Client, saKey, &corev1.ServiceAccount{}); exists {
			podSpec.ServiceAccountName = saKey.Name
		}
	}
	return podSpec, nil
}

func (w *WorkloadAction) injectOpsUtils(podSpec *corev1.PodSpec) {
	opsUtilVolumeName := "ops-utils"
	opsUtilVolume := corev1.Volume{
		Name: opsUtilVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	volumeMount := corev1.VolumeMount{
		Name:      opsUtilVolumeName,
		MountPath: "/scripts",
	}
	// TODO: If necessary, you can package a tool for operating OpsRequest.status.extras.
	scripts := `cp /usr/bin/kubectl /scripts/kubectl;
echo '/scripts/kubectl -n "${KB_OPS_NAMESPACE}" patch opsrequests.operations.kubeblocks.io "${KB_OPS_NAME}" --subresource=status --type=merge --patch "{\"status\":{\"extras\":$1}}"' >/scripts/patch-extras-status.sh
`
	initContainer := corev1.Container{
		Name:            opsUtilVolumeName,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Image:           viper.GetString(constant.KBToolsImage),
		Command:         []string{"sh", "-c", scripts},
		VolumeMounts:    []corev1.VolumeMount{volumeMount},
	}
	podSpec.InitContainers = append(podSpec.InitContainers, initContainer)
	podSpec.Volumes = append(podSpec.Volumes, opsUtilVolume)
	for i := range podSpec.Containers {
		podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, volumeMount)
	}
}
