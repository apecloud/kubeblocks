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

package dataprotection

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-helpers/storage/volume"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dperrors "github.com/apecloud/kubeblocks/pkg/dataprotection/errors"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

// VolumePopulatorReconciler reconciles a Restore object
type VolumePopulatorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *VolumePopulatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("volume-populator", req.NamespacedName),
		Recorder: r.Recorder,
	}

	// Get pvc
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, pvc); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.syncPVC(reqCtx, pvc); err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			r.Recorder.Event(pvc, corev1.EventTypeWarning, ReasonVolumePopulateFailed, err.Error())
			if patchErr := r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingFailed, err.Error()); patchErr != nil {
				return intctrlutil.RequeueWithError(patchErr, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		} else if intctrlutil.IsTargetError(err, dperrors.ErrorTypeWaitForExternalHandler) && r.ContainPopulatingCondition(pvc) {
			// ignore the error if external controller handles it.
			return intctrlutil.Reconciled()
		}
		return RecorderEventAndRequeue(reqCtx, r.Recorder, pvc, err)
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumePopulatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *VolumePopulatorReconciler) MatchToPopulate(pvc *corev1.PersistentVolumeClaim) (bool, error) {
	dataSourceRef := pvc.Spec.DataSourceRef
	if dataSourceRef == nil {
		// Ignore PVCs without a datasource
		return false, nil
	}
	apiGroup := ""
	if dataSourceRef.APIGroup != nil {
		apiGroup = *dataSourceRef.APIGroup
	}
	if apiGroup != dptypes.DataprotectionAPIGroup || dataSourceRef.Kind != dptypes.RestoreKind || dataSourceRef.Name == "" {
		// Ignore PVCs that aren't for this populator to handle
		return false, nil
	}
	if dataSourceRef.Namespace != nil && *dataSourceRef.Namespace != pvc.Namespace {
		message := fmt.Sprintf(`custom resource of restore "%s" should be in the same namespace as the persistentVolumeClaim's namespace.`, *dataSourceRef.Namespace)
		return false, intctrlutil.NewFatalError(message)
	}
	return true, nil
}

func (r *VolumePopulatorReconciler) syncPVC(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) error {
	matched, err := r.MatchToPopulate(pvc)
	if err != nil {
		return err
	}
	if !matched {
		return nil
	}
	restoreMgr, err := r.validateRestoreAndBuildMGR(reqCtx, pvc)
	if err != nil {
		return err
	}
	// if pvc has not bound pv, populate it.
	if pvc.Spec.VolumeName == "" {
		return r.Populate(reqCtx, pvc, restoreMgr)
	}
	return r.Cleanup(reqCtx, pvc)
}

func (r *VolumePopulatorReconciler) validateRestoreAndBuildMGR(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) (*dprestore.RestoreManager, error) {
	restore := &dpv1alpha1.Restore{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: pvc.Spec.DataSourceRef.Name,
		Namespace: pvc.Namespace}, restore); err != nil {
		return nil, err
	}
	if restore.Spec.PrepareDataConfig == nil || restore.Spec.PrepareDataConfig.DataSourceRef == nil {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(`spec.prepareDataConfig.datasourceRef of restore "%s" can not be empty`, restore.Name))
	}
	restoreMgr := dprestore.NewRestoreManager(restore, r.Recorder, r.Scheme)
	if err := dprestore.ValidateAndInitRestoreMGR(reqCtx, r.Client, restoreMgr); err != nil {
		return nil, err
	}
	return restoreMgr, nil
}

