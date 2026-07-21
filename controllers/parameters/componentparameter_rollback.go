/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package parameters

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func applyRollbackMetadata(owner client.Object, configMap *corev1.ConfigMap, targetRevision string) error {
	compParam, ok := owner.(*parametersv1alpha1.ComponentParameter)
	if !ok || compParam.Spec.Rollback == nil {
		return nil
	}
	if configMap.Annotations == nil {
		configMap.Annotations = map[string]string{}
	}
	request := compParam.Spec.Rollback
	sourceRevision := strconv.FormatInt(request.SourceGeneration, 10)
	if gate, exists := configMap.Annotations[constant.DisableUpgradeInsConfigurationAnnotationKey]; exists {
		if gate != strconv.FormatBool(true) ||
			configMap.Annotations[constant.ReconfigureFailureRevisionAnnotationKey] != sourceRevision {
			return fmt.Errorf("ConfigMap %s does not carry the Parameters-owned failure gate for source revision %s",
				configMap.Name, sourceRevision)
		}
		delete(configMap.Annotations, constant.DisableUpgradeInsConfigurationAnnotationKey)
		delete(configMap.Annotations, constant.ReconfigureFailureRevisionAnnotationKey)
	} else if _, exists := configMap.Annotations[constant.ReconfigureFailureRevisionAnnotationKey]; exists {
		return fmt.Errorf("ConfigMap %s carries a failure revision without its Parameters-owned gate", configMap.Name)
	}
	configMap.Annotations[constant.ParameterRollbackRevisionAnnotationKey] = targetRevision
	configMap.Annotations[constant.ParameterRollbackRestartAnnotationKey] = strconv.FormatBool(request.Restart)
	return nil
}

