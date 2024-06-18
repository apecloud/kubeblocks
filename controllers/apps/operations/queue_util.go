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

package operations

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const opsRequestQueueLimitSize = 20

// DequeueOpsRequestInClusterAnnotation when OpsRequest.status.phase is Succeeded or Failed
// we should remove the OpsRequest Annotation of cluster, then unlock cluster
func DequeueOpsRequestInClusterAnnotation(ctx context.Context, cli client.Client, opsRes *OpsResource) error {
	var (
		opsRequestSlice []appsv1alpha1.OpsRecorder
		err             error
	)
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster); err != nil {
		return err
	}
	index, _ := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name)
	if index == -1 {
		return nil
	}
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsFailedPhase && index == 0 {
		var newOpsRequestSlice []appsv1alpha1.OpsRecorder
		// 1. update all pending opsRequest phase to Cancelled if the head opsRequest is Failed.
		for i := 1; i < len(opsRequestSlice); i++ {
			if !opsRequestSlice[i].InQueue {
				// ignore the running opsRequests.
				newOpsRequestSlice = append(newOpsRequestSlice, opsRequestSlice[i])
				continue
			}
			ops := &appsv1alpha1.OpsRequest{}
			if err = cli.Get(ctx, client.ObjectKey{Name: opsRequestSlice[i].Name, Namespace: opsRes.OpsRequest.Namespace}, ops); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
			patch := client.MergeFrom(ops.DeepCopy())
			ops.Status.Phase = appsv1alpha1.OpsCancelledPhase
			ops.Status.CompletionTimestamp = metav1.Time{Time: time.Now()}
			ops.SetStatusCondition(metav1.Condition{
				Type:    appsv1alpha1.ConditionTypeCancelled,
				Reason:  appsv1alpha1.ReasonOpsCancelByController,
				Status:  metav1.ConditionTrue,
				Message: fmt.Sprintf(`Cancelled by controller due to the failure of previous OpsRequest "%s"`, opsRes.OpsRequest.Name),
			})
			if err = cli.Status().Patch(ctx, ops, patch); err != nil && apierrors.IsNotFound(err) {
				return err
			}
		}
		// 2. cleanup opsRequest queue
		opsRequestSlice = newOpsRequestSlice
	} else {
		// delete the opsRequest in Cluster.annotations
		opsRequestSlice = slices.Delete(opsRequestSlice, index, index+1)
	}
	return opsutil.UpdateClusterOpsAnnotations(ctx, cli, opsRes.Cluster, opsRequestSlice)
}

// enqueueOpsRequestToClusterAnnotation adds the OpsRequest Annotation to Cluster.metadata.Annotations to acquire the lock.
func enqueueOpsRequestToClusterAnnotation(ctx context.Context, cli client.Client, opsRes *OpsResource, opsBehaviour OpsBehaviour) (*appsv1alpha1.OpsRecorder, error) {
	var (
		opsRequestSlice []appsv1alpha1.OpsRecorder
		err             error
	)
	if !opsBehaviour.QueueByCluster && !opsBehaviour.QueueBySelf {
		return nil, nil
	}
	// if the running opsRequest is deleted, do not enqueue the opsRequest to cluster annotation.
	if !opsRes.OpsRequest.DeletionTimestamp.IsZero() {
		return nil, nil
	}
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster); err != nil {
		return nil, err
	}

	index, opsRecorder := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name)
	switch index {
	case -1:
		// if not exists but reach the queue limit size, throw an error
		if len(opsRequestSlice) >= opsRequestQueueLimitSize {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf("The opsRequest queue is limited to a size of %d", opsRequestQueueLimitSize))
		}
		// if not exists, enqueue
		if opsRequestSlice == nil {
			opsRequestSlice = make([]appsv1alpha1.OpsRecorder, 0)
		}
		opsRecorder = appsv1alpha1.OpsRecorder{
			Name:        opsRes.OpsRequest.Name,
			Type:        opsRes.OpsRequest.Spec.Type,
			QueueBySelf: opsBehaviour.QueueBySelf,
			// check if the opsRequest should be in the queue.
			InQueue: existOtherRunningOps(opsRequestSlice, opsRes.OpsRequest.Spec.Type, opsBehaviour) && !opsRes.OpsRequest.Force(),
		}
		opsRequestSlice = append(opsRequestSlice, opsRecorder)
	default:
		if !opsRecorder.InQueue {
			// the opsRequest is already running.
			return &opsRecorder, nil
		}
		if existOtherRunningOps(opsRequestSlice, opsRecorder.Type, opsBehaviour) {
			// if exists other running opsRequest, return.
			return &opsRecorder, nil
		}
		// mark to handle the next opsRequest
		opsRequestSlice[index].InQueue = false
	}
	return &opsRecorder, opsutil.UpdateClusterOpsAnnotations(ctx, cli, opsRes.Cluster, opsRequestSlice)
}

// existOtherRunningOps checks if exists other running opsRequest.
func existOtherRunningOps(opsRecorderSlice []appsv1alpha1.OpsRecorder, opsType appsv1alpha1.OpsType, opsBehaviour OpsBehaviour) bool {
	for i := range opsRecorderSlice {
		if opsBehaviour.QueueByCluster && opsRecorderSlice[i].QueueBySelf {
			continue
		}
		if opsBehaviour.QueueBySelf && opsRecorderSlice[i].Type != opsType {
			continue
		}
		if !opsRecorderSlice[i].InQueue {
			return true
		}
	}
	return false
}
