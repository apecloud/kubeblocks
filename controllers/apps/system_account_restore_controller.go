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

package apps

import (
	"context"
	"fmt"
	"hash/fnv"
	"reflect"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type restoreOperationState string

const (
	restoreOperationActive      restoreOperationState = "Active"
	restoreOperationSucceeded   restoreOperationState = "TerminalSuccess"
	restoreOperationFailed      restoreOperationState = "TerminalFailure"
	restoreOperationTerminating restoreOperationState = "Terminating"
	restoreOperationGone        restoreOperationState = "Gone"
)

// SystemAccountRestoreLifecycleReconciler keeps request cleanup reachable when
// Component and Cluster reconcilers return early during owner deletion.
type SystemAccountRestoreLifecycleReconciler struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

func (r *SystemAccountRestoreLifecycleReconciler) Reconcile(
	ctx context.Context,
	req controllerruntime.Request,
) (controllerruntime.Result, error) {
	reader, err := r.authorityReader()
	if err != nil {
		return controllerruntime.Result{}, err
	}
	request := &corev1.Secret{}
	if err := reader.Get(ctx, req.NamespacedName, request); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, log.FromContext(ctx), "")
	}
	if request.Annotations[systemaccount.RestoreProtocolAnnotationKey] != systemaccount.RestoreProtocolV2 {
		return controllerruntime.Result{}, nil
	}
	intent, err := systemaccount.DecodeRestoreRequestV2(request)
	if err != nil {
		return r.reconcileInvalidRequestMetadata(ctx, request, err)
	}
	rootState, err := r.restoreOperationState(ctx, intent.Operation)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	phase := systemaccount.RestoreRequestPhase(
		request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	hasFinalizer := slices.Contains(request.Finalizers, systemaccount.RestoreProtocolFinalizer)

	if !phase.Valid() {
		updated := request.DeepCopy()
		if updated.Annotations == nil {
			updated.Annotations = map[string]string{}
		}
		updated.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] =
			systemaccount.InvalidPhaseReason
		if rootState != restoreOperationActive {
			updated.Finalizers = slices.DeleteFunc(updated.Finalizers,
				func(value string) bool { return value == systemaccount.RestoreProtocolFinalizer })
		}
		if reflect.DeepEqual(request, updated) {
			return r.lifecycleRequeue(request), nil
		}
		if err := r.Client.Update(ctx, updated); err != nil {
			return controllerruntime.Result{}, err
		}
		return r.lifecycleRequeue(updated), nil
	}

	if rootState == restoreOperationGone || rootState == restoreOperationTerminating {
		return r.reconcileUnavailableRoot(ctx, request, phase, rootState)
	}
	if rootState == restoreOperationSucceeded || rootState == restoreOperationFailed {
		return r.reconcileTerminalOperation(ctx, request, phase)
	}
	if !request.DeletionTimestamp.IsZero() {
		if phase == systemaccount.RestoreRequestPhaseClaimed ||
			phase == systemaccount.RestoreRequestPhaseCommitted {
			target, exact, err := r.findExactTarget(ctx, request, intent)
			if err != nil {
				return controllerruntime.Result{}, err
			}
			if exact {
				if err := r.clearTargetReceipt(ctx, target); err != nil {
					return controllerruntime.Result{}, err
				}
			}
		}
		if phase != systemaccount.RestoreRequestPhaseFailed {
			return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseFailed,
				systemaccount.RequestDeletionRequestedReason, nil, false)
		}
		return r.lifecycleRequeue(request), nil
	}
	if !hasFinalizer {
		return controllerruntime.Result{}, nil
	}

	ownerLive, err := r.targetOwnerLive(ctx, intent.Target.Owner)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if !ownerLive {
		target, exact, err := r.findExactTarget(ctx, request, intent)
		if err != nil {
			return controllerruntime.Result{}, err
		}
		reason := systemaccount.TargetOwnerUnavailableReason
		if phase == systemaccount.RestoreRequestPhaseClaimed && exact {
			if err := r.clearTargetReceipt(ctx, target); err != nil {
				return controllerruntime.Result{}, err
			}
			reason = systemaccount.PostWriteCancellationReason
		}
		if phase != systemaccount.RestoreRequestPhaseFailed {
			return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseFailed,
				reason, nil, false)
		}
		return r.lifecycleRequeue(request), nil
	}
	if phase == systemaccount.RestoreRequestPhaseClaimed ||
		phase == systemaccount.RestoreRequestPhaseCommitted {
		target, exact, err := r.findExactTarget(ctx, request, intent)
		if err != nil {
			return controllerruntime.Result{}, err
		}
		if exact {
			revision, err := systemaccount.TargetCommitRevision(target,
				requiredSystemAccountFinalizer(intent.Target.Scope))
			if err != nil {
				return controllerruntime.Result{}, err
			}
			return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseCommitted,
				"", &systemaccount.RestoreCommitReceipt{
					TargetName:     target.Name,
					TargetUID:      target.UID,
					CommitRevision: revision,
				}, false)
		}
	}
	return r.lifecycleRequeue(request), nil
}

