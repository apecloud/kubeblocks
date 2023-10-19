/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package common

import (
	"context"
	"regexp"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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

// statefulSetOfComponentIsReady checks if statefulSet of component is ready.
func statefulSetOfComponentIsReady(sts *appsv1.StatefulSet, statefulStatusRevisionIsEquals bool, targetReplicas *int32) bool {
	if targetReplicas == nil {
		targetReplicas = sts.Spec.Replicas
	}
	return statefulSetPodsAreReady(sts, *targetReplicas) && statefulStatusRevisionIsEquals
}

// statefulSetPodsAreReady checks if all pods of statefulSet are ready.
func statefulSetPodsAreReady(sts *appsv1.StatefulSet, targetReplicas int32) bool {
	return sts.Status.AvailableReplicas == targetReplicas &&
		sts.Status.Replicas == targetReplicas &&
		sts.Status.ObservedGeneration == sts.Generation
}

func convertToStatefulSet(obj client.Object) *appsv1.StatefulSet {
	if obj == nil {
		return nil
	}
	if sts, ok := obj.(*appsv1.StatefulSet); ok {
		return sts
	}
	return nil
}

// ParseParentNameAndOrdinal gets the name of cluster-component and StatefulSet's ordinal as extracted from its Name. If
// the StatefulSet's Name was not match a statefulSetRegex, its parent is considered to be empty string,
// and its ordinal is considered to be -1.
func ParseParentNameAndOrdinal(s string) (string, int32) {
	parent := ""
	ordinal := int32(-1)
	subMatches := statefulSetRegex.FindStringSubmatch(s)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int32(i)
	}
	return parent, ordinal
}

// GetPodListByStatefulSet gets statefulSet pod list.
func GetPodListByStatefulSet(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet) ([]corev1.Pod, error) {
	selector, err := metav1.LabelSelectorAsMap(stsObj.Spec.Selector)
	if err != nil {
		return nil, err
	}
	return GetPodListByStatefulSetWithSelector(ctx, cli, stsObj, selector)
}

// GetPodListByStatefulSetWithSelector gets statefulSet pod list.
func GetPodListByStatefulSetWithSelector(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet, selector client.MatchingLabels) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: stsObj.Namespace},
		selector); err != nil {
		return nil, err
	}
	var pods []corev1.Pod
	for _, pod := range podList.Items {
		if IsMemberOf(stsObj, &pod) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}
