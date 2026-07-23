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
	"maps"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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
	Scheme *runtime.Scheme
}

func (r *SystemAccountRestoreLifecycleReconciler) Reconcile(
	ctx context.Context,
	req controllerruntime.Request,
) (controllerruntime.Result, error) {
	request := &corev1.Secret{}
	if err := r.Client.Get(ctx, req.NamespacedName, request); err != nil {
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
		if err := r.Client.Update(ctx, updated); err != nil {
			return controllerruntime.Result{}, err
		}
		return r.lifecycleRequeue(request), nil
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
	if phase == systemaccount.RestoreRequestPhaseClaimed {
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
		if receipt == nil {
			return r.releaseProtocolFinalizer(ctx, request)
		}
		return r.transitionRequest(ctx, request, phase, "", receipt, true)
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
	if phase == systemaccount.RestoreRequestPhaseClaimed {
		ownerLive, err := r.targetOwnerLive(ctx, intent.Target.Owner)
		if err != nil {
			return controllerruntime.Result{}, err
		}
		if !ownerLive {
			return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseFailed,
				systemaccount.OperationTerminalReason, nil, true)
		}
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
		return r.transitionRequest(ctx, request, systemaccount.RestoreRequestPhaseFailed,
			systemaccount.OperationTerminalReason, nil, true)
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
	if err := r.Client.Update(ctx, updated); err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

func (r *SystemAccountRestoreLifecycleReconciler) restoreOperationState(
	ctx context.Context,
	operation systemaccount.RestoreOperationIdentity,
) (restoreOperationState, error) {
	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: operation.Root.Namespace,
		Name:      operation.Root.Name,
	}, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return restoreOperationGone, nil
		}
		return "", err
	}
	if cluster.UID != operation.Root.UID {
		return restoreOperationGone, nil
	}
	if !cluster.DeletionTimestamp.IsZero() {
		return restoreOperationTerminating, nil
	}
	if operation.Profile == systemaccount.RestoreProfileLegacyPVCGroup {
		return r.legacyRestoreOperationState(ctx, operation)
	}
	if cluster.Spec.Restore == nil {
		return restoreOperationGone, nil
	}
	namespace := cluster.Spec.Restore.Source.Namespace
	if namespace == "" {
		namespace = cluster.Namespace
	}
	current := systemaccount.RestoreOperationIdentity{
		Protocol: systemaccount.RestoreProtocolV2,
		Profile:  systemaccount.RestoreProfileInitialCluster,
		Root:     appsObjectIdentity(cluster),
		Source: systemaccount.SourceIdentity{
			APIGroup:  cluster.Spec.Restore.Source.APIGroup,
			Kind:      cluster.Spec.Restore.Source.Kind,
			Namespace: namespace,
			Name:      cluster.Spec.Restore.Source.Name,
		},
		PITR:       cluster.Spec.Restore.PITR,
		Parameters: maps.Clone(cluster.Spec.Restore.Parameters),
	}
	currentDigest, err := systemaccount.OperationDigest(current)
	if err != nil {
		return "", err
	}
	expectedDigest, err := systemaccount.OperationDigest(operation)
	if err != nil || currentDigest != expectedDigest {
		return restoreOperationGone, err
	}
	condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	if condition == nil || condition.ObservedGeneration != cluster.Generation ||
		condition.Status == metav1.ConditionUnknown {
		return restoreOperationActive, nil
	}
	if condition.Status == metav1.ConditionTrue {
		return restoreOperationSucceeded, nil
	}
	return restoreOperationFailed, nil
}

func (r *SystemAccountRestoreLifecycleReconciler) legacyRestoreOperationState(
	ctx context.Context,
	operation systemaccount.RestoreOperationIdentity,
) (restoreOperationState, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(ctx, pvcs, client.InNamespace(operation.Root.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: operation.Root.Name}); err != nil {
		return "", err
	}
	found := false
	allSucceeded := true
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if !legacyPVCMatchesOperation(pvc, operation) ||
			!r.legacyPVCHasAuthority(ctx, pvc, operation.Root) {
			continue
		}
		found = true
		condition := findPVCCondition(pvc, appsv1.ConditionTypeRestore)
		if condition == nil || condition.Status == corev1.ConditionUnknown {
			allSucceeded = false
			continue
		}
		if condition.Status == corev1.ConditionFalse {
			return restoreOperationFailed, nil
		}
	}
	if !found {
		return restoreOperationGone, nil
	}
	if allSucceeded {
		return restoreOperationSucceeded, nil
	}
	return restoreOperationActive, nil
}