func (r *ComponentParameterReconciler) reconcileRollback(reqCtx intctrlutil.RequestCtx,
	compParam *parametersv1alpha1.ComponentParameter) (bool, error) {
	request := compParam.Spec.Rollback
	if request == nil {
		return false, nil
	}

	status := compParam.Status.Rollback
	if status != nil && status.RequestID == request.RequestID &&
		(status.Phase == parametersv1alpha1.ParameterRollbackSucceeded ||
			status.Phase == parametersv1alpha1.ParameterRollbackFailed) {
		changed, err := r.cleanupRollbackMarkers(reqCtx, compParam)
		if err != nil || changed {
			return changed, err
		}
		patch := client.MergeFromWithOptions(compParam.DeepCopy(), client.MergeFromWithOptimisticLock{})
		compParam.Spec.Rollback = nil
		return true, r.Client.Patch(reqCtx.Ctx, compParam, patch)
	}

	if request.RequestID == "" || request.SourceGeneration < 1 {
		return r.failRollback(reqCtx, compParam, request.RequestID, "rollback request identity is incomplete")
	}
	if compParam.Annotations[constant.OpsRequestUIDAnnotationKey] != request.RequestID {
		return r.failRollback(reqCtx, compParam, request.RequestID, "rollback request does not own this ComponentParameter")
	}
	requestHash, err := rollbackRequestHash(request)
	if err != nil {
		return false, err
	}
	if status != nil && status.RequestID == request.RequestID {
		if status.RequestHash == "" {
			updated := *status
			updated.RequestHash = requestHash
			return r.patchRollbackStatus(reqCtx, compParam, &updated)
		}
		if status.RequestHash != requestHash {
			return r.failRollback(reqCtx, compParam, request.RequestID,
				"rollback request changed after it was accepted")
		}
	}

	if status == nil || status.RequestID != request.RequestID {
		if compParam.Generation != request.SourceGeneration+1 ||
			compParam.Status.ObservedGeneration != request.SourceGeneration ||
			!isRollbackSourceFailure(compParam.Status.Phase) {
			return r.failRollback(reqCtx, compParam, request.RequestID,
				"ComponentParameter changed before rollback intent could be accepted")
		}
		return r.patchRollbackStatus(reqCtx, compParam, &parametersv1alpha1.ParameterRollbackStatus{
			RequestID:   request.RequestID,
			RequestHash: requestHash,
			Phase:       parametersv1alpha1.ParameterRollbackPending,
			Message:     "rollback intent accepted",
		})
	}

	if status.Phase == parametersv1alpha1.ParameterRollbackPending {
		if !reflect.DeepEqual(compParam.Spec.Desired, request.Desired) {
			if compParam.Generation != request.SourceGeneration+1 {
				return r.failRollback(reqCtx, compParam, request.RequestID,
					"ComponentParameter desired state changed before rollback was applied")
			}
			patch := client.MergeFromWithOptions(compParam.DeepCopy(), client.MergeFromWithOptimisticLock{})
			compParam.Spec.Desired = request.Desired.DeepCopy()
			return true, r.Client.Patch(reqCtx.Ctx, compParam, patch)
		}
		return r.patchRollbackStatus(reqCtx, compParam, &parametersv1alpha1.ParameterRollbackStatus{
			RequestID:   request.RequestID,
			RequestHash: requestHash,
			Phase:       parametersv1alpha1.ParameterRollbackRunning,
			Message:     "rendering restored parameter inputs",
		})
	}

	if status.Phase != parametersv1alpha1.ParameterRollbackRunning {
		return r.failRollback(reqCtx, compParam, request.RequestID, "unknown rollback phase")
	}
	if !reflect.DeepEqual(compParam.Spec.Desired, request.Desired) {
		return r.failRollback(reqCtx, compParam, request.RequestID, "rollback desired state was changed")
	}
	if compParam.Status.ObservedGeneration == compParam.Generation && isRollbackSourceFailure(compParam.Status.Phase) {
		return r.failRollback(reqCtx, compParam, request.RequestID,
			"restored parameter reconciliation failed")
	}

	if status.TargetGeneration != 0 {
		if compParam.Generation != status.TargetGeneration {
			return r.failRollback(reqCtx, compParam, request.RequestID,
				"ComponentParameter generation changed while rollback was running")
		}
		if compParam.Status.ObservedGeneration != status.TargetGeneration {
			return false, nil
		}
		switch compParam.Status.Phase {
		case parametersv1alpha1.CFinishedPhase:
			message := "restored parameters converged"
			if request.Restart {
				message = "restored parameters and controlled restart converged"
			}
			return r.patchRollbackStatus(reqCtx, compParam, &parametersv1alpha1.ParameterRollbackStatus{
				RequestID:        request.RequestID,
				RequestHash:      requestHash,
				Phase:            parametersv1alpha1.ParameterRollbackSucceeded,
				TargetGeneration: status.TargetGeneration,
				Message:          message,
			})
		case parametersv1alpha1.CMergeFailedPhase, parametersv1alpha1.CFailedAndPausePhase:
			return r.failRollback(reqCtx, compParam, request.RequestID,
				"restored parameter reconciliation failed")
		default:
			return false, nil
		}
	}

	ready, err := r.prepareRollbackConfigMaps(reqCtx, compParam, request)
	if err != nil {
		return false, err
	}
	if !ready {
		return false, nil
	}
	return r.patchRollbackStatus(reqCtx, compParam, &parametersv1alpha1.ParameterRollbackStatus{
		RequestID:        request.RequestID,
		RequestHash:      requestHash,
		Phase:            parametersv1alpha1.ParameterRollbackRunning,
		TargetGeneration: compParam.Generation,
		Message:          "waiting for restored configuration to converge",
	})
}

