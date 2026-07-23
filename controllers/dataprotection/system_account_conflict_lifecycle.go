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

package dataprotection

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// SystemAccountConflictReceiptLifecycleReconciler retains loser decisions
// while their exact root UID is live and releases them when that root starts
// terminating or is gone.
type SystemAccountConflictReceiptLifecycleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *SystemAccountConflictReceiptLifecycleReconciler) Reconcile(
	ctx context.Context,
	req controllerruntime.Request,
) (controllerruntime.Result, error) {
	receipt := &corev1.Secret{}
	if err := r.Client.Get(ctx, req.NamespacedName, receipt); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, log.FromContext(ctx), "")
	}
	if receipt.Annotations[systemaccount.RestoreProtocolAnnotationKey] !=
		systemaccount.ConflictProtocolV1 {
		return controllerruntime.Result{}, nil
	}
	envelope, err := systemaccount.DecodeAndValidateConflictReceipt(receipt, false)
	if err != nil {
		return r.reconcileInvalidConflictMetadata(ctx, receipt, err)
	}
	rootGone, rootTerminating, err := r.conflictRootState(ctx, envelope)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if !rootGone && !rootTerminating {
		return conflictReceiptRequeue(receipt), nil
	}

	if rootTerminating {
		if projectionErr := r.projectConflictReceipt(ctx, receipt, envelope); projectionErr != nil {
			log.FromContext(ctx).Error(projectionErr,
				"bounded loser projection failed before conflict receipt release",
				"receipt", client.ObjectKeyFromObject(receipt))
		}
	}
	if slices.Contains(receipt.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		updated := receipt.DeepCopy()
		updated.Finalizers = slices.DeleteFunc(updated.Finalizers,
			func(value string) bool { return value == systemaccount.RestoreProtocolFinalizer })
		if err := r.Client.Update(ctx, updated); err != nil {
			return controllerruntime.Result{}, err
		}
		return controllerruntime.Result{}, nil
	}
	if rootGone && receipt.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, r.Client.Delete(ctx, receipt)
	}
	return controllerruntime.Result{}, nil
}

func (r *SystemAccountConflictReceiptLifecycleReconciler) reconcileInvalidConflictMetadata(
	ctx context.Context,
	receipt *corev1.Secret,
	validationErr error,
) (controllerruntime.Result, error) {
	if receipt.Immutable == nil || !*receipt.Immutable {
		return controllerruntime.Result{}, validationErr
	}
	envelope, err := systemaccount.DecodeConflictEnvelope(receipt)
	if err != nil || envelope.Protocol != systemaccount.ConflictProtocolV1 ||
		envelope.Decision != systemaccount.ConcurrentRestoreIntentReason {
		return controllerruntime.Result{}, validationErr
	}
	rootGone, rootTerminating, err := r.conflictRootState(ctx, envelope)
	if err != nil {
		return controllerruntime.Result{}, err
	}
	if !rootGone && !rootTerminating {
		return controllerruntime.Result{}, validationErr
	}
	if !slices.Contains(receipt.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		return controllerruntime.Result{}, nil
	}
	updated := receipt.DeepCopy()
	updated.Finalizers = slices.DeleteFunc(updated.Finalizers,
		func(value string) bool { return value == systemaccount.RestoreProtocolFinalizer })
	if err := r.Client.Update(ctx, updated); err != nil {
		return controllerruntime.Result{}, err
	}
	return controllerruntime.Result{}, nil
}

func (r *SystemAccountConflictReceiptLifecycleReconciler) conflictRootState(
	ctx context.Context,
	envelope systemaccount.ConflictEnvelope,
) (bool, bool, error) {
	root := &appsv1.Cluster{}
	rootKey := client.ObjectKey{
		Namespace: envelope.LoserOperation.Root.Namespace,
		Name:      envelope.LoserOperation.Root.Name,
	}
	err := r.Client.Get(ctx, rootKey, root)
	rootGone := apierrors.IsNotFound(err)
	if err != nil && !rootGone {
		return false, false, err
	}
	if !rootGone && root.UID != envelope.LoserOperation.Root.UID {
		rootGone = true
	}
	return rootGone, !rootGone && !root.DeletionTimestamp.IsZero(), nil
}

