/*
Copyright ApeCloud Inc.
Copyright 2016 The Kubernetes Authors.

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// getParentName gets the name of pod's parent StatefulSet. If pod has not parent, the empty string is returned.
func getParentName(pod *corev1.Pod) string {
	parent, _ := intctrlutil.GetParentNameAndOrdinal(pod)
	return parent
}

// IsMemberOf tests if pod is a member of set.
func IsMemberOf(set *appsv1.StatefulSet, pod *corev1.Pod) bool {
	return getParentName(pod) == set.Name
}

// GetPodRevision gets the revision of Pod by inspecting the StatefulSetRevisionLabel. If pod has no revision the empty
// string is returned.
func GetPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[appsv1.StatefulSetRevisionLabel]
}

// StatefulSetIsReady checks if statefulSet is ready.
func StatefulSetIsReady(sts *appsv1.StatefulSet, statefulStatusRevisionIsEquals bool) bool {
	var (
		componentIsRunning = true
		targetReplicas     = *sts.Spec.Replicas
	)
	// judge whether statefulSet is ready
	if sts.Status.AvailableReplicas != targetReplicas ||
		sts.Status.Replicas != targetReplicas ||
		sts.Status.ObservedGeneration != sts.GetGeneration() ||
		!statefulStatusRevisionIsEquals {
		componentIsRunning = false
	}
	return componentIsRunning
}

// StatefulSetPodsIsReady checks if pods of statefulSet are ready.
func StatefulSetPodsIsReady(sts *appsv1.StatefulSet) bool {
	targetReplicas := *sts.Spec.Replicas
	return sts.Status.AvailableReplicas == targetReplicas &&
		sts.Status.Replicas == targetReplicas &&
		sts.Status.ObservedGeneration == sts.GetGeneration()
}

func CovertToStatefulSet(obj client.Object) *appsv1.StatefulSet {
	if obj == nil {
		return nil
	}
	if sts, ok := obj.(*appsv1.StatefulSet); ok {
		return sts
	}
	return nil
}
