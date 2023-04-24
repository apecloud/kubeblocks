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

package util

import (
	"context"
	"regexp"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/constant"
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

// IsStsAndPodsRevisionConsistent checks if StatefulSet and pods of the StatefulSet have the same revision.
func IsStsAndPodsRevisionConsistent(ctx context.Context, cli client.Client, sts *appsv1.StatefulSet) (bool, error) {
	pods, err := GetPodListByStatefulSet(ctx, cli, sts)
	if err != nil {
		return false, err
	}

	revisionConsistent := true
	if len(pods) != int(*sts.Spec.Replicas) {
		return false, nil
	}

	for _, pod := range pods {
		if intctrlutil.GetPodRevision(&pod) != sts.Status.UpdateRevision {
			revisionConsistent = false
			break
		}
	}
	return revisionConsistent, nil
}

// GetPods4Delete gets all pods for delete
func GetPods4Delete(ctx context.Context, cli client.Client, sts *appsv1.StatefulSet) ([]*corev1.Pod, error) {
	if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType {
		return nil, nil
	}

	pods, err := GetPodListByStatefulSet(ctx, cli, sts)
	if err != nil {
		return nil, nil
	}

	podList := make([]*corev1.Pod, 0)
	for i, pod := range pods {
		// do nothing if the pod is terminating
		if pod.DeletionTimestamp != nil {
			continue
		}
		// do nothing if the pod has the latest version
		if intctrlutil.GetPodRevision(&pod) == sts.Status.UpdateRevision {
			continue
		}

		podList = append(podList, &pods[i])
	}
	return podList, nil
}

// DeleteStsPods deletes pods of the StatefulSet manually
func DeleteStsPods(ctx context.Context, cli client.Client, sts *appsv1.StatefulSet) error {
	pods, err := GetPods4Delete(ctx, cli, sts)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		// delete the pod to trigger associate StatefulSet to re-create it
		if err := cli.Delete(ctx, pod); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// StatefulSetOfComponentIsReady checks if statefulSet of component is ready.
func StatefulSetOfComponentIsReady(sts *appsv1.StatefulSet, statefulStatusRevisionIsEquals bool, targetReplicas *int32) bool {
	if targetReplicas == nil {
		targetReplicas = sts.Spec.Replicas
	}
	return StatefulSetPodsAreReady(sts, *targetReplicas) && statefulStatusRevisionIsEquals
}

// StatefulSetPodsAreReady checks if all pods of statefulSet are ready.
func StatefulSetPodsAreReady(sts *appsv1.StatefulSet, targetReplicas int32) bool {
	return sts.Status.AvailableReplicas == targetReplicas &&
		sts.Status.Replicas == targetReplicas &&
		sts.Status.ObservedGeneration == sts.Generation
}

func ConvertToStatefulSet(obj client.Object) *appsv1.StatefulSet {
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
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: stsObj.Namespace},
		client.MatchingLabels{
			constant.KBAppComponentLabelKey: stsObj.Labels[constant.KBAppComponentLabelKey],
			constant.AppInstanceLabelKey:    stsObj.Labels[constant.AppInstanceLabelKey],
		}); err != nil {
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

// GetPodOwnerReferencesSts gets the owner reference statefulSet of the pod.
func GetPodOwnerReferencesSts(ctx context.Context, cli client.Client, podObj *corev1.Pod) (*appsv1.StatefulSet, error) {
	stsList := &appsv1.StatefulSetList{}
	if err := cli.List(ctx, stsList,
		&client.ListOptions{Namespace: podObj.Namespace},
		client.MatchingLabels{
			constant.KBAppComponentLabelKey: podObj.Labels[constant.KBAppComponentLabelKey],
			constant.AppInstanceLabelKey:    podObj.Labels[constant.AppInstanceLabelKey],
		}); err != nil {
		return nil, err
	}
	for _, sts := range stsList.Items {
		if IsMemberOf(&sts, podObj) {
			return &sts, nil
		}
	}
	return nil, nil
}

// MarkPrimaryStsToReconcile marks the primary statefulSet annotation to be reconciled.
func MarkPrimaryStsToReconcile(ctx context.Context, cli client.Client, sts *appsv1.StatefulSet) error {
	patch := client.MergeFrom(sts.DeepCopy())
	if sts.Annotations == nil {
		sts.Annotations = map[string]string{}
	}
	sts.Annotations[constant.ReconcileAnnotationKey] = time.Now().Format(time.RFC3339Nano)
	return cli.Patch(ctx, sts, patch)
}