func (r *VolumePopulatorReconciler) Populate(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim, restoreMgr *dprestore.RestoreManager) error {
	wait, nodeName, err := r.waitForPVCSelectedNode(reqCtx, pvc)
	if err != nil || wait {
		return err
	}
	// Make sure the PVC finalizer is present
	if !slices.Contains(pvc.Finalizers, dptypes.DataProtectionFinalizerName) {
		pvcPatch := client.MergeFrom(pvc.DeepCopy())
		controllerutil.AddFinalizer(pvc, dptypes.DataProtectionFinalizerName)
		if err = r.Client.Patch(reqCtx.Ctx, pvc, pvcPatch); err != nil {
			return err
		}
	}
	if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingProcessing, "Populator started"); err != nil {
		return err
	}
	// set scheduling for restore
	restoreMgr.Restore.Spec.PrepareDataConfig.SchedulingSpec = dpv1alpha1.SchedulingSpec{
		Tolerations: []corev1.Toleration{
			{Operator: corev1.TolerationOpExists},
		},
	}
	if nodeName != "" {
		restoreMgr.Restore.Spec.PrepareDataConfig.SchedulingSpec.NodeSelector = map[string]string{
			corev1.LabelHostname: nodeName,
		}
	}
	var populatePVC *corev1.PersistentVolumeClaim
	for i, v := range restoreMgr.PrepareDataBackupSets {
		if populatePVC == nil {
			populatePVC, err = r.getPopulatePVC(reqCtx, pvc, v,
				restoreMgr.Restore.Spec.PrepareDataConfig.DataSourceRef.VolumeSource, nodeName)
			if err != nil {
				return err
			}
		}

		// 1. build populate job
		job, err := restoreMgr.BuildVolumePopulateJob(reqCtx, r.Client, v, populatePVC, i)
		if err != nil {
			return err
		}
		if job == nil {
			continue
		}

		// 2. create job
		jobs, err := restoreMgr.CreateJobsIfNotExist(reqCtx, r.Client, pvc, []*batchv1.Job{job})
		if err != nil {
			return err
		}

		// 3. check if jobs are finished.
		isCompleted, _, errMsg := utils.IsJobFinished(jobs[0])
		if !isCompleted {
			return nil
		}
		if errMsg != "" {
			return intctrlutil.NewFatalError(errMsg)
		}
	}
	// 4. if jobs are succeed, rebind the pvc and pv
	if err = r.rebindPVCAndPV(reqCtx, populatePVC, pvc); err != nil {
		return err
	}
	if err = r.UpdatePVCConditions(reqCtx, pvc, ReasonPopulatingSucceed, "Populator finished"); err != nil {
		return err
	}
	return nil
}

func (r *VolumePopulatorReconciler) Cleanup(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) error {
	if slices.Contains(pvc.Finalizers, dptypes.DataProtectionFinalizerName) {
		pvcPatch := client.MergeFrom(pvc.DeepCopy())
		controllerutil.RemoveFinalizer(pvc, dptypes.DataProtectionFinalizerName)
		if err := r.Client.Patch(reqCtx.Ctx, pvc, pvcPatch); err != nil {
			return err
		}
	}

	jobs := &batchv1.JobList{}
	if err := r.Client.List(reqCtx.Ctx, jobs,
		client.InNamespace(pvc.Namespace), client.MatchingLabels(map[string]string{
			dprestore.DataProtectionPopulatePVCLabelKey: getPopulatePVCName(pvc.UID),
		})); err != nil {
		return err
	}

	for i := range jobs.Items {
		job := &jobs.Items[i]
		if controllerutil.ContainsFinalizer(job, dptypes.DataProtectionFinalizerName) {
			patch := client.MergeFrom(job.DeepCopy())
			controllerutil.RemoveFinalizer(job, dptypes.DataProtectionFinalizerName)
			if err := r.Patch(reqCtx.Ctx, job, patch); err != nil {
				return err
			}
		}
		if !job.DeletionTimestamp.IsZero() {
			continue
		}
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, job); err != nil {
			return err
		}
	}

	populatePVC := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: getPopulatePVCName(pvc.UID),
		Namespace: pvc.Namespace}, populatePVC); err != nil {
		return client.IgnoreNotFound(err)
	}
	return r.Client.Delete(reqCtx.Ctx, populatePVC)
}

func (r *VolumePopulatorReconciler) checkIntreeStorageClass(pvc *corev1.PersistentVolumeClaim, sc *storagev1.StorageClass) error {
	if !strings.HasPrefix(sc.Provisioner, "kubernetes.io/") {
		// This is not an in-tree StorageClass
		return nil
	}

	if pvc.Annotations != nil {
		if migrated := pvc.Annotations[volume.AnnMigratedTo]; migrated != "" {
			// The PVC is migrated to CSI
			return nil
		}
	}
	// The SC is in-tree & PVC is not migrated
	return intctrlutil.NewFatalError(fmt.Sprintf("in-tree volume volume plugin %q cannot use volume populator", sc.Provisioner))
}

func (r *VolumePopulatorReconciler) waitForPVCSelectedNode(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim) (bool, string, error) {
	var nodeName string
	if pvc.Spec.StorageClassName != nil {
		storageClassName := *pvc.Spec.StorageClassName
		storageClass := &storagev1.StorageClass{}
		if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: storageClassName}, storageClass); err != nil {
			return false, nodeName, err
		}

		if err := r.checkIntreeStorageClass(pvc, storageClass); err != nil {
			return false, nodeName, err
		}
		if storageClass.VolumeBindingMode != nil && storagev1.VolumeBindingWaitForFirstConsumer == *storageClass.VolumeBindingMode {
			nodeName = pvc.Annotations[AnnSelectedNode]
			if nodeName == "" {
				// Wait for the PVC to get a node name before continuing
				return true, nodeName, nil
			}
		}
	}
	return false, nodeName, nil
}