func (r *SystemAccountRestoreLifecycleReconciler) reconcileInvalidRequestMetadata(
	ctx context.Context,
	request *corev1.Secret,
	validationErr error,
) (controllerruntime.Result, error) {
	intent, err := systemaccount.DecodeRestoreIntentEnvelope(request)
	if err != nil {
		return controllerruntime.Result{}, validationErr
	}
	rootState, err := r.restoreOperationState(ctx, intent.Operation)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if rootState != restoreOperationGone && rootState != restoreOperationTerminating {
		return controllerruntime.Result{}, validationErr
	}
	if !slices.Contains(request.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		return controllerruntime.Result{}, nil
	}
	updated := request.DeepCopy()
	updated.Finalizers = slices.DeleteFunc(updated.Finalizers,
		func(value string) bool { return value == systemaccount.RestoreProtocolFinalizer })
	if updated.Annotations == nil {
		updated.Annotations = map[string]string{}
	}
	updated.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] =
		systemaccount.InvalidPhaseReason
	if err := r.Client.Update(ctx, updated); err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

func (r *SystemAccountRestoreLifecycleReconciler) reconcileUnavailableRoot(
	ctx context.Context,
	request *corev1.Secret,
	phase systemaccount.RestoreRequestPhase,
	rootState restoreOperationState,
) (controllerruntime.Result, error) {
	intent, _ := systemaccount.DecodeRestoreRequestV2(request)
	target, exact, err := r.findExactTarget(ctx, request, intent)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	reason := systemaccount.RootUnavailableReason
	if !request.DeletionTimestamp.IsZero() {
		reason = systemaccount.RequestDeletionRequestedReason
	}
	if phase == systemaccount.RestoreRequestPhaseClaimed && exact {
		if err := r.clearTargetReceipt(ctx, target); err != nil {
			return controllerruntime.Result{}, err
		}
		reason = systemaccount.PostWriteCancellationReason
	}
	if phase == systemaccount.RestoreRequestPhaseCommitted {
		receipt := committedReceipt(request)
		if slices.Contains(request.Finalizers, systemaccount.RestoreProtocolFinalizer) {
			if receipt == nil {
				return r.releaseProtocolFinalizer(ctx, request)
			}
			return r.transitionRequest(ctx, request, phase, "", receipt, true)
		}
		if rootState == restoreOperationGone && request.DeletionTimestamp.IsZero() {
			return controllerruntime.Result{}, r.Client.Delete(ctx, request)
		}
		return controllerruntime.Result{}, nil
	}
	if phase != systemaccount.RestoreRequestPhaseFailed {
		return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseFailed,
			reason, nil, true)
	}
	if slices.Contains(request.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		storedReason := request.Annotations[systemaccount.RestoreRequestReasonAnnotationKey]
		if !systemaccount.IsRestoreFailureReason(storedReason) {
			storedReason = systemaccount.RootUnavailableReason
		}
		return r.transitionRequest(ctx, request, phase,
			storedReason, nil, true)
	}
	if rootState == restoreOperationGone && request.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, r.Client.Delete(ctx, request)
	}
	return controllerruntime.Result{}, nil
}

