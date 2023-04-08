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

package types

import (
	"time"
)

const (
	// RoleProbeTimeoutReason the event reason when all pods of the component role probe timed out.
	RoleProbeTimeoutReason = "RoleProbeTimeout"

	// PodContainerFailedTimeout the timeout for container of pod failures, the component phase will be set to Failed/Abnormal after this time.
	PodContainerFailedTimeout = time.Minute
)

//
//// Component is the interface to use for component status
//type Component interface {
//	// IsRunning when relevant k8s workloads changes, it checks whether the component is running.
//	// you can also reconcile the pods of component till the component is Running here.
//	IsRunning(ctx context.Context, obj client.Object) (bool, error)
//
//	// PodsReady checks whether all pods of the component are ready.
//	// it means the pods are available in StatefulSet or Deployment.
//	PodsReady(ctx context.Context, obj client.Object) (bool, error)
//
//	// PodIsAvailable checks whether a pod of the component is available.
//	// if the component is Stateless/StatefulSet, the available conditions follows as:
//	// 1. the pod is ready.
//	// 2. readyTime reached minReadySeconds.
//	// if the component is ConsensusSet,it will be available when the pod is ready and labeled with its role.
//	PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool
//
//	//// HandleProbeTimeoutWhenPodsReady if the component has no role probe, return false directly. otherwise,
//	//// we should handle the component phase when the role probe timeout and return a bool.
//	//// if return true, means probe is not timing out and need to requeue after an interval time to handle probe timeout again.
//	//// else return false, means probe has timed out and needs to update the component phase to Failed or Abnormal.
//	//HandleProbeTimeoutWhenPodsReady(ctx context.Context, recorder record.EventRecorder) (bool, error)
//
//	// GetPhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
//	// if return an empty phase, means the pods of component are ready and skips it.
//	GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (appsv1alpha1.ClusterComponentPhase, error)
//
//	//// HandleUpdate handles component updating when basic workloads of the components are updated
//	//HandleUpdate(ctx context.Context, obj client.Object) error
//
//	HandleRestart(ctx context.Context, obj client.Object) error
//
//	HandleRoleChange(ctx context.Context, obj client.Object) error
//
//	GetName() string
//	GetNamespace() string
//	GetMatchingLabels() client.MatchingLabels
//
//	GetDefinition() *appsv1alpha1.ClusterComponentDefinition
//
//	GetLatestStatus(ctx context.Context, obj client.Object) (*appsv1alpha1.ClusterComponentStatus, error)
//}
//
//type ComponentBase struct {
//	Cli               client.Client
//	Cluster           *appsv1alpha1.Cluster
//	Component         *appsv1alpha1.ClusterComponentSpec
//	ComponentDef      *appsv1alpha1.ClusterComponentDefinition
//	ConcreteComponent Component
//	Dag               *graph.DAG
//}
//
//func (c *ComponentBase) GetName() string {
//	return c.Component.Name
//}
//
//func (c *ComponentBase) GetNamespace() string {
//	return c.Cluster.GetNamespace()
//}
//
//func (c *ComponentBase) GetMatchingLabels() client.MatchingLabels {
//	return util.GetComponentMatchLabels(c.GetNamespace(), c.GetName())
//}
//
//func (c *ComponentBase) GetDefinition() *appsv1alpha1.ClusterComponentDefinition {
//	return c.ComponentDef
//}
//
////func (c *ComponentBase) HandleProbeTimeoutWhenPodsReady(_ context.Context, _ record.EventRecorder) (bool, error) {
////	return false, nil
////}
//
////func (c *ComponentBase) HandleUpdate(_ context.Context, _ client.Object) error {
////	return nil
////}
//
//func (c *ComponentBase) HandleRestart(_ context.Context, _ client.Object) error {
//	return nil
//}
//
//func (c *ComponentBase) HandleRoleChange(_ context.Context, _ client.Object) error {
//	return nil
//}
//
//func (c *ComponentBase) GetLatestStatus(ctx context.Context, obj client.Object) (*appsv1alpha1.ClusterComponentStatus, error) {
//	return c.RebuildLatestStatus(ctx, obj, nil)
//}
//
//func (c *ComponentBase) RebuildLatestStatus(ctx context.Context, obj client.Object,
//	handleProbeTimeoutWhenPodsReady func(*appsv1alpha1.ClusterComponentStatus, []*corev1.Pod)) (*appsv1alpha1.ClusterComponentStatus, error) {
//	pods, err := util.ListPodOwnedByComponent(ctx, c.Cli, c.GetNamespace(), c.GetMatchingLabels())
//	if err != nil {
//		return nil, err
//	}
//
//	isRunning, err := c.ConcreteComponent.IsRunning(ctx, obj)
//	if err != nil {
//		return nil, err
//	}
//
//	var podsReady *bool
//	if c.Component.Replicas > 0 {
//		podsReadyForComponent, err := c.ConcreteComponent.PodsReady(ctx, obj)
//		if err != nil {
//			return nil, err
//		}
//		podsReady = &podsReadyForComponent
//	}
//
//	status := &appsv1alpha1.ClusterComponentStatus{}
//
//	//var wait bool
//	hasTimedOutPod := false
//	if !isRunning {
//		if podsReady != nil && *podsReady {
//			if handleProbeTimeoutWhenPodsReady != nil {
//				// check if the role probe timed out when component phase is not Running but all pods of component are ready.
//				handleProbeTimeoutWhenPodsReady(status, pods)
//				//wait = true
//			}
//		} else {
//			hasTimedOutPod, status.Message, err = hasFailedAndTimedOutPod(pods)
//			if err != nil {
//				return nil, err
//			}
//			if !hasTimedOutPod {
//				//wait = true
//			}
//		}
//	}
//
//	if err = c.rebuildComponentStatus(ctx, isRunning, podsReady, hasTimedOutPod, status); err != nil {
//		return nil, err
//	}
//	return status, nil
//}
//
//// updateComponentsPhase updates the component status Phase etc. into the cluster.Status.Components map.
//func (c *ComponentBase) rebuildComponentStatus(ctx context.Context,
//	running bool,
//	podsAreReady *bool,
//	hasFailedPodTimedOut bool,
//	status *appsv1alpha1.ClusterComponentStatus) error {
//	if !running {
//		// if no operation is running in cluster or failed pod timed out,
//		// means the component is Failed or Abnormal.
//		if slices.Contains(appsv1alpha1.GetClusterUpRunningPhases(), c.Cluster.Status.Phase) || hasFailedPodTimedOut {
//			if phase, err := c.ConcreteComponent.GetPhaseWhenPodsNotReady(ctx, c.GetName()); err != nil {
//				return err
//			} else if phase != "" {
//				status.Phase = phase
//			}
//		}
//	} else {
//		if c.Component.Replicas == 0 {
//			// if replicas number of component is zero, the component has stopped.
//			// 'Stopped' is a special 'Running' for workload(StatefulSet/Deployment).
//			status.Phase = appsv1alpha1.StoppedClusterCompPhase
//		} else {
//			// change component phase to Running when workloads of component are running.
//			status.Phase = appsv1alpha1.RunningClusterCompPhase
//		}
//		status.SetMessage(nil)
//	}
//	status.PodsReady = podsAreReady
//	if podsAreReady != nil && *podsAreReady {
//		status.PodsReadyTime = &metav1.Time{Time: time.Now()}
//	} else {
//		status.PodsReadyTime = nil
//	}
//	return nil
//}
//
//// hasFailedAndTimedOutPod returns whether the pod of components is still failed after a PodFailedTimeout period.
//// if return ture, component phase will be set to Failed/Abnormal.
//func hasFailedAndTimedOutPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, error) {
//	hasTimedoutPod := false
//	messages := appsv1alpha1.ComponentMessageMap{}
//	for _, pod := range pods {
//		isFailed, isTimedOut, messageStr := isPodFailedAndTimedOut(pod)
//		if !isFailed {
//			continue
//		}
//		if isTimedOut {
//			hasTimedoutPod = true
//			messages.SetObjectMessage(pod.Kind, pod.Name, messageStr)
//		}
//	}
//	return hasTimedoutPod, messages, nil
//}
//
//// isPodFailedAndTimedOut checks if the pod is failed and timed out.
//func isPodFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
//	initContainerFailed, message := isAnyContainerFailed(pod.Status.InitContainerStatuses)
//	if initContainerFailed {
//		return initContainerFailed, isContainerFailedAndTimedOut(pod, corev1.PodInitialized), message
//	}
//	containerFailed, message := isAnyContainerFailed(pod.Status.ContainerStatuses)
//	if containerFailed {
//		return containerFailed, isContainerFailedAndTimedOut(pod, corev1.ContainersReady), message
//	}
//	return false, false, ""
//}
//
//// isAnyContainerFailed checks whether any container in the list is failed.
//func isAnyContainerFailed(containersStatus []corev1.ContainerStatus) (bool, string) {
//	for _, v := range containersStatus {
//		waitingState := v.State.Waiting
//		if waitingState != nil && waitingState.Message != "" {
//			return true, waitingState.Message
//		}
//		terminatedState := v.State.Terminated
//		if terminatedState != nil && terminatedState.Message != "" {
//			return true, terminatedState.Message
//		}
//	}
//	return false, ""
//}
//
//// isContainerFailedAndTimedOut checks whether the failed container has timed out.
//func isContainerFailedAndTimedOut(pod *corev1.Pod, podConditionType corev1.PodConditionType) bool {
//	containerReadyCondition := intctrlutil.GetPodCondition(&pod.Status, podConditionType)
//	if containerReadyCondition == nil || containerReadyCondition.LastTransitionTime.IsZero() {
//		return false
//	}
//	return time.Now().After(containerReadyCondition.LastTransitionTime.Add(PodContainerFailedTimeout))
//}
