/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"strconv"

	apps "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		// use rsm's generation instead of sts's
		rsm.Status.ObservedGeneration = rsm.Generation
	case model.IsObjectStatusUpdating(rsmOrig):
		// read the underlying sts
		sts := &apps.StatefulSet{}
		err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(rsm), sts)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		// is managing sts if the underlying sts exists
		if err == nil {
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
			setMembersStatus(rsm, &pods)
		} else {
			// 1. get all pods
			selector, err := metav1.LabelSelectorAsSelector(rsm.Spec.Selector)
			if err != nil {
				return err
			}
			ml := client.MatchingLabelsSelector{Selector: selector}
			podList := &corev1.PodList{}
			if err = transCtx.Client.List(transCtx.Context, podList, ml); err != nil {
				return err
			}
			// 2. calculate status summary
			// the key is how to know pod is updated.
			// proposal 1:
			// build a pod.name to revision map, store it in status.currentRevisions and status.updatedRevisions.
			// keep the status.currentRevision and status.updatedRevision as the last template's revision.
			//
			// proposal 2:
			// build a pod.name to revision map, store it in a cm. set currentRevision and updatedRevision as the name of the cm.
			//
			// proposal 3:
			// patch updated revision to the outdated pod as a label
			//
			// proposal 1 is used currently.
			rsm.Status.Replicas = int32(len(podList.Items))
			currentReplicas, updatedReplicas := int32(0), int32(0)
			readyReplicas, availableReplicas := int32(0), int32(0)
			for _, pod := range podList.Items {
				switch revision, ok := rsm.Status.UpdateRevisions[pod.Name]; {
				case !ok:
					currentReplicas++
				case revision != pod.Labels[apps.ControllerRevisionHashLabelKey]:
					currentReplicas++
				default:
					updatedReplicas++
				}
				switch {
				case controllerutil.IsAvailable(&pod, rsm.Spec.MinReadySeconds):
					availableReplicas++
					readyReplicas++
				case podutils.IsPodReady(&pod):
					readyReplicas++
				}
			}
			rsm.Status.ReadyReplicas = readyReplicas
			rsm.Status.AvailableReplicas = availableReplicas
			rsm.Status.CurrentReplicas = currentReplicas
			rsm.Status.UpdatedReplicas = updatedReplicas
			rsm.Status.CurrentGeneration = rsm.Generation
			// all pods have been updated
			if currentReplicas == 0 {
				rsm.Status.CurrentReplicas = rsm.Status.UpdatedReplicas
				rsm.Status.CurrentRevisions = rsm.Status.UpdateRevisions
				rsm.Status.CurrentRevision = rsm.Status.UpdateRevision
			}

			// 3. set members status
			setMembersStatus(rsm, &podList.Items)
		}
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Status(dag, rsmOrig, rsm)

	return nil
}
