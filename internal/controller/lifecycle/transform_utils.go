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

package lifecycle

import (
	"time"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func newRequeueError(after time.Duration, reason string) error {
	return &realRequeueError{
		reason:       reason,
		requeueAfter: after,
	}
}

func isClusterDeleting(cluster appsv1alpha1.Cluster) bool {
	return !cluster.GetDeletionTimestamp().IsZero()
}

func isClusterUpdating(cluster appsv1alpha1.Cluster) bool {
	return cluster.Status.ObservedGeneration != cluster.Generation
}

func isClusterStatusUpdating(cluster appsv1alpha1.Cluster) bool {
	return !isClusterDeleting(cluster) && !isClusterUpdating(cluster)
	// return cluster.Status.ObservedGeneration == cluster.Generation &&
	//	slices.Contains(appsv1alpha1.GetClusterTerminalPhases(), cluster.Status.Phase)
}
