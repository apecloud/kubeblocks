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

	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ComponentStatusSynchronizer gathers running status from Cluster, component's Workload and Pod objects,
// then fills status of component (e.g. abnormalities or failures) into the Cluster.Status.Components map.
//
// Although it works to use warning event to determine whether the component is abnormal or failed.
// In some conditions, the warning events are possible to be throttled and dropped due to K8s event rate control.
// For example, after the kubelet fails to pull the image, it will put the image into the backoff cache
// and send the BackOff (Normal) event. If it has already consumed the 25 burst quota to send event, event can only be
// sent in the rate of once per 300s, in this way, the subsequent warning events of ImagePullError would be dropped.
type ComponentStatusSynchronizer struct {
	ctx           context.Context
	cli           client.Client
	cluster       *appsv1alpha1.Cluster
	component     types.Component
	componentSpec *appsv1alpha1.ClusterComponentSpec
	podList       *corev1.PodList
}

// newClusterStatusSynchronizer creates and initializes a ComponentStatusSynchronizer objects.
// It represents a snapshot of cluster status, including workloads and pods.
func newClusterStatusSynchronizer(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec,
	component types.Component,
) (*ComponentStatusSynchronizer, error) {
	podList, err := util.GetComponentPodList(ctx, cli, *cluster, componentSpec.Name)
	if err != nil {
		return nil, err
	}
	return &ComponentStatusSynchronizer{
		ctx:           ctx,
		cli:           cli,
		cluster:       cluster,
		component:     component,
		componentSpec: componentSpec,
		podList:       podList,
	}, nil
}

func (cs *ComponentStatusSynchronizer) Update(ctx context.Context, obj client.Object, logger *logr.Logger,
	recorder record.EventRecorder) (bool, error) {
	var (
		component = cs.component
		wait      = false
	)

	if component == nil {
		return false, nil
	}
	// handle the components changes
	err := component.HandleUpdate(ctx, obj)
	if err != nil {
		return false, nil
	}

	isRunning, err := component.IsRunning(ctx, obj)
	if err != nil {
		return false, err
	}

	var podsReady *bool
	if cs.componentSpec.Replicas > 0 {
		podsReadyForComponent, err := component.PodsReady(ctx, obj)
		if err != nil {
			return false, err
		}
		podsReady = &podsReadyForComponent
	}

	cluster := cs.cluster
	hasFailedAndTimedOutPod := false
	clusterDeepCopy := cluster.DeepCopy()
	if !isRunning {
		if podsReady != nil && *podsReady {
			// check if the role probe timed out when component phase is not Running but all pods of component are ready.
			if requeueWhenPodsReady, err := component.HandleProbeTimeoutWhenPodsReady(ctx, recorder); err != nil {
				return false, err
			} else if requeueWhenPodsReady {
				wait = true
			}
		} else {
			// check whether there is a failed pod of component that has timed out
			var hasFailedPod bool
			hasFailedAndTimedOutPod, hasFailedPod = cs.hasFailedAndTimedOutPod()
			if !hasFailedAndTimedOutPod && hasFailedPod {
				wait = true
			}
		}
	}

	if err = cs.updateComponentsPhase(ctx, isRunning,
		podsReady, hasFailedAndTimedOutPod); err != nil {
		return wait, err
	}

	componentName := cs.componentSpec.Name
	oldComponentStatus := clusterDeepCopy.Status.Components[componentName]

	// REVIEW: this function was refactored by Caowei, the original function create need to review this part
	if componentStatus, ok := cluster.Status.Components[componentName]; ok && !reflect.DeepEqual(oldComponentStatus, componentStatus) {
		logger.Info("component status changed", "componentName", componentName, "phase",
			componentStatus.Phase, "componentIsRunning", isRunning, "podsAreReady", podsReady)
		patch := client.MergeFrom(clusterDeepCopy)
		if err = cs.cli.Status().Patch(cs.ctx, cluster, patch); err != nil {
			return false, err
		}
	}
	return wait, nil
}

