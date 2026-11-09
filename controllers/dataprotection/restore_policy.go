/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package dataprotection

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func isInvalidRestorePolicy(err error) bool {
	var policyErr *dptypes.InvalidRestorePolicyError
	return errors.As(err, &policyErr)
}

func configuredRestorePolicyError(restore *dpv1alpha1.Restore) error {
	if restore.Spec.PrepareDataConfig == nil {
		return nil
	}
	return dptypes.ValidateVolumeRestorePolicy(string(restore.Spec.PrepareDataConfig.VolumeClaimRestorePolicy),
		"spec.prepareDataConfig.volumeClaimRestorePolicy")
}

func pvcRestorePolicyError(ctx context.Context, cli client.Reader, pvc *corev1.PersistentVolumeClaim) error {
	parameters, err := restoreParametersMapFromPVC(pvc)
	if err != nil {
		return intctrlutil.NewFatalError(err.Error())
	}
	if err := dptypes.ValidateVolumeRestorePolicy(parameters[dptypes.VolumeRestorePolicyParameterKey],
		"PVC restore parameters["+dptypes.VolumeRestorePolicyParameterKey+"]"); err != nil {
		return err
	}
	if pvc.Spec.DataSourceRef != nil && pvc.Spec.DataSourceRef.Kind == dptypes.RestoreKind {
		source := &dpv1alpha1.Restore{}
		if err := cli.Get(ctx, types.NamespacedName{Namespace: pvc.Namespace, Name: pvc.Spec.DataSourceRef.Name}, source); err != nil {
			return err
		}
		if err := configuredRestorePolicyError(source); err != nil {
			return err
		}
	}
	return clusterRestorePolicyError(ctx, cli, pvc.Namespace, pvc.Labels[constant.AppInstanceLabelKey], clusterRestorePVCUID(pvc))
}

func clusterRestorePolicyError(ctx context.Context, cli client.Reader, namespace, name, uid string) error {
	if name == "" || uid == "" {
		return nil
	}
	cluster := &appsv1.Cluster{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, cluster); err != nil {
		return client.IgnoreNotFound(err)
	}
	if string(cluster.UID) != uid || cluster.Spec.Restore == nil {
		return nil
	}
	return dptypes.ValidateVolumeRestorePolicy(cluster.Spec.Restore.Parameters[dptypes.VolumeRestorePolicyParameterKey],
		"spec.restore.parameters["+dptypes.VolumeRestorePolicyParameterKey+"]")
}

// Execution children also check the original source intent. In particular,
// postReady Restores have no PrepareDataConfig of their own.
func executionRestorePolicyError(ctx context.Context, cli client.Reader, restore *dpv1alpha1.Restore) error {
	if err := configuredRestorePolicyError(restore); err != nil {
		return err
	}
	if owner := exactOwnerReference(restore.OwnerReferences, corev1.SchemeGroupVersion.String(), "PersistentVolumeClaim"); owner != nil &&
		restore.Name == getPopulatePVCName(owner.UID) && restore.Labels[dprestore.DataProtectionPopulatePVCLabelKey] == restore.Name {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(ctx, types.NamespacedName{Namespace: restore.Namespace, Name: owner.Name}, pvc); err != nil {
			return client.IgnoreNotFound(err)
		}
		if pvc.UID == owner.UID {
			return client.IgnoreNotFound(pvcRestorePolicyError(ctx, cli, pvc))
		}
	}
	if internalPostReadyRestoreOwner(restore) == nil {
		return nil
	}
	clusterUID := restore.Labels[dptypes.ClusterUIDLabelKey]
	if err := clusterRestorePolicyError(ctx, cli, restore.Namespace, restore.Labels[constant.AppInstanceLabelKey], clusterUID); err != nil {
		return err
	}
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(ctx, pvcs, client.InNamespace(restore.Namespace), client.MatchingLabels{
		constant.AppInstanceLabelKey:    restore.Labels[constant.AppInstanceLabelKey],
		constant.KBAppComponentLabelKey: restore.Labels[constant.KBAppComponentLabelKey],
	}); err != nil {
		return err
	}
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if clusterUID != "" && isClusterRestorePVC(pvc) && clusterRestorePVCUID(pvc) == clusterUID &&
			pvc.Spec.DataSourceRef.Name == restore.Labels[dptypes.BackupNameLabelKey] {
			if err := pvcRestorePolicyError(ctx, cli, pvc); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (r *RestoreReconciler) rejectRestorePolicy(reqCtx intctrlutil.RequestCtx, restore *dpv1alpha1.Restore, policyErr error) (ctrl.Result, error) {
	mgr := dprestore.NewRestoreManager(restore, r.Recorder, r.Scheme, r.Client)
	drained, err := mgr.DrainExistingJobs(reqCtx)
	if err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}
	message := policyErr.Error()
	if !drained {
		message += "; waiting for existing restore Jobs and managers to finish; no new work will be started"
	}
	dprestore.SetRestoreValidationCondition(restore, dprestore.ReasonValidateFailed, message)
	if drained {
		restore.Status.Phase = dpv1alpha1.RestorePhaseFailed
		restore.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
		restore.Status.Duration = dprestore.GetRestoreDuration(restore.Status)
	} else if restore.Status.Phase == "" {
		restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
	}
	if !reflect.DeepEqual(mgr.OriginalRestore.Status, restore.Status) {
		if err := r.Client.Status().Patch(reqCtx.Ctx, restore, client.MergeFrom(mgr.OriginalRestore)); err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		r.Recorder.Event(restore, corev1.EventTypeWarning, dprestore.ReasonValidateFailed, message)
	}
	if !drained {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, message)
	}
	return intctrlutil.Reconciled()
}

