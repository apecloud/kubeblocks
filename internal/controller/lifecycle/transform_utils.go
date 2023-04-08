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

//
//func getGVKName(object client.Object, scheme *runtime.Scheme) (*gvkName, error) {
//	gvk, err := apiutil.GVKForObject(object, scheme)
//	if err != nil {
//		return nil, err
//	}
//	return &gvkName{
//		gvk:  gvk,
//		ns:   object.GetNamespace(),
//		name: object.GetName(),
//	}, nil
//}
//
//func isOwnerOf(owner, obj client.Object, scheme *runtime.Scheme) bool {
//	ro, ok := owner.(runtime.Object)
//	if !ok {
//		return false
//	}
//	gvk, err := apiutil.GVKForObject(ro, scheme)
//	if err != nil {
//		return false
//	}
//	ref := metav1.OwnerReference{
//		APIVersion: gvk.GroupVersion().String(),
//		Kind:       gvk.Kind,
//		UID:        owner.GetUID(),
//		Name:       owner.GetName(),
//	}
//	owners := obj.GetOwnerReferences()
//	referSameObject := func(a, b metav1.OwnerReference) bool {
//		aGV, err := schema.ParseGroupVersion(a.APIVersion)
//		if err != nil {
//			return false
//		}
//
//		bGV, err := schema.ParseGroupVersion(b.APIVersion)
//		if err != nil {
//			return false
//		}
//
//		return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
//	}
//	for _, ownerRef := range owners {
//		if referSameObject(ownerRef, ref) {
//			return true
//		}
//	}
//	return false
//}

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
