/*
Copyright ApeCloud Inc.

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

// Component is the interface to use for component status
type Component interface {
	// IsRunning when relevant k8s workloads changes, check whether the component is running.
	// you can also reconcile the pods of component util the component is Running here.
	IsRunning(obj client.Object) (bool, error)

	// PodsReady check whether all pods of the component are ready.
	PodsReady(obj client.Object) (bool, error)

	// HandleProbeTimeoutWhenPodsReady if the component need role probe and the pods of component are ready,
	// we should handle the component phase when the role probe timeout and return a bool.
	// if return true, means probe has not timed out and need to requeue after an interval time to handle probe timeout again.
	// else return false, means probe has timed out and need to update the component phase to Failed or Abnormal.
	HandleProbeTimeoutWhenPodsReady() (bool, error)

	// CalculatePhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
	// if return an empty phase, means the pods of component are ready and skips it.
	CalculatePhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error)
}
