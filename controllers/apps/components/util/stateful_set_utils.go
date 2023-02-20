/*
Copyright ApeCloud, Inc.
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
	"context"
	"regexp"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// DescendingOrdinalSts is a sort.Interface that Sorts a list of StatefulSet based on the ordinals extracted from the statefulSet.
type DescendingOrdinalSts []*appsv1.StatefulSet

// statefulSetRegex is a regular expression that extracts StatefulSet's ordinal from the Name of StatefulSet
var statefulSetRegex = regexp.MustCompile("(.*)-([0-9]+)$")

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

// IsStsAndPodsRevisionConsistent checks if StatefulSet and pods of the StatefuleSet have the same revison,
func IsStsAndPodsRevisionConsistent(ctx context.Context, cli client.Client, sts *appsv1.StatefulSet) (bool, error) {
	pods, err := GetPodListByStatefulSet(ctx, cli, sts)
	if err != nil {
		return false, err
	}
	revisionConsistent := true
	for _, pod := range pods {
		if GetPodRevision(&pod) != sts.Status.UpdateRevision {
			revisionConsistent = false
			break
		}
	}
	return revisionConsistent, nil
}

// StatefulSetOfComponentIsReady checks if statefulSet of component is ready.
func StatefulSetOfComponentIsReady(sts *appsv1.StatefulSet, statefulStatusRevisionIsEquals bool, targetReplicas *int32) bool {
	if targetReplicas == nil {
		targetReplicas = sts.Spec.Replicas
	}
	// judge whether statefulSet is ready
	return StatefulSetPodsAreReady(sts, *targetReplicas) && statefulStatusRevisionIsEquals
}

// StatefulSetPodsAreReady checks if all pods of statefulSet are ready.
func StatefulSetPodsAreReady(sts *appsv1.StatefulSet, targetReplicas int32) bool {
	return sts.Status.AvailableReplicas == targetReplicas &&
		sts.Status.Replicas == targetReplicas &&
		sts.Status.ObservedGeneration == sts.Generation
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

// Len is the implementation of the sort.Interface, calculate the length of the list of DescendingOrdinalSts.
func (dos DescendingOrdinalSts) Len() int {
	return len(dos)
}

// Swap is the implementation of the sort.Interface, exchange two items in DescendingOrdinalSts.
func (dos DescendingOrdinalSts) Swap(i, j int) {
	dos[i], dos[j] = dos[j], dos[i]
}

// Less is the implementation of the sort.Interface, sort the size of the statefulSet ordinal in descending order.
func (dos DescendingOrdinalSts) Less(i, j int) bool {
	return GetOrdinalSts(dos[i]) > GetOrdinalSts(dos[j])
}

// GetOrdinalSts gets StatefulSet's ordinal. If StatefulSet has no ordinal, -1 is returned.
func GetOrdinalSts(sts *appsv1.StatefulSet) int {
	_, ordinal := getParentNameAndOrdinalSts(sts)
	return ordinal
}

// getParentNameAndOrdinalSts gets the name of cluster-component and StatefulSet's ordinal as extracted from its Name. If
// the StatefulSet's Name was not match a statefulSetRegex, its parent is considered to be empty string,
// and its ordinal is considered to be -1.
func getParentNameAndOrdinalSts(sts *appsv1.StatefulSet) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := statefulSetRegex.FindStringSubmatch(sts.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

// GetPodListByStatefulSet gets statefulSet pod list.
func GetPodListByStatefulSet(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: stsObj.Namespace},
		client.MatchingLabels{intctrlutil.AppComponentLabelKey: stsObj.Labels[intctrlutil.AppComponentLabelKey]}); err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		if IsMemberOf(stsObj, &pod) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}
