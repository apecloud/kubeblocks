/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	opsManagerOnce sync.Once
	opsManager     *OpsManager
)

// RegisterOps registers operation with OpsType and OpsBehaviour
func (opsMgr *OpsManager) RegisterOps(opsType opsv1alpha1.OpsType, opsBehaviour OpsBehaviour) {
	opsManager.OpsMap[opsType] = opsBehaviour
	opsv1alpha1.OpsRequestBehaviourMapper[opsType] = opsv1alpha1.OpsRequestBehaviour{
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

	if opsRequest.Spec.Type == opsv1alpha1.CustomType {
		err = initOpsDefAndValidate(reqCtx, cli, opsRes)
	} else {
		// validate OpsRequest.spec
		err = opsRequest.ValidateOps(reqCtx.Ctx, cli, opsRes.Cluster)
	}
	if err != nil {
		return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
	}

	if opsRequest.Status.Phase == opsv1alpha1.OpsPendingPhase {
		if opsRequest.Spec.Cancel {
			return &ctrl.Result{}, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsv1alpha1.OpsCancelledPhase)
		}
		if err = opsMgr.doPreConditionAndTransPhaseToCreating(reqCtx, cli, opsRes, opsBehaviour); intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			return &ctrl.Result{}, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
		} else if err != nil {
			if _, ok := err.(*WaitForClusterPhaseErr); ok {
				return intctrlutil.ResultToP(intctrlutil.RequeueAfter(time.Second, reqCtx.Log, "wait cluster to a right phase"))
			}
			return nil, err
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
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

func (opsMgr *OpsManager) doPreConditionAndTransPhaseToCreating(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsBehaviour OpsBehaviour) error {
	if opsBehaviour.QueueByCluster || opsBehaviour.QueueBySelf {
		// if ToClusterPhase is not empty, enqueue OpsRequest to the cluster Annotation.
		opsRecorde, err := enqueueOpsRequestToClusterAnnotation(reqCtx.Ctx, cli, opsRes, opsBehaviour)
		if err != nil {
			return err
		}
		if opsRecorde != nil && opsRecorde.InQueue {
			// if the opsRequest is in the queue, return
			return nil
		}
	}
	// validate if the dependent ops have been successful
	pass, err := opsMgr.validateDependOnSuccessfulOps(reqCtx, cli, opsRes)
	if err != nil || !pass {
		return err
	}
	if preConditionDeadlineSecondsIsSet(opsRes.OpsRequest) &&
		opsRes.OpsRequest.Annotations[constant.QueueEndTimeAnnotationKey] == "" {
		// set the queue end time for preConditionDeadline validation
		if opsRes.OpsRequest.Annotations == nil {
			opsRes.OpsRequest.Annotations = map[string]string{}
		}
		opsRes.OpsRequest.Annotations[constant.QueueEndTimeAnnotationKey] = time.Now().Format(time.RFC3339)
		return cli.Update(reqCtx.Ctx, opsRes.OpsRequest)
	}
	// if the operation will create a new cluster, don't validate the cluster phase
	if !opsBehaviour.IsClusterCreation {
		// validate entry condition for OpsRequest, check if the cluster is in the right phase
		if err = validateOpsNeedWaitingClusterPhase(opsRes.Cluster, opsRes.OpsRequest, opsBehaviour); err != nil {
			return err
		}
		if err = opsRes.OpsRequest.ValidateClusterPhase(opsRes.Cluster); err != nil {
			return intctrlutil.NewFatalError(err.Error())
		}
	}
	opsDeepCopy := opsRes.OpsRequest.DeepCopy()
	// save last configuration into status.lastConfiguration
	if err = opsBehaviour.OpsHandler.SaveLastConfiguration(reqCtx, cli, opsRes); err != nil {
		return err
	}
	return patchOpsRequestToCreating(reqCtx, cli, opsRes, opsDeepCopy, opsBehaviour.OpsHandler)
}

// Reconcile entry function when OpsRequest.status.phase is Running.
// loops till the operation is completed.
func (opsMgr *OpsManager) Reconcile(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (time.Duration, error) {
	var (
		opsBehaviour    OpsBehaviour
		ok              bool
		err             error
		requeueAfter    time.Duration
		opsRequestPhase opsv1alpha1.OpsPhase
		opsRequest      = opsRes.OpsRequest
	)

	if opsBehaviour, ok = opsMgr.OpsMap[opsRes.OpsRequest.Spec.Type]; !ok || opsBehaviour.OpsHandler == nil {
		return 0, PatchOpsHandlerNotSupported(reqCtx.Ctx, cli, opsRes)
	}
	opsRes.ToClusterPhase = opsBehaviour.ToClusterPhase
	if opsRequest.Spec.Type == opsv1alpha1.CustomType {
		err = initOpsDefAndValidate(reqCtx, cli, opsRes)
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			return requeueAfter, patchValidateErrorCondition(reqCtx.Ctx, cli, opsRes, err.Error())
		}
		if err != nil {
			return requeueAfter, err
		}
	}
	if opsRequestPhase, requeueAfter, err = opsBehaviour.OpsHandler.ReconcileAction(reqCtx, cli, opsRes); err != nil &&
		!isOpsRequestFailedPhase(opsRequestPhase) {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			return requeueAfter, patchFatalFailErrorCondition(reqCtx.Ctx, cli, opsRes, err)
		}
		// if the opsRequest phase is not failed, skipped
		return requeueAfter, err
	}
	switch opsRequestPhase {
	case opsv1alpha1.OpsSucceedPhase:
		return 0, opsMgr.handleOpsCompleted(reqCtx, cli, opsRes, opsRequestPhase,
			opsv1alpha1.NewCancelSucceedCondition(opsRequest.Name), opsv1alpha1.NewSucceedCondition(opsRequest))
	case opsv1alpha1.OpsFailedPhase:
		return 0, opsMgr.handleOpsCompleted(reqCtx, cli, opsRes, opsRequestPhase,
			opsv1alpha1.NewCancelFailedCondition(opsRequest, err), opsv1alpha1.NewFailedCondition(opsRequest, err))
	default:
		return opsMgr.checkAndHandleOpsTimeout(reqCtx, cli, opsRes, requeueAfter)
	}
}

func (opsMgr *OpsManager) handleOpsCompleted(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsRequestPhase opsv1alpha1.OpsPhase,
	cancelledCondition,
	completedCondition *metav1.Condition) error {
	if err := updateHAConfigIfNecessary(reqCtx, cli, opsRes.OpsRequest, "true"); err != nil {
		return err
	}
	if opsRes.OpsRequest.Status.Phase == opsv1alpha1.OpsCancellingPhase {
		return PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsv1alpha1.OpsCancelledPhase, cancelledCondition)
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
		ops := &opsv1alpha1.OpsRequest{}
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
		if slices.Contains([]opsv1alpha1.OpsPhase{opsv1alpha1.OpsFailedPhase, opsv1alpha1.OpsCancelledPhase, opsv1alpha1.OpsAbortedPhase}, ops.Status.Phase) {
			return false, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsv1alpha1.OpsCancelledPhase)
		}
		if ops.Status.Phase != opsv1alpha1.OpsSucceedPhase {
			return false, nil
		}
	}
	return true, nil
}

// handleOpsIsRunningTimedOut handles if the opsRequest is timed out.
func (opsMgr *OpsManager) checkAndHandleOpsTimeout(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	requeueAfter time.Duration) (time.Duration, error) {
	timeoutSeconds := opsRes.OpsRequest.Spec.TimeoutSeconds
	if timeoutSeconds == nil || *timeoutSeconds == 0 {
		return requeueAfter, nil
	}
	timeoutPoint := opsRes.OpsRequest.Status.StartTimestamp.Add(time.Duration(*timeoutSeconds) * time.Second)
	if !time.Now().Before(timeoutPoint) {
		return 0, PatchOpsStatus(reqCtx.Ctx, cli, opsRes, opsv1alpha1.OpsAbortedPhase,
			opsv1alpha1.NewAbortedCondition("Aborted due to exceeding the specified timeout period (timeoutSeconds)"))
	}
	if requeueAfter != 0 {
		return requeueAfter, nil
	}
	return time.Until(timeoutPoint), nil
}

func GetOpsManager() *OpsManager {
	opsManagerOnce.Do(func() {
		opsManager = &OpsManager{OpsMap: make(map[opsv1alpha1.OpsType]OpsBehaviour)}
	})
	return opsManager
}
