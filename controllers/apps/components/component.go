/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package components

import (
	"context"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// componentContext wrapper for handling component status procedure context parameters.
type componentContext struct {
	reqCtx        intctrlutil.RequestCtx
	cli           client.Client
	recorder      record.EventRecorder
	component     types.Component
	obj           client.Object
	componentSpec *appsv1alpha1.ClusterComponentSpec
}

// newComponentContext creates a componentContext object.
func newComponentContext(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	recorder record.EventRecorder,
	component types.Component,
	obj client.Object,
	componentSpec *appsv1alpha1.ClusterComponentSpec) componentContext {
	return componentContext{
		reqCtx:        reqCtx,
		cli:           cli,
		recorder:      recorder,
		component:     component,
		obj:           obj,
		componentSpec: componentSpec,
	}
}

// NewComponentByType creates a component object
func NewComponentByType(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	component *appsv1alpha1.ClusterComponentSpec) types.Component {
	if componentDef == nil {
		return nil
	}
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return consensusset.NewConsensusSet(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Replication:
		return replicationset.NewReplicationSet(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Stateful:
		return stateful.NewStateful(ctx, cli, cluster, component, componentDef)
	case appsv1alpha1.Stateless:
		return stateless.NewStateless(ctx, cli, cluster, component, componentDef)
	}
	return nil
}

// handleComponentStatusAndSyncCluster handles component status. if the component status changed, sync cluster.status.components
func handleComponentStatusAndSyncCluster(compCtx componentContext,
	cluster *appsv1alpha1.Cluster) (requeueAfter time.Duration, err error) {
	var (
		obj                  = compCtx.obj
		component            = compCtx.component
		podsReady            *bool
		isRunning            bool
		requeueWhenPodsReady bool
		hasFailedPodTimedOut bool
	)

	if component == nil {
		return
	}
	if compCtx.componentSpec.Replicas == 0 {
		podsReady = nil
	} else {
		podsReadyForComponent, err := component.PodsReady(obj)
		if err != nil {
			return 0, err
		}
		podsReady = &podsReadyForComponent
	}
	if err = component.HandleUpdate(obj); err != nil {
		return
	}
	if isRunning, err = component.IsRunning(obj); err != nil {
		return
	}
	// snapshot cluster
	clusterDeepCopy := cluster.DeepCopy()
	if !isRunning {
		if podsReady != nil && *podsReady {
			// check if the role probe timed out when component phase is not Running but all pods of component are ready.
			if requeueWhenPodsReady, err = component.HandleProbeTimeoutWhenPodsReady(compCtx.recorder); err != nil {
				return
			} else if requeueWhenPodsReady {
				requeueAfter = time.Minute
			}
		} else {
			// check whether there is a failed pod of component that has timed out
			if hasFailedPodTimedOut, requeueAfter, err = hasPodFailedTimedOut(compCtx, cluster); err != nil {
				return
			}
		}
	}

	if err = handleClusterComponentStatus(compCtx, clusterDeepCopy, cluster, podsReady, isRunning, hasFailedPodTimedOut); err != nil {
		return
	}

	return requeueAfter, opsutil.MarkRunningOpsRequestAnnotation(compCtx.reqCtx.Ctx, compCtx.cli, cluster)
}

// handleClusterComponentStatus handles Cluster.status.component status
func handleClusterComponentStatus(
	compCtx componentContext,
	clusterDeepCopy *appsv1alpha1.Cluster,
	cluster *appsv1alpha1.Cluster,
	podsAreReady *bool,
	componentIsRunning,
	hasFailedPodTimedOut bool) error {
	// when component phase is changed, set needSyncStatusComponent to true, then patch cluster.status
	patch := client.MergeFrom(clusterDeepCopy)
	if err := syncComponentsStatus(compCtx, cluster, podsAreReady, componentIsRunning, hasFailedPodTimedOut); err != nil {
		return err
	}
	componentName := compCtx.componentSpec.Name
	oldComponentStatus := clusterDeepCopy.Status.Components[componentName]
	componentStatus := cluster.Status.Components[componentName]
	if reflect.DeepEqual(oldComponentStatus, componentStatus) {
		return nil
	}
	compCtx.reqCtx.Log.Info("component status changed", "componentName", componentName, "phase",
		cluster.Status.Components[componentName].Phase, "componentIsRunning", componentIsRunning, "podsAreReady", podsAreReady)
	return compCtx.cli.Status().Patch(compCtx.reqCtx.Ctx, cluster, patch)
}

// syncComponentsStatus syncs the component status.
func syncComponentsStatus(compCtx componentContext,
	cluster *appsv1alpha1.Cluster,
	podsAreReady *bool,
	componentIsRunning,
	hasFailedPodTimedOut bool) error {
	var (
		status        = &cluster.Status
		podsReadyTime *metav1.Time
		componentName = compCtx.componentSpec.Name
	)
	if podsAreReady != nil && *podsAreReady {
		podsReadyTime = &metav1.Time{Time: time.Now()}
	}
	componentStatus := getClusterComponentStatus(cluster, componentName)
	if !componentIsRunning {
		// if no operation is running in cluster or failed pod timed out,
		// means the component is Failed or Abnormal.
		if util.IsCompleted(cluster.Status.Phase) || hasFailedPodTimedOut {
			if phase, err := compCtx.component.GetPhaseWhenPodsNotReady(componentName); err != nil {
				return err
			} else if phase != "" {
				componentStatus.Phase = phase
			}
		}
	} else {
		if compCtx.componentSpec.Replicas == 0 {
			// if replicas number of component is zero, the component has stopped.
			// 'Stopped' is a special 'Running' for workload(StatefulSet/Deployment).
			componentStatus.Phase = appsv1alpha1.StoppedPhase
		} else {
			// change component phase to Running when workloads of component are running.
			componentStatus.Phase = appsv1alpha1.RunningPhase
		}
		componentStatus.SetMessage(nil)
	}
	componentStatus.PodsReadyTime = podsReadyTime
	componentStatus.PodsReady = podsAreReady
	status.Components[componentName] = componentStatus
	return nil
}

// getClusterComponentStatus gets the component status in cluster by component name.
func getClusterComponentStatus(cluster *appsv1alpha1.Cluster, componentName string) appsv1alpha1.ClusterComponentStatus {
	var (
		componentStatus appsv1alpha1.ClusterComponentStatus
		status          = &cluster.Status
		ok              bool
	)
	if status.Components == nil {
		status.Components = map[string]appsv1alpha1.ClusterComponentStatus{}
	}
	if componentStatus, ok = status.Components[componentName]; !ok {
		componentStatus = appsv1alpha1.ClusterComponentStatus{
			Phase: cluster.Status.Phase,
		}
		status.Components[componentName] = componentStatus
	}
	return componentStatus
}

// updateComponentStatusMessage updates the message of the component in Cluster.status.components.
func updateComponentStatusMessage(compStatus *appsv1alpha1.ClusterComponentStatus,
	pod *corev1.Pod,
	message string) {
	if compStatus.Message == nil {
		compStatus.Message = appsv1alpha1.ComponentMessageMap{}
	}
	compStatus.Message.SetObjectMessage(pod.Kind, pod.Name, message)
}

// hasPodFailedTimedOut returns whether the pod of components is still failed after a PodFailedTimeout period.
// if return ture, component phase will be set to Failed/Abnormal.
// Generally, it is sufficient to use warning event to determine whether the component is abnormal or failed.
// However, the warning event will be lost all the time due to the event manager's flow restriction policy.
// For example, after the kubelet fails to pull the image, it will put the image into the backoff cache
// and send the BackOff (Normal) event first which consumes the quota of 1/300s when the 25 burst quota of event are consumed.
// so the warning event of ImagePullError will be lost all the time.
func hasPodFailedTimedOut(compCtx componentContext, cluster *appsv1alpha1.Cluster) (failedAndTimedOut bool, requeueAfter time.Duration, err error) {
	componentName := compCtx.componentSpec.Name
	podList, err := util.GetComponentPodList(compCtx.reqCtx.Ctx, compCtx.cli, cluster, componentName)
	if err != nil {
		return
	}
	componentStatus := getClusterComponentStatus(cluster, componentName)
	for _, v := range podList.Items {
		isFailed, isTimedOut, message := podFailedAndTimedOut(&v)
		if !isFailed {
			continue
		}
		if !isTimedOut {
			requeueAfter = time.Minute
		}
		if isTimedOut {
			updateComponentStatusMessage(&componentStatus, &v, message)
			cluster.Status.Components[componentName] = componentStatus
			failedAndTimedOut = true
			return
		}
	}
	return false, requeueAfter, nil
}

// podFailedAndTimedOut checks if the pod is failed and timed out.
func podFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
	initContainerFailed, message := hasContainerFailed(pod.Status.InitContainerStatuses)
	if initContainerFailed {
		return initContainerFailed, containerFailedTimedOut(pod, corev1.PodInitialized), message
	}
	containerFailed, message := hasContainerFailed(pod.Status.ContainerStatuses)
	if containerFailed {
		return containerFailed, containerFailedTimedOut(pod, corev1.ContainersReady), message
	}
	return false, false, ""
}

// hasContainerFailed checks whether the container of pod is failed.
func hasContainerFailed(containersStatus []corev1.ContainerStatus) (bool, string) {
	for _, v := range containersStatus {
		waitingState := v.State.Waiting
		if waitingState != nil && waitingState.Message != "" {
			return true, waitingState.Message
		}
		terminatedState := v.State.Terminated
		if terminatedState != nil && terminatedState.Message != "" {
			return true, waitingState.Message
		}
	}
	return false, ""
}

// containerFailedTimedOut checks whether the failed container has timed out.
func containerFailedTimedOut(pod *corev1.Pod, podConditionType corev1.PodConditionType) bool {
	containerReadyCondition := intctrlutil.GetPodCondition(&pod.Status, podConditionType)
	if containerReadyCondition == nil || containerReadyCondition.LastTransitionTime.IsZero() {
		return false
	}
	return time.Now().After(containerReadyCondition.LastTransitionTime.Add(types.PodContainerFailedTimeout))
}