func (r *SystemAccountRestoreLifecycleReconciler) reconcileTerminalOperation(
	ctx context.Context,
	request *corev1.Secret,
	phase systemaccount.RestoreRequestPhase,
) (controllerruntime.Result, error) {
	intent, _ := systemaccount.DecodeRestoreRequestV2(request)
	if phase == systemaccount.RestoreRequestPhaseClaimed ||
		phase == systemaccount.RestoreRequestPhaseCommitted {
		ownerLive, err := r.targetOwnerLive(ctx, intent.Target.Owner)
		if err != nil {
			return controllerruntime.Result{}, err
		}
		if phase == systemaccount.RestoreRequestPhaseClaimed && !ownerLive {
			return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseFailed,
				systemaccount.OperationTerminalReason, nil, true)
		}
		target, exact, err := r.findExactTarget(ctx, request, intent)
		if err != nil {
			return controllerruntime.Result{}, err
		}
		if exact {
			if phase == systemaccount.RestoreRequestPhaseCommitted &&
				!slices.Contains(request.Finalizers, systemaccount.RestoreProtocolFinalizer) {
				if request.DeletionTimestamp.IsZero() {
					return controllerruntime.Result{}, r.Client.Delete(ctx, request)
				}
				return controllerruntime.Result{}, nil
			}
			revision, err := systemaccount.TargetCommitRevision(target,
				requiredSystemAccountFinalizer(intent.Target.Scope))
			if err != nil {
				return controllerruntime.Result{}, err
			}
			return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseCommitted,
				"", &systemaccount.RestoreCommitReceipt{
					TargetName:     target.Name,
					TargetUID:      target.UID,
					CommitRevision: revision,
				}, phase == systemaccount.RestoreRequestPhaseCommitted)
		}
		if phase == systemaccount.RestoreRequestPhaseClaimed {
			return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseFailed,
				systemaccount.OperationTerminalReason, nil, true)
		}
	}
	if phase == systemaccount.RestoreRequestPhasePending {
		return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseFailed,
			systemaccount.OperationTerminalReason, nil, true)
	}
	if slices.Contains(request.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		receipt := committedReceipt(request)
		if phase == systemaccount.RestoreRequestPhaseCommitted && receipt == nil {
			return r.releaseProtocolFinalizer(ctx, request)
		}
		reason := request.Annotations[systemaccount.RestoreRequestReasonAnnotationKey]
		if phase == systemaccount.RestoreRequestPhaseFailed &&
			!systemaccount.IsRestoreFailureReason(reason) {
			reason = systemaccount.OperationTerminalReason
		}
		return r.transitionRequest(ctx, request, phase, reason, receipt, true)
	}
	if request.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, r.Client.Delete(ctx, request)
	}
	return controllerruntime.Result{}, nil
}

func (r *SystemAccountRestoreLifecycleReconciler) transitionRequest(
	ctx context.Context,
	request *corev1.Secret,
	phase systemaccount.RestoreRequestPhase,
	reason string,
	receipt *systemaccount.RestoreCommitReceipt,
	releaseFinalizer bool,
) (controllerruntime.Result, error) {
	updated, err := systemaccount.TransitionRestoreRequestV2(
		request, phase, reason, receipt, releaseFinalizer)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if reflect.DeepEqual(request, updated) {
		return r.lifecycleRequeue(request), nil
	}
	if err := r.Client.Update(ctx, updated); err != nil {
		return controllerruntime.Result{}, err
	}
	return r.lifecycleRequeue(updated), nil
}

