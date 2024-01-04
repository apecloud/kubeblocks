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

package rsm

import (
	"strconv"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// ObjectStatusTransformer computes the current status:
// 1. read the underlying sts's status and copy them to the primary object's status
// 2. read pod role label and update the primary object's status role fields
type ObjectStatusTransformer struct{}

var _ graph.Transformer = &ObjectStatusTransformer{}

func (t *ObjectStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rsmTransformContext)
	rsm := transCtx.rsm
	rsmOrig := transCtx.rsmOrig

	// fast return
	if model.IsObjectDeleting(rsmOrig) {
		return nil
	}

	switch {
	case model.IsObjectUpdating(rsmOrig):
		// use rsm's generation wo of sts's
		rsm.Status.ObservedGeneration = rsm.Generation
	case model.IsObjectStatusUpdating(rsmOrig):
		if rsm.Spec.RsmTransformPolicy == v1alpha1.ToPod {
			ml := GetPodsLabels(rsm.Labels)
			pods := &corev1.PodList{}
			if err := transCtx.Client.List(transCtx, pods, client.InNamespace(rsm.Namespace), ml); err != nil {
				return err
			}
			fliteredPods := FilterActivePods(pods.Items)
			rsm.Status.Replicas = int32(len(fliteredPods))
			readyReplicasCount, availableReplicasCount := calculateStatus(fliteredPods)
			rsm.Status.ReadyReplicas = int32(readyReplicasCount)
			rsm.Status.AvailableReplicas = int32(availableReplicasCount)
			rsm.Status.UpdatedReplicas = rsm.Status.Replicas

			// update role fields
			setMembersStatus(rsm, pods.Items)
		} else {
			// read the underlying sts
			sts := &apps.StatefulSet{}
			if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(rsm), sts); err != nil {
				return err
			}
			// keep rsm's ObservedGeneration to avoid override by sts's ObservedGeneration
			generation := rsm.Status.ObservedGeneration
			rsm.Status.StatefulSetStatus = sts.Status
			rsm.Status.ObservedGeneration = generation
			if currentGenerationLabel, ok := sts.Labels[rsmGenerationLabelKey]; ok {
				currentGeneration, err := strconv.ParseInt(currentGenerationLabel, 10, 64)
				if err != nil {
					return err
				}
				rsm.Status.CurrentGeneration = currentGeneration
			}
			// read all pods belong to the sts, hence belong to the rsm
			pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, sts)
			if err != nil {
				return err
			}
			// update role fields
			setMembersStatus(rsm, pods)
		}
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Status(dag, rsmOrig, rsm)

	return nil
}

func calculateStatus(pods []*corev1.Pod) (int, int) {
	readyReplicasCount := 0
	availableReplicasCount := 0
	for _, pod := range pods {
		if podutils.IsPodReady(pod) {
			readyReplicasCount++
			if podutils.IsPodAvailable(pod, 0, metav1.Now()) {
				availableReplicasCount++
			}
		}
	}
	return readyReplicasCount, availableReplicasCount
}

// FilterActivePods returns pods that have not terminated.
func FilterActivePods(pods []corev1.Pod) []*corev1.Pod {
	var result []*corev1.Pod
	for _, p := range pods {
		if IsPodActive(&p) {
			result = append(result, &p)
		}
	}
	return result
}
func IsPodActive(p *corev1.Pod) bool {
	return corev1.PodSucceeded != p.Status.Phase &&
		corev1.PodFailed != p.Status.Phase &&
		p.DeletionTimestamp == nil
}

func GetPodsLabels(labels map[string]string) client.MatchingLabels {
	ml := client.MatchingLabels{}
	for key, val := range labels {
		ml[key] = val
	}
	return ml
}
