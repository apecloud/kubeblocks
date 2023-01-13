package types

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

// Component is the interface to use for component status
type Component interface {
	// IsRunning when relevant k8s workloads changes, it checks whether the component is running.
	// you can also reconcile the pods of component till the component is Running here.
	IsRunning(obj client.Object) (bool, error)

	// PodsReady checks whether all pods of the component are ready.
	// it means the pods are available in StatefulSet or Deployment.
	PodsReady(obj client.Object) (bool, error)

	// PodIsAvailable checks whether a pod of the component is available.
	// if the component is Stateless/StatefulSet, the available conditions follows as:
	// 1. the pod is ready.
	// 2. readyTime reached minReadySeconds.
	// if the component is ConsensusSet,it will be available when the pod is ready and labeled with its role.
	PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool

	// HandleProbeTimeoutWhenPodsReady if the component needs role probe and the pods of component are ready,
	// we should handle the component phase when the role probe timeout and return a bool.
	// if return true, means probe is not timing out and need to requeue after an interval time to handle probe timeout again.
	// else return false, means probe has timed out and needs to update the component phase to Failed or Abnormal.
	HandleProbeTimeoutWhenPodsReady(recorder record.EventRecorder) (bool, error)

	// GetPhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
	// if return an empty phase, means the pods of component are ready and skips it.
	GetPhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error)
}

const (
	// ProbeTimeoutReason the event reason when all pods of the component probe role timed out.
	ProbeTimeoutReason = "ProbeTimeout"

	// ProbeTimeout the probe timeout
	ProbeTimeout = 1 * time.Minute
)
