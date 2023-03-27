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
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// Component is the interface to use for component status
type Component interface {
	// IsRunning when relevant k8s workloads changes, it checks whether the component is running.
	// you can also reconcile the pods of component till the component is Running here.
	IsRunning(ctx context.Context, obj client.Object) (bool, error)

	// PodsReady checks whether all pods of the component are ready.
	// it means the pods are available in StatefulSet or Deployment.
	PodsReady(ctx context.Context, obj client.Object) (bool, error)

	// PodIsAvailable checks whether a pod of the component is available.
	// if the component is Stateless/StatefulSet, the available conditions follows as:
	// 1. the pod is ready.
	// 2. readyTime reached minReadySeconds.
	// if the component is ConsensusSet,it will be available when the pod is ready and labeled with its role.
	PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool

	// HandleProbeTimeoutWhenPodsReady if the component has no role probe, return false directly. otherwise,
	// we should handle the component phase when the role probe timeout and return a bool.
	// if return true, means probe is not timing out and need to requeue after an interval time to handle probe timeout again.
	// else return false, means probe has timed out and needs to update the component phase to Failed or Abnormal.
	HandleProbeTimeoutWhenPodsReady(ctx context.Context, recorder record.EventRecorder) (bool, error)

	// GetPhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
	// if return an empty phase, means the pods of component are ready and skips it.
	GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (appsv1alpha1.ClusterComponentPhase, error)

	// HandleUpdate handles component updating when basic workloads of the components are updated
	HandleUpdate(ctx context.Context, obj client.Object) error
}

const (
	// RoleProbeTimeoutReason the event reason when all pods of the component role probe timed out.
	RoleProbeTimeoutReason = "RoleProbeTimeout"

	// PodContainerFailedTimeout the timeout for container of pod failures, the component phase will be set to Failed/Abnormal after this time.
	PodContainerFailedTimeout = time.Minute
)