func (r *SystemAccountConflictReceiptLifecycleReconciler) projectConflictReceipt(
	ctx context.Context,
	receipt *corev1.Secret,
	envelope systemaccount.ConflictEnvelope,
) error {
	pvcs, err := r.participantsForOperation(ctx, envelope.LoserOperation)
	if err != nil {
		return err
	}
	loserDigest, _ := systemaccount.OperationDigest(envelope.LoserOperation)
	volumePopulator := &VolumePopulatorReconciler{Client: r.Client, Scheme: r.Scheme}
	var projectionErrors []error
	for _, pvc := range pvcs {
		if err := volumePopulator.projectSystemAccountConflictIdentity(
			intctrlutil.RequestCtx{Ctx: ctx}, pvc, envelope); err != nil {
			projectionErrors = append(projectionErrors,
				fmt.Errorf("project conflict receipt %s identity to PVC %s: %w",
					client.ObjectKeyFromObject(receipt), client.ObjectKeyFromObject(pvc), err))
			continue
		}
		updated := pvc.DeepCopy()
		upsertPVCCondition(&updated.Status.Conditions, corev1.PersistentVolumeClaimCondition{
			Type:               appsv1.ConditionTypeRestore,
			Status:             corev1.ConditionFalse,
			LastProbeTime:      metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             systemaccount.ConcurrentRestoreIntentReason,
			Message: fmt.Sprintf("restore operation %s is blocked by request %s/%s UID %s",
				loserDigest,
				envelope.BlockingRequest.Namespace,
				envelope.BlockingRequest.Name,
				envelope.BlockingRequest.UID),
		})
		if err := r.Client.Status().Update(ctx, updated); err != nil {
			projectionErrors = append(projectionErrors,
				fmt.Errorf("project conflict receipt %s to PVC %s: %w",
					client.ObjectKeyFromObject(receipt), client.ObjectKeyFromObject(pvc), err))
		}
	}
	return errors.Join(projectionErrors...)
}

func (r *SystemAccountConflictReceiptLifecycleReconciler) participantsForOperation(
	ctx context.Context,
	operation systemaccount.RestoreOperationIdentity,
) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(ctx, pvcs, client.InNamespace(operation.Root.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: operation.Root.Name}); err != nil {
		return nil, err
	}
	operationDigest, err := systemaccount.OperationDigest(operation)
	if err != nil {
		return nil, err
	}
	volumePopulator := &VolumePopulatorReconciler{Client: r.Client, Scheme: r.Scheme}
	participants := make([]*corev1.PersistentVolumeClaim, 0, len(pvcs.Items))
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if !pvcReferencesOperationSource(pvc, operation) {
			continue
		}
		backup := &dpv1alpha1.Backup{}
		if err := r.Client.Get(ctx, client.ObjectKey{
			Namespace: operation.Source.Namespace,
			Name:      operation.Source.Name,
		}, backup); err != nil {
			continue
		}
		authority, err := volumePopulator.resolveSystemAccountRestoreAuthority(
			intctrlutil.RequestCtx{Ctx: ctx}, pvc, backup, operation.Root.Name)
		if err != nil {
			continue
		}
		candidateDigest, err := systemaccount.OperationDigest(authority.operation)
		if err == nil && candidateDigest == operationDigest {
			participants = append(participants, pvc)
		}
	}
	return participants, nil
}

func (r *SystemAccountConflictReceiptLifecycleReconciler) SetupWithManager(
	mgr controllerruntime.Manager,
) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		Named("system-account-conflict-receipt-lifecycle").
		For(&corev1.Secret{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			func(object client.Object) bool {
				return object.GetAnnotations()[systemaccount.RestoreProtocolAnnotationKey] ==
					systemaccount.ConflictProtocolV1
			}))).
		Complete(r)
}

func conflictReceiptRequeue(receipt *corev1.Secret) controllerruntime.Result {
	if !slices.Contains(receipt.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		return controllerruntime.Result{}
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(receipt.Namespace + "/" + receipt.Name))
	return controllerruntime.Result{RequeueAfter: time.Duration(1+hash.Sum32()%30) * time.Second}
}

func pvcReferencesOperationSource(
	pvc *corev1.PersistentVolumeClaim,
	operation systemaccount.RestoreOperationIdentity,
) bool {
	if pvc.Spec.DataSourceRef == nil || pvc.Spec.DataSourceRef.APIGroup == nil ||
		*pvc.Spec.DataSourceRef.APIGroup != operation.Source.APIGroup ||
		pvc.Spec.DataSourceRef.Kind != operation.Source.Kind ||
		pvc.Spec.DataSourceRef.Name != operation.Source.Name ||
		pvc.Annotations[constant.RestorePITRAnnotationKey] != operation.PITR {
		return false
	}
	namespace := pvc.Namespace
	if pvc.Spec.DataSourceRef.Namespace != nil {
		namespace = *pvc.Spec.DataSourceRef.Namespace
	}
	return namespace == operation.Source.Namespace
}
