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
	"slices"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	opsManagerOnce sync.Once
	opsManager     *OpsManager
)

// RegisterOps registers operation with OpsType and OpsBehaviour
func (opsMgr *OpsManager) RegisterOps(opsType appsv1alpha1.OpsType, opsBehaviour OpsBehaviour) {
	opsManager.OpsMap[opsType] = opsBehaviour
	appsv1alpha1.OpsRequestBehaviourMapper[opsType] = appsv1alpha1.OpsRequestBehaviour{
		FromClusterPhases: opsBehaviour.FromClusterPhases,
		ToClusterPhase:    opsBehaviour.ToClusterPhase,
	}
}

// Do the entry function for handling OpsRequest
func (opsMgr *OpsManager) Do(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*ctrl.Result, error) {
	var (
		opsBehaviour OpsBehaviour
		err          error
		ok           bool
		opsRequest   = opsRes.OpsRequest
	)
	if opsBehaviour, ok = opsMgr.OpsMap[opsRequest.Spec.Type]; !ok || opsBehaviour.OpsHandler == nil {
		return &ctrl.Result{}, PatchOpsHandlerNotSupported(reqCtx.Ctx, cli, opsRes)
	}

	if opsRequest.Spec.Type == appsv1alpha1.CustomType {
		err = initOpsDefAndValidate(reqCtx, cli, opsRes)
		if err != nil {
			return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
		}
	} else {
		// validate OpsRequest.spec
		// if the operation will create a new cluster, don't validate the cluster
		if err = opsRequest.Validate(reqCtx.Ctx, cli, opsRes.Cluster, !opsBehaviour.IsClusterCreation); err != nil {
			return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
		}
	}

	if opsRequest.Status.Phase == appsv1alpha1.OpsPendingPhase {
		if opsRequest.Spec.Cancel {
			return &ctrl.Result{}, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, appsv1alpha1.OpsCancelledPhase)
		}
		// validate entry condition for OpsRequest, check if the cluster is in the right phase
		if err = validateOpsWaitingPhase(opsRes.Cluster, opsRequest, opsBehaviour); err != nil {
			// check if the error is caused by WaitForClusterPhaseErr  error
			if _, ok := err.(*WaitForClusterPhaseErr); ok {
				return intctrlutil.ResultToP(intctrlutil.RequeueAfter(time.Second, reqCtx.Log, ""))
			}
			return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
		}
		// TODO: abort last OpsRequest if using 'force' and intersecting with cluster component name or shard name.
		if opsBehaviour.QueueByCluster || opsBehaviour.QueueBySelf {
			// if ToClusterPhase is not empty, enqueue OpsRequest to the cluster Annotation.
			opsRecorde, err := enqueueOpsRequestToClusterAnnotation(reqCtx.Ctx, cli, opsRes, opsBehaviour)
			if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
			} else if err != nil {
				return nil, err
			}
			if opsRecorde != nil && opsRecorde.InQueue {
				// if the opsRequest is in the queue, return
				return intctrlutil.ResultToP(intctrlutil.Reconciled())
			}
		}

		// validate if the dependent ops have been successful
		if pass, err := opsMgr.validateDependOnSuccessfulOps(reqCtx, cli, opsRes); intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
		} else if err != nil {
			return nil, err
		} else if !pass {
			return intctrlutil.ResultToP(intctrlutil.Reconciled())
		}
		opsDeepCopy := opsRequest.DeepCopy()
		// save last configuration into status.lastConfiguration
		if err = opsBehaviour.OpsHandler.SaveLastConfiguration(reqCtx, cli, opsRes); err != nil {
			return nil, err
		}

		return &ctrl.Result{}, patchOpsRequestToCreating(reqCtx, cli, opsRes, opsDeepCopy, opsBehaviour.OpsHandler)
	}

	if err = updateHAConfigIfNecessary(reqCtx, cli, opsRes.OpsRequest, "false"); err != nil {
		return nil, err
	}
	if err = opsBehaviour.OpsHandler.Action(reqCtx, cli, opsRes); err != nil {
		// patch the status.phase to Failed when the error is Fatal, which means the operation is failed and there is no need to retry
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			return &ctrl.Result{}, patchFatalFailErrorCondition(reqCtx.Ctx, cli, opsRes, err)
		}
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting) {
			return intctrlutil.ResultToP(intctrlutil.Reconciled())
		}
		return nil, err
	}
	return nil, nil
}