func (r *VolumePopulatorReconciler) getPopulatePVC(reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backupSet dprestore.BackupActionSet,
	volumeSource,
	nodeName string) (*corev1.PersistentVolumeClaim, error) {
	populatePVCName := getPopulatePVCName(pvc.UID)
	populatePVC := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: populatePVCName,
		Namespace: pvc.Namespace}, populatePVC); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		// create populate pvc
		populatePVC = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      populatePVCName,
				Namespace: pvc.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      pvc.Spec.AccessModes,
				Resources:        pvc.Spec.Resources,
				StorageClassName: pvc.Spec.StorageClassName,
				VolumeMode:       pvc.Spec.VolumeMode,
			},
		}
		if nodeName != "" {
			populatePVC.Annotations = map[string]string{
				AnnSelectedNode: pvc.Annotations[AnnSelectedNode],
			}
		}
		if backupSet.UseVolumeSnapshot {
			// restore from volume snapshot.
			populatePVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
				Name:     utils.GetBackupVolumeSnapshotName(backupSet.Backup.Name, volumeSource),
				Kind:     constant.VolumeSnapshotKind,
				APIGroup: &dprestore.VolumeSnapshotGroup,
			}
		}
		if err = r.Client.Create(reqCtx.Ctx, populatePVC); err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}
	return populatePVC, nil
}

func (r *VolumePopulatorReconciler) rebindPVCAndPV(reqCtx intctrlutil.RequestCtx, populatePVC, pvc *corev1.PersistentVolumeClaim) error {
	pv := &corev1.PersistentVolume{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{Name: populatePVC.Spec.VolumeName, Namespace: pvc.Namespace}, pv); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// We'll get called again later when the PV exists
		return nil
	}
	// Examine the claimref for the PV and see if it's bound to the correct PVC
	claimRef := pv.Spec.ClaimRef
	if claimRef.Name == pvc.Name && claimRef.Namespace == pvc.Namespace && claimRef.UID == pvc.UID {
		return nil
	}
	// Make new PV with strategic patch values to perform the PV rebind
	patchPV := client.MergeFrom(pv.DeepCopy())
	pv.Spec.ClaimRef = &corev1.ObjectReference{
		Namespace:       pvc.Namespace,
		Name:            pvc.Name,
		UID:             pvc.UID,
		ResourceVersion: pvc.ResourceVersion,
	}
	if pv.Annotations == nil {
		pv.Annotations = map[string]string{}
	}
	pv.Annotations[AnnPopulateFrom] = pvc.Spec.DataSourceRef.Name
	return r.Client.Patch(reqCtx.Ctx, pv, patchPV)
}

func (r *VolumePopulatorReconciler) UpdatePVCConditions(reqCtx intctrlutil.RequestCtx, pvc *corev1.PersistentVolumeClaim, reason, message string) error {
	progressCondition := corev1.PersistentVolumeClaimCondition{
		Type:               PersistentVolumeClaimPopulating,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	pvcPatch := client.MergeFrom(pvc.DeepCopy())
	var existPopulating bool
	for i, v := range pvc.Status.Conditions {
		if v.Type != PersistentVolumeClaimPopulating {
			continue
		}
		if reason == v.Reason {
			return nil
		}
		if v.Reason == ReasonPopulatingSucceed {
			// ignore succeed condition
			return nil
		}
		existPopulating = true
		pvc.Status.Conditions[i] = progressCondition
	}
	if !existPopulating {
		pvc.Status.Conditions = append(pvc.Status.Conditions, progressCondition)
	}
	switch reason {
	case ReasonPopulatingProcessing:
		r.Recorder.Event(pvc, corev1.EventTypeNormal, ReasonStartToVolumePopulate, message)
	case ReasonPopulatingSucceed:
		r.Recorder.Event(pvc, corev1.EventTypeNormal, ReasonVolumePopulateSucceed, message)
	}
	return r.Client.Status().Patch(reqCtx.Ctx, pvc, pvcPatch)
}

func (r *VolumePopulatorReconciler) ContainPopulatingCondition(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc == nil {
		return false
	}
	for _, v := range pvc.Status.Conditions {
		if v.Type == PersistentVolumeClaimPopulating {
			return true
		}
	}
	return false
}