func (r *SystemAccountRestoreLifecycleReconciler) releaseProtocolFinalizer(
	ctx context.Context,
	request *corev1.Secret,
) (controllerruntime.Result, error) {
	updated := request.DeepCopy()
	updated.Finalizers = slices.DeleteFunc(updated.Finalizers,
		func(value string) bool { return value == systemaccount.RestoreProtocolFinalizer })
	if reflect.DeepEqual(request, updated) {
		return controllerruntime.Result{}, nil
	}
	if err := r.Client.Update(ctx, updated); err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

func (r *SystemAccountRestoreLifecycleReconciler) restoreOperationState(
	ctx context.Context,
	operation systemaccount.RestoreOperationIdentity,
) (restoreOperationState, error) {
	reader, err := r.authorityReader()
	if err != nil {
		return "", err
	}
	state, err := systemaccount.ReadRestoreOperationState(ctx, reader, operation)
	if err != nil {
		return "", err
	}
	switch state {
	case systemaccount.RestoreOperationActive:
		return restoreOperationActive, nil
	case systemaccount.RestoreOperationSucceeded:
		return restoreOperationSucceeded, nil
	case systemaccount.RestoreOperationFailed:
		return restoreOperationFailed, nil
	case systemaccount.RestoreOperationTerminating:
		return restoreOperationTerminating, nil
	case systemaccount.RestoreOperationGone:
		return restoreOperationGone, nil
	default:
		return "", fmt.Errorf("unsupported system account restore operation state %q", state)
	}
}

func (r *SystemAccountRestoreLifecycleReconciler) targetOwnerLive(
	ctx context.Context,
	identity systemaccount.ObjectIdentity,
) (bool, error) {
	reader, err := r.authorityReader()
	if err != nil {
		return false, err
	}
	return systemaccount.ReadRestoreTargetOwnerLive(ctx, reader, identity)
}

func (r *SystemAccountRestoreLifecycleReconciler) findExactTarget(
	ctx context.Context,
	request *corev1.Secret,
	intent systemaccount.CredentialIntent,
) (*corev1.Secret, bool, error) {
	requiredFinalizer := requiredSystemAccountFinalizer(intent.Target.Scope)
	reader, err := r.authorityReader()
	if err != nil {
		return nil, false, err
	}
	if name := request.Annotations[systemaccount.TargetSecretNameAnnotationKey]; name != "" {
		target := &corev1.Secret{}
		if err := reader.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: name}, target); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, false, err
			}
		} else if systemaccount.TargetReceiptExactV2(target, request, requiredFinalizer) {
			return target, true, nil
		}
	}
	secrets := &corev1.SecretList{}
	if err := reader.List(ctx, secrets, client.InNamespace(request.Namespace)); err != nil {
		return nil, false, err
	}
	var exactTarget *corev1.Secret
	for i := range secrets.Items {
		target := &secrets.Items[i]
		if target.Annotations[systemaccount.RestoreRequestNameAnnotationKey] == request.Name &&
			target.Annotations[systemaccount.RestoreRequestUIDAnnotationKey] == string(request.UID) &&
			systemaccount.TargetReceiptExactV2(target, request, requiredFinalizer) {
			if exactTarget != nil {
				return nil, false, fmt.Errorf(
					"multiple exact targets %s/%s and %s/%s link restore request %s/%s",
					exactTarget.Namespace, exactTarget.Name, target.Namespace, target.Name,
					request.Namespace, request.Name)
			}
			exactTarget = target
		}
	}
	if exactTarget != nil {
		return exactTarget, true, nil
	}
	return nil, false, nil
}

func (r *SystemAccountRestoreLifecycleReconciler) clearTargetReceipt(
	ctx context.Context,
	target *corev1.Secret,
) error {
	if target == nil || !target.DeletionTimestamp.IsZero() {
		return nil
	}
	updated := target.DeepCopy()
	systemaccount.ClearTargetRestoreReceipt(updated)
	return r.Client.Update(ctx, updated)
}

func (r *SystemAccountRestoreLifecycleReconciler) lifecycleRequeue(
	request *corev1.Secret,
) controllerruntime.Result {
	if !slices.Contains(request.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		return controllerruntime.Result{}
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(request.Namespace + "/" + request.Name))
	return controllerruntime.Result{RequeueAfter: time.Duration(1+hash.Sum32()%30) * time.Second}
}

func (r *SystemAccountRestoreLifecycleReconciler) SetupWithManager(mgr controllerruntime.Manager) error {
	if r.APIReader == nil {
		r.APIReader = mgr.GetAPIReader()
	}
	return intctrlutil.NewControllerManagedBy(mgr).
		Named("system-account-restore-lifecycle").
		For(&corev1.Secret{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			func(object client.Object) bool {
				return object.GetAnnotations()[systemaccount.RestoreProtocolAnnotationKey] ==
					systemaccount.RestoreProtocolV2
			}))).
		Complete(r)
}

func (r *SystemAccountRestoreLifecycleReconciler) authorityReader() (client.Reader, error) {
	if r.APIReader == nil {
		return nil, fmt.Errorf("system account restore authority API reader is not configured")
	}
	return r.APIReader, nil
}

func committedReceipt(request *corev1.Secret) *systemaccount.RestoreCommitReceipt {
	if systemaccount.RestoreRequestPhase(
		request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey]) !=
		systemaccount.RestoreRequestPhaseCommitted {
		return nil
	}
	receipt := &systemaccount.RestoreCommitReceipt{
		TargetName:     request.Annotations[systemaccount.TargetSecretNameAnnotationKey],
		TargetUID:      types.UID(request.Annotations[systemaccount.TargetSecretUIDAnnotationKey]),
		CommitRevision: request.Annotations[systemaccount.TargetCommitRevisionAnnotationKey],
	}
	if receipt.TargetName == "" || receipt.TargetUID == "" || receipt.CommitRevision == "" {
		return nil
	}
	return receipt
}

func requiredSystemAccountFinalizer(scope string) string {
	if scope == systemaccount.SystemAccountScopeSharding {
		return constant.DBClusterFinalizerName
	}
	return constant.DBComponentFinalizerName
}
