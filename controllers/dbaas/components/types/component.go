package types

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

// Component is the interface to use for component status
type Component interface {
	// IsRunning when relevant k8s workloads changes, check whether the component is running.
	// you can also reconcile the pods of component util the component is Running here.
	IsRunning(obj client.Object) (bool, error)

	// PodsReady check whether all pods of the component are ready.
	// it means the pods are available in StatefulSet or Deployment here.
	PodsReady(obj client.Object) (bool, error)

	// PodIsAvailable check whether the pod of the component is available.
	// if the component is Stateless/StatefulSet, the available conditions follows as:
	// 1. the pod is ready.
	// 2. readyTime reached minReadySeconds.
	// if the component is ConsensusSet,it will be available when the pod is ready and contains role label.
	PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool

	// HandleProbeTimeoutWhenPodsReady if the component need role probe and the pods of component are ready,
	// we should handle the component phase when the role probe timeout and return a bool.
	// if return true, means probe has not timed out and need to requeue after an interval time to handle probe timeout again.
	// else return false, means probe has timed out and need to update the component phase to Failed or Abnormal.
	HandleProbeTimeoutWhenPodsReady() (bool, error)

	// CalculatePhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
	// if return an empty phase, means the pods of component are ready and skips it.
	CalculatePhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error)
}