// hasFailedAndTimedOutPod returns whether the pod of components is still failed after a PodFailedTimeout period.
// if return ture, component phase will be set to Failed/Abnormal.
func (cs *ComponentStatusSynchronizer) hasFailedAndTimedOutPod() (hasFailedAndTimedoutPod bool, hasFailedPod bool) {
	// init a new ComponentMessageMap to store the message of failed pods
	message := appsv1alpha1.ComponentMessageMap{}
	for _, pod := range cs.podList.Items {
		isFailed, isTimedOut, messageStr := isPodFailedAndTimedOut(&pod)
		if !isFailed {
			continue
		}
		hasFailedPod = true

		if isTimedOut {
			hasFailedAndTimedoutPod = true
			message.SetObjectMessage(pod.Kind, pod.Name, messageStr)
		}
	}
	if hasFailedAndTimedoutPod {
		cs.updateMessage(message)
	}
	return
}

// updateComponentsPhase updates the component status Phase etc. into the cluster.Status.Components map.
func (cs *ComponentStatusSynchronizer) updateComponentsPhase(
	ctx context.Context,
	componentIsRunning bool,
	podsAreReady *bool,
	hasFailedPodTimedOut bool) error {
	var (
		status        = &cs.cluster.Status
		podsReadyTime *metav1.Time
		componentName = cs.componentSpec.Name
	)
	if podsAreReady != nil && *podsAreReady {
		podsReadyTime = &metav1.Time{Time: time.Now()}
	}
	componentStatus := cs.cluster.Status.Components[cs.componentSpec.Name]
	if !componentIsRunning {
		// if no operation is running in cluster or failed pod timed out,
		// means the component is Failed or Abnormal.
		if slices.Contains(appsv1alpha1.GetClusterTerminalPhases(), cs.cluster.Status.Phase) || hasFailedPodTimedOut {
			if phase, err := cs.component.GetPhaseWhenPodsNotReady(ctx, componentName); err != nil {
				return err
			} else if phase != "" {
				componentStatus.Phase = phase
			}
		}
	} else {
		if cs.componentSpec.Replicas == 0 {
			// if replicas number of component is zero, the component has stopped.
			// 'Stopped' is a special 'Running' for workload(StatefulSet/Deployment).
			componentStatus.Phase = appsv1alpha1.StoppedClusterCompPhase
		} else {
			// change component phase to Running when workloads of component are running.
			componentStatus.Phase = appsv1alpha1.RunningClusterCompPhase
		}
		componentStatus.SetMessage(nil)
	}
	componentStatus.PodsReadyTime = podsReadyTime
	componentStatus.PodsReady = podsAreReady
	status.SetComponentStatus(componentName, componentStatus)
	return nil
}

// updateMessage is an internal helper method which updates the component status message in the Cluster.Status.Components map.
func (cs *ComponentStatusSynchronizer) updateMessage(message appsv1alpha1.ComponentMessageMap) {
	compStatus := cs.cluster.Status.Components[cs.componentSpec.Name]
	compStatus.Message = message
	cs.setStatus(compStatus)
}

// setStatus is an internal helper method which sets component status in Cluster.Status.Components map.
func (cs *ComponentStatusSynchronizer) setStatus(compStatus appsv1alpha1.ClusterComponentStatus) {
	cs.cluster.Status.SetComponentStatus(cs.componentSpec.Name, compStatus)
}

// isPodFailedAndTimedOut checks if the pod is failed and timed out.
func isPodFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
	initContainerFailed, message := isAnyContainerFailed(pod.Status.InitContainerStatuses)
	if initContainerFailed {
		return initContainerFailed, isContainerFailedAndTimedOut(pod, corev1.PodInitialized), message
	}
	containerFailed, message := isAnyContainerFailed(pod.Status.ContainerStatuses)
	if containerFailed {
		return containerFailed, isContainerFailedAndTimedOut(pod, corev1.ContainersReady), message
	}
	return false, false, ""
}

// isAnyContainerFailed checks whether any container in the list is failed.
func isAnyContainerFailed(containersStatus []corev1.ContainerStatus) (bool, string) {
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

// isContainerFailedAndTimedOut checks whether the failed container has timed out.
func isContainerFailedAndTimedOut(pod *corev1.Pod, podConditionType corev1.PodConditionType) bool {
	containerReadyCondition := intctrlutil.GetPodCondition(&pod.Status, podConditionType)
	if containerReadyCondition == nil || containerReadyCondition.LastTransitionTime.IsZero() {
		return false
	}
	return time.Now().After(containerReadyCondition.LastTransitionTime.Add(types.PodContainerFailedTimeout))
}