// Reconcile entry function when OpsRequest.status.phase is Running.
// loops till the operation is completed.
func (opsMgr *OpsManager) Reconcile(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (time.Duration, error) {
	var (
		opsBehaviour    OpsBehaviour
		ok              bool
		err             error
		requeueAfter    time.Duration
		opsRequestPhase appsv1alpha1.OpsPhase
		opsRequest      = opsRes.OpsRequest
	)

	if opsBehaviour, ok = opsMgr.OpsMap[opsRes.OpsRequest.Spec.Type]; !ok || opsBehaviour.OpsHandler == nil {
		return 0, PatchOpsHandlerNotSupported(reqCtx.Ctx, cli, opsRes)
	}
	opsRes.ToClusterPhase = opsBehaviour.ToClusterPhase
	if opsRequest.Spec.Type == appsv1alpha1.CustomType {
		err = initOpsDefAndValidate(reqCtx, cli, opsRes)
		if err != nil {
			return requeueAfter, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
		}
	}
	if opsRequestPhase, requeueAfter, err = opsBehaviour.OpsHandler.ReconcileAction(reqCtx, cli, opsRes); err != nil &&
		!isOpsRequestFailedPhase(opsRequestPhase) {
		// if the opsRequest phase is not failed, skipped
		return requeueAfter, err
	}
	switch opsRequestPhase {
	case appsv1alpha1.OpsSucceedPhase:
		return 0, opsMgr.handleOpsCompleted(reqCtx, cli, opsRes, opsRequestPhase,
			appsv1alpha1.NewCancelSucceedCondition(opsRequest.Name), appsv1alpha1.NewSucceedCondition(opsRequest))
	case appsv1alpha1.OpsFailedPhase:
		return 0, opsMgr.handleOpsCompleted(reqCtx, cli, opsRes, opsRequestPhase,
			appsv1alpha1.NewCancelFailedCondition(opsRequest, err), appsv1alpha1.NewFailedCondition(opsRequest, err))
	default:
		return opsMgr.handleOpsIsRunningTimedOut(reqCtx, cli, opsRes, requeueAfter)
	}
}

func (opsMgr *OpsManager) handleOpsCompleted(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsRequestPhase appsv1alpha1.OpsPhase,
	cancelledCondition,
	completedCondition *metav1.Condition) error {
	if err := updateHAConfigIfNecessary(reqCtx, cli, opsRes.OpsRequest, "true"); err != nil {
		return err
	}
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
		return PatchOpsStatus(reqCtx.Ctx, cli, opsRes, appsv1alpha1.OpsCancelledPhase, cancelledCondition)
	}
	return PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsRequestPhase, completedCondition)
}

// validateDependOnOps validates if the dependent ops have been successful
func (opsMgr *OpsManager) validateDependOnSuccessfulOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) (bool, error) {
	dependentOpsStr := opsRes.OpsRequest.Annotations[constant.OpsDependentOnSuccessfulOpsAnnoKey]
	if dependentOpsStr == "" {
		return true, nil
	}
	opsNames := strings.Split(dependentOpsStr, ",")
	for _, opsName := range opsNames {
		ops := &appsv1alpha1.OpsRequest{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: opsName, Namespace: opsRes.OpsRequest.Namespace}, ops); err != nil {
			if apierrors.IsNotFound(err) {
				return false, intctrlutil.NewFatalError(err.Error())
			}
			return false, err
		}
		var relatedOpsArr []string
		relatedOpsStr := ops.Annotations[constant.RelatedOpsAnnotationKey]
		if relatedOpsStr != "" {
			relatedOpsArr = strings.Split(relatedOpsStr, ",")
		}
		if !slices.Contains(relatedOpsArr, opsRes.OpsRequest.Name) {
			// annotate to the dependent opsRequest
			relatedOpsArr = append(relatedOpsArr, opsRes.OpsRequest.Name)
			if ops.Annotations == nil {
				ops.Annotations = map[string]string{}
			}
			ops.Annotations[constant.RelatedOpsAnnotationKey] = strings.Join(relatedOpsArr, ",")
			if err := cli.Update(reqCtx.Ctx, ops); err != nil {
				return false, err
			}
		}
		if slices.Contains([]appsv1alpha1.OpsPhase{appsv1alpha1.OpsFailedPhase, appsv1alpha1.OpsCancelledPhase, appsv1alpha1.OpsAbortedPhase}, ops.Status.Phase) {
			return false, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, appsv1alpha1.OpsCancelledPhase)
		}
		if ops.Status.Phase != appsv1alpha1.OpsSucceedPhase {
			return false, nil
		}
	}
	return true, nil
}

// handleOpsIsRunningTimedOut handles if the opsRequest is timed out.
func (opsMgr *OpsManager) handleOpsIsRunningTimedOut(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	requeueAfter time.Duration) (time.Duration, error) {
	timeoutSeconds := opsRes.OpsRequest.Spec.TimeoutSeconds
	if timeoutSeconds == nil || *timeoutSeconds == 0 {
		return requeueAfter, nil
	}
	checkTimedOut := func(startTime metav1.Time) (time.Duration, error) {
		timeoutPoint := startTime.Add(time.Duration(*timeoutSeconds))
		if !time.Now().Before(timeoutPoint) {
			return 0, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, appsv1alpha1.OpsAbortedPhase,

				appsv1alpha1.NewAbortedCondition("Aborted due to exceeding the specified timeout period (timeoutSeconds)"))
		}
		if requeueAfter != 0 {
			return requeueAfter, nil
		}
		return time.Until(timeoutPoint), nil
	}
	if !opsRes.OpsRequest.Spec.Cancel {
		return checkTimedOut(opsRes.OpsRequest.Status.StartTimestamp)
	}
	if opsRes.OpsRequest.Status.CancelTimestamp.IsZero() {
		return requeueAfter, nil
	}
	// After canceling opsRequest, recalculate whether the opsRequest runs out of time based on the cancellation time point
	return checkTimedOut(opsRes.OpsRequest.Status.CancelTimestamp)
}

func GetOpsManager() *OpsManager {
	opsManagerOnce.Do(func() {
		opsManager = &OpsManager{OpsMap: make(map[appsv1alpha1.OpsType]OpsBehaviour)}
	})
	return opsManager
}