func (r *SystemAccountRestoreLifecycleReconciler) legacyPVCHasAuthority(
	ctx context.Context,
	pvc *corev1.PersistentVolumeClaim,
	root systemaccount.ObjectIdentity,
) bool {
	ref := metav1.GetControllerOf(pvc)
	if ref == nil || ref.APIVersion != workloads.GroupVersion.String() {
		return false
	}
	var workload client.Object
	switch ref.Kind {
	case workloads.InstanceSetKind:
		workload = &workloads.InstanceSet{}
	case "Instance":
		workload = &workloads.Instance{}
	default:
		return false
	}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: pvc.Namespace, Name: ref.Name}, workload); err != nil ||
		workload.GetUID() != ref.UID {
		return false
	}
	componentRef := metav1.GetControllerOf(workload)
	if componentRef == nil || componentRef.APIVersion != appsv1.GroupVersion.String() ||
		componentRef.Kind != appsv1.ComponentKind {
		return false
	}
	component := &appsv1.Component{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: pvc.Namespace, Name: componentRef.Name}, component); err != nil ||
		component.UID != componentRef.UID {
		return false
	}
	clusterRef := metav1.GetControllerOf(component)
	return clusterRef != nil && clusterRef.APIVersion == appsv1.GroupVersion.String() &&
		clusterRef.Kind == appsv1.ClusterKind &&
		clusterRef.Name == root.Name && clusterRef.UID == root.UID
}

func (r *SystemAccountRestoreLifecycleReconciler) targetOwnerLive(
	ctx context.Context,
	identity systemaccount.ObjectIdentity,
) (bool, error) {
	if identity.APIVersion != appsv1.GroupVersion.String() {
		return false, nil
	}
	var owner client.Object
	switch identity.Kind {
	case appsv1.ComponentKind:
		owner = &appsv1.Component{}
	case appsv1.ClusterKind:
		owner = &appsv1.Cluster{}
	default:
		return false, nil
	}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: identity.Namespace,
		Name:      identity.Name,
	}, owner); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return owner.GetUID() == identity.UID && owner.GetDeletionTimestamp().IsZero(), nil
}

func (r *SystemAccountRestoreLifecycleReconciler) findExactTarget(
	ctx context.Context,
	request *corev1.Secret,
	intent systemaccount.CredentialIntent,
) (*corev1.Secret, bool, error) {
	requiredFinalizer := requiredSystemAccountFinalizer(intent.Target.Scope)
	if name := request.Annotations[systemaccount.TargetSecretNameAnnotationKey]; name != "" {
		target := &corev1.Secret{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: name}, target); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, false, nil
			}
			return nil, false, err
		}
		return target, systemaccount.TargetReceiptExactV2(target, request, requiredFinalizer), nil
	}
	secrets := &corev1.SecretList{}
	if err := r.Client.List(ctx, secrets, client.InNamespace(request.Namespace)); err != nil {
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
	return intctrlutil.NewControllerManagedBy(mgr).
		Named("system-account-restore-lifecycle").
		For(&corev1.Secret{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			func(object client.Object) bool {
				return object.GetAnnotations()[systemaccount.RestoreProtocolAnnotationKey] ==
					systemaccount.RestoreProtocolV2
			}))).
		Complete(r)
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

func appsObjectIdentity(object client.Object) systemaccount.ObjectIdentity {
	kind := appsv1.ClusterKind
	if _, ok := object.(*appsv1.Component); ok {
		kind = appsv1.ComponentKind
	}
	return systemaccount.ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       kind,
		Namespace:  object.GetNamespace(),
		Name:       object.GetName(),
		UID:        object.GetUID(),
	}
}

func legacyPVCMatchesOperation(
	pvc *corev1.PersistentVolumeClaim,
	operation systemaccount.RestoreOperationIdentity,
) bool {
	if pvc.Spec.DataSourceRef == nil || pvc.Spec.DataSourceRef.APIGroup == nil ||
		*pvc.Spec.DataSourceRef.APIGroup != operation.Source.APIGroup ||
		pvc.Spec.DataSourceRef.Name != operation.Source.Name ||
		pvc.Spec.DataSourceRef.Kind != operation.Source.Kind ||
		pvc.Annotations[constant.RestorePITRAnnotationKey] != operation.PITR {
		return false
	}
	namespace := pvc.Namespace
	if pvc.Spec.DataSourceRef.Namespace != nil {
		namespace = *pvc.Spec.DataSourceRef.Namespace
	}
	return namespace == operation.Source.Namespace
}

func findPVCCondition(
	pvc *corev1.PersistentVolumeClaim,
	conditionType corev1.PersistentVolumeClaimConditionType,
) *corev1.PersistentVolumeClaimCondition {
	for i := range pvc.Status.Conditions {
		if pvc.Status.Conditions[i].Type == conditionType {
			return &pvc.Status.Conditions[i]
		}
	}
	return nil
}
