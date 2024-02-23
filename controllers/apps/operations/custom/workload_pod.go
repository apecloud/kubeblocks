package custom

import (
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

// createJob creates the job workload.
func (w *WorkloadAction) createPod(actionCtx ActionContext,
	podSpec *corev1.PodSpec,
	targetPodName string,
	targetPodIndex,
	retries int) (*appsv1alpha1.ActionTask, error) {
	podName := buildActionPodName(w.OpsRequest, w.Comp.Name, actionCtx.Action.Name, targetPodIndex, retries)
	pod := builder.NewPodBuilder(w.OpsRequest.Namespace, podName).
		AddLabelsInMap(buildLabels(w.Cluster.Name, w.OpsRequest.Name, w.Comp.Name, actionCtx.Action.Name)).
		AddLabels(constant.OpsRequestNamespaceLabelKey, w.OpsRequest.Namespace).
		SetPodSpec(*podSpec).
		SetRestartPolicy(corev1.RestartPolicyNever).
		GetObject()
	pod.Kind = constant.PodKind
	return actionCtx.createActionK8sWorkload(w.OpsRequest, pod, targetPodName)
}

// checkPodStatus checks if the pod is finished and failed by status.
func (w *WorkloadAction) checkPodStatus(actionCtx ActionContext,
	task *appsv1alpha1.ActionTask,
	taskIndex int) (bool, bool, error) {
	return actionCtx.checkPodTaskStatus(task, actionCtx.Action.Workload.BackoffLimit, func() error {
		targetPodTemplate, targetPod, err := getTargetTemplateAndPod(actionCtx.ReqCtx.Ctx, actionCtx.Client,
			w.OpsDef, actionCtx.Action.Workload.TargetPodTemplate, task.TargetPodName, w.OpsRequest.Namespace)
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
