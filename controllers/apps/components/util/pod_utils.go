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

package util

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// PodIsReady checks whether pod is ready or not if the component is ConsensusSet or ReplicationSet,
// it will be available when the pod is ready and labeled with its role.
func PodIsReady(pod corev1.Pod) bool {
	if _, ok := pod.Labels[intctrlutil.RoleLabelKey]; !ok {
		return false
	}

	return intctrlutil.PodIsReady(&pod)
}

// PodIsControlledByLatestRevision checks if the pod is controlled by latest controller revision.
func PodIsControlledByLatestRevision(pod *corev1.Pod, sts *appsv1.StatefulSet) bool {
	return GetPodRevision(pod) == sts.Status.UpdateRevision && sts.Status.ObservedGeneration == sts.Generation
}