func (r *ComponentParameterReconciler) prepareRollbackConfigMaps(reqCtx intctrlutil.RequestCtx,
	compParam *parametersv1alpha1.ComponentParameter, request *parametersv1alpha1.ParameterRollbackRequest) (bool, error) {
	targetRevision := strconv.FormatInt(compParam.Generation, 10)
	expectedRestart := strconv.FormatBool(request.Restart)
	for _, item := range compParam.Spec.ConfigItemDetails {
		if item.ConfigSpec == nil {
			continue
		}
		configMap := &corev1.ConfigMap{}
		key := client.ObjectKey{
			Namespace: compParam.Namespace,
			Name: parameterscore.GetComponentCfgName(
				compParam.Spec.ClusterName, compParam.Spec.ComponentName, item.Name),
		}
		if err := r.Client.Get(reqCtx.Ctx, key, configMap); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if configMap.Annotations[constant.ConfigurationRevision] != targetRevision {
			return false, nil
		}
		if configMap.Annotations[constant.ParameterRollbackRevisionAnnotationKey] != targetRevision ||
			configMap.Annotations[constant.ParameterRollbackRestartAnnotationKey] != expectedRestart {
			return false, nil
		}
	}
	return true, nil
}

func (r *ComponentParameterReconciler) cleanupRollbackMarkers(reqCtx intctrlutil.RequestCtx,
	compParam *parametersv1alpha1.ComponentParameter) (bool, error) {
	changed := false
	for _, item := range compParam.Spec.ConfigItemDetails {
		if item.ConfigSpec == nil {
			continue
		}
		configMap := &corev1.ConfigMap{}
		key := client.ObjectKey{
			Namespace: compParam.Namespace,
			Name: parameterscore.GetComponentCfgName(
				compParam.Spec.ClusterName, compParam.Spec.ComponentName, item.Name),
		}
		if err := r.Client.Get(reqCtx.Ctx, key, configMap); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return changed, err
		}
		_, hasRevision := configMap.Annotations[constant.ParameterRollbackRevisionAnnotationKey]
		_, hasRestart := configMap.Annotations[constant.ParameterRollbackRestartAnnotationKey]
		if !hasRevision && !hasRestart {
			continue
		}
		patch := client.MergeFromWithOptions(configMap.DeepCopy(), client.MergeFromWithOptimisticLock{})
		delete(configMap.Annotations, constant.ParameterRollbackRevisionAnnotationKey)
		delete(configMap.Annotations, constant.ParameterRollbackRestartAnnotationKey)
		if err := r.Client.Patch(reqCtx.Ctx, configMap, patch); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}

func (r *ComponentParameterReconciler) patchRollbackStatus(reqCtx intctrlutil.RequestCtx,
	compParam *parametersv1alpha1.ComponentParameter, status *parametersv1alpha1.ParameterRollbackStatus) (bool, error) {
	if reflect.DeepEqual(compParam.Status.Rollback, status) {
		return true, nil
	}
	patch := client.MergeFromWithOptions(compParam.DeepCopy(), client.MergeFromWithOptimisticLock{})
	compParam.Status.Rollback = status
	return true, r.Client.Status().Patch(reqCtx.Ctx, compParam, patch)
}

func (r *ComponentParameterReconciler) failRollback(reqCtx intctrlutil.RequestCtx,
	compParam *parametersv1alpha1.ComponentParameter, requestID, message string) (bool, error) {
	requestHash := ""
	targetGeneration := int64(0)
	if compParam.Status.Rollback != nil && compParam.Status.Rollback.RequestID == requestID {
		requestHash = compParam.Status.Rollback.RequestHash
		targetGeneration = compParam.Status.Rollback.TargetGeneration
	}
	return r.patchRollbackStatus(reqCtx, compParam, &parametersv1alpha1.ParameterRollbackStatus{
		RequestID:        requestID,
		RequestHash:      requestHash,
		Phase:            parametersv1alpha1.ParameterRollbackFailed,
		TargetGeneration: targetGeneration,
		Message:          message,
	})
}

func rollbackRequestHash(request *parametersv1alpha1.ParameterRollbackRequest) (string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("%x", digest), nil
}

func isRollbackSourceFailure(phase parametersv1alpha1.ParameterPhase) bool {
	return phase == parametersv1alpha1.CMergeFailedPhase || phase == parametersv1alpha1.CFailedAndPausePhase
}