func (r *VolumePopulatorReconciler) rejectPVCPolicy(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim, policyErr error) error {
	// Each execution Restore is reconciled by its own controller, including
	// source-policy rejection. Do not patch child status from this controller.
	list := &dpv1alpha1.RestoreList{}
	if err := r.Client.List(reqCtx.Ctx, list, client.InNamespace(pvc.Namespace)); err != nil {
		return err
	}
	for i := range list.Items {
		restore := &list.Items[i]
		owned := hasExactOwnerReference(restore.OwnerReferences, corev1.SchemeGroupVersion.String(), "PersistentVolumeClaim", pvc.Name, pvc.UID) &&
			restore.Name == getPopulatePVCName(pvc.UID) && restore.Labels[dprestore.DataProtectionPopulatePVCLabelKey] == restore.Name
		postReady := internalPostReadyRestoreOwner(restore) != nil && clusterRestorePVCUID(pvc) != "" &&
			restore.Labels[dptypes.ClusterUIDLabelKey] == clusterRestorePVCUID(pvc) &&
			restore.Labels[constant.KBAppComponentLabelKey] == pvc.Labels[constant.KBAppComponentLabelKey] &&
			pvc.Spec.DataSourceRef != nil && restore.Labels[dptypes.BackupNameLabelKey] == pvc.Spec.DataSourceRef.Name
		if (owned || postReady) && restore.Status.Phase != dpv1alpha1.RestorePhaseCompleted && restore.Status.Phase != dpv1alpha1.RestorePhaseFailed {
			message := policyErr.Error() + "; waiting for existing execution Restore to finish"
			if err := r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, message); err != nil {
				return err
			}
			return intctrlutil.NewRequeueError(reconcileInterval, message)
		}
	}
	if err := r.finishRejectedPVHandoff(reqCtx, pvc); err != nil {
		return err
	}
	if err := r.releaseTargetPVC(reqCtx, pvc); err != nil {
		return err
	}
	return intctrlutil.NewFatalError(policyErr.Error())
}

// Finish only an already-started handoff. Keeping the helper claim preserves
// data when rejection occurs before the PV has been handed to the target.
func (r *VolumePopulatorReconciler) finishRejectedPVHandoff(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) error {
	helper := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: pvc.Namespace, Name: getPopulatePVCName(pvc.UID)}, helper); err != nil {
		return client.IgnoreNotFound(err)
	}
	if helper.Labels[dprestore.DataProtectionPopulatePVCLabelKey] != helper.Name || helper.Spec.VolumeName == "" {
		return nil
	}
	pv := &corev1.PersistentVolume{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: helper.Spec.VolumeName}, pv); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !pvClaimRefMatchesPVC(pv.Spec.ClaimRef, pvc) && pvc.Spec.VolumeName != pv.Name {
		return nil
	}
	rebound, err := r.rebindPVCAndPV(reqCtx, helper, pvc)
	if err != nil {
		return err
	}
	if !rebound {
		return intctrlutil.NewRequeueError(reconcileInterval, fmt.Sprintf("waiting for interrupted PV handoff for %s/%s", pvc.Namespace, pvc.Name))
	}
	return nil
}
