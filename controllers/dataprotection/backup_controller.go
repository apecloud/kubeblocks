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
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	ctrlbuilder "github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	backupPathBase                 = "/backupdata"
	deleteBackupFilesJobNamePrefix = "delete-"
)

var (
	// errBreakReconcile is not a real error, it is used to break the current reconciliation
	errBreakReconcile = errors.New("break reconcile")
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme      *k8sruntime.Scheme
	Recorder    record.EventRecorder
	clock       clock.RealClock
	snapshotCli *intctrlutil.VolumeSnapshotCompatClient
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/finalizers,verbs=update

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotclasses/finalizers,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// NOTES:
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backup", req.NamespacedName),
		Recorder: r.Recorder,
	}
	// initialize snapshotCompatClient
	r.snapshotCli = &intctrlutil.VolumeSnapshotCompatClient{
		Client: r.Client,
		Ctx:    ctx,
	}
	// Get backup obj
	backup := &dataprotectionv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backup); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	reqCtx.Log.V(1).Info("in Backup Reconciler:", "backup", backup.Name, "phase", backup.Status.Phase)

	// handle deletion
	res, err := r.handleBackupDeletion(reqCtx, backup)
	if res != nil {
		return *res, err
	}

	switch backup.Status.Phase {
	case "", dataprotectionv1alpha1.BackupNew:
		return r.doNewPhaseAction(reqCtx, backup)
	case dataprotectionv1alpha1.BackupInProgress:
		return r.doInProgressPhaseAction(reqCtx, backup)
	case dataprotectionv1alpha1.BackupRunning:
		if err = r.doInRunningPhaseAction(reqCtx, backup); err != nil {
			sendWarningEventForError(r.Recorder, backup, err)
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	case dataprotectionv1alpha1.BackupCompleted:
		return r.doCompletedPhaseAction(reqCtx, backup)
	default:
		return intctrlutil.Reconciled()
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {

	b := ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.Backup{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurDataProtectionReconKey),
		}).
		Owns(&batchv1.Job{}).
		Owns(&appsv1.StatefulSet{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(r.filterBackupPods))

	if viper.GetBool("VOLUMESNAPSHOT") {
		if intctrlutil.InVolumeSnapshotV1Beta1() {
			b.Owns(&snapshotv1beta1.VolumeSnapshot{}, builder.Predicates{})
		} else {
			b.Owns(&snapshotv1.VolumeSnapshot{}, builder.Predicates{})
		}
	}

	return b.Complete(r)
}

// checkPodsOfStatefulSetHasDeleted checks if the pods of statefulSet have been deleted
func (r *BackupReconciler) checkPodsOfStatefulSetHasDeleted(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) (bool, error) {
	podList := &corev1.PodList{}
	if err := r.Client.List(reqCtx.Ctx, podList, client.MatchingLabels(buildBackupWorkloadsLabels(backup))); err != nil {
		return false, err
	}
	for _, pod := range podList.Items {
		for _, owner := range pod.OwnerReferences {
			// checks if the pod is owned by sts
			if owner.Kind == constant.StatefulSetKind && owner.Name == backup.Name {
				return false, nil
			}
		}
	}
	return true, nil
}

// handleBackupDeleting handles the Deleting phase of backup.
func (r *BackupReconciler) handleBackupDeleting(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
	hasDeleted, err := r.checkPodsOfStatefulSetHasDeleted(reqCtx, backup)
	if err != nil {
		return err
	}
	// wait for pods of sts clean up successfully
	if !hasDeleted {
		return nil
	}
	deleteFileJob, err := r.handleDeleteBackupFiles(reqCtx, backup)
	if err != nil {
		return err
	}
	deleteBackup := func() error {
		// remove backup finalizers to delete it
		patch := client.MergeFrom(backup.DeepCopy())
		controllerutil.RemoveFinalizer(backup, dataProtectionFinalizerName)
		return r.Patch(reqCtx.Ctx, backup, patch)
	}
	// if deleteFileJob is nil, do not to delete backup files
	if deleteFileJob == nil {
		return deleteBackup()
	}
	if containsJobCondition(deleteFileJob, batchv1.JobComplete) {
		return deleteBackup()
	}
	if containsJobCondition(deleteFileJob, batchv1.JobFailed) {
		failureReason := fmt.Sprintf(`the job "%s" for backup files deletion failed, you can delete it to re-delete the files`, deleteFileJob.Name)
		if backup.Status.FailureReason == failureReason {
			return nil
		}
		backupPatch := client.MergeFrom(backup.DeepCopy())
		backup.Status.FailureReason = failureReason
		r.Recorder.Event(backup, corev1.EventTypeWarning, "DeleteBackupFilesFailed", failureReason)
		return r.Status().Patch(reqCtx.Ctx, backup, backupPatch)
	}
	// wait for the deletion job completed
	return nil
}

func (r *BackupReconciler) handleBackupDeletion(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) (*ctrl.Result, error) {
	if backup.Status.Phase == dataprotectionv1alpha1.BackupDeleting {
		// handle deleting
		if err := r.handleBackupDeleting(reqCtx, backup); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	if !backup.GetDeletionTimestamp().IsZero() {
		if err := r.deleteExternalResources(reqCtx, backup); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
		}
		// backup phase to Deleting
		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status.Phase = dataprotectionv1alpha1.BackupDeleting
		if err := r.Client.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	return nil, nil
}

func (r *BackupReconciler) filterBackupPods(obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if v, ok := labels[constant.AppManagedByLabelKey]; !ok || v != constant.AppName {
		return []reconcile.Request{}
	}
	backupName, ok := labels[constant.DataProtectionLabelBackupNameKey]
	if !ok {
		return []reconcile.Request{}
	}
	var isCreateByStatefulSet bool
	for _, v := range obj.GetOwnerReferences() {
		if v.Kind == constant.StatefulSetKind && v.Name == backupName {
			isCreateByStatefulSet = true
			break
		}
	}
	if !isCreateByStatefulSet {
		return []reconcile.Request{}
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      backupName,
			},
		},
	}
}

func (r *BackupReconciler) getBackupPolicyAndValidate(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (*dataprotectionv1alpha1.BackupPolicy, error) {
	// get referenced backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Spec.BackupPolicyName,
	}
	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		return nil, err
	}

	if len(backupPolicy.Name) == 0 {
		return nil, intctrlutil.NewNotFound(`backup policy "%s" not found`, backupPolicyNameSpaceName)
	}

	// validate backup spec
	if err := backup.Spec.Validate(backupPolicy); err != nil {
		return nil, err
	}
	return backupPolicy, nil
}

func (r *BackupReconciler) validateLogfileBackupLegitimacy(backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	backupType := backup.Spec.BackupType
	if backupType != dataprotectionv1alpha1.BackupTypeLogFile {
		return nil
	}
	if backup.Name != getCreatedCRNameByBackupPolicy(backupPolicy, backupType) {
		return intctrlutil.NewInvalidLogfileBackupName(backupPolicy.Name)
	}
	if backupPolicy.Spec.Schedule.Logfile == nil {
		return intctrlutil.NewBackupNotSupported(string(backupType), backupPolicy.Name)
	}
	if !backupPolicy.Spec.Schedule.Logfile.Enable {
		return intctrlutil.NewBackupScheduleDisabled(string(backupType), backupPolicy.Name)
	}
	return nil
}

func (r *BackupReconciler) doNewPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (ctrl.Result, error) {

	patch := client.MergeFrom(backup.DeepCopy())
	// HACK/TODO: ought to move following check to validation webhook
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot && !viper.GetBool("VOLUMESNAPSHOT") {
		backup.Status.Phase = dataprotectionv1alpha1.BackupFailed
		backup.Status.FailureReason = "VolumeSnapshot feature disabled."
		if err := r.Client.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	backupPolicy, err := r.getBackupPolicyAndValidate(reqCtx, backup)
	if err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	}

	if err = r.validateLogfileBackupLegitimacy(backup, backupPolicy); err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	}

	updateLabels := map[string]string{}

	// TODO: get pod with matching labels to do backup.
	var targetCluster dataprotectionv1alpha1.TargetCluster
	var isStatefulSetKind bool
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		targetCluster = backupPolicy.Spec.Snapshot.Target
	} else {
		commonPolicy := backupPolicy.Spec.GetCommonPolicy(backup.Spec.BackupType)
		if commonPolicy == nil {
			return r.updateStatusIfFailed(reqCtx, backup, intctrlutil.NewBackupNotSupported(string(backup.Spec.BackupType), backupPolicy.Name))
		}
		targetCluster = commonPolicy.Target
		backupTool, err := getBackupToolByName(reqCtx, r.Client, commonPolicy.BackupToolName)
		if err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, intctrlutil.NewNotFound("backupTool: %s not found", commonPolicy.BackupToolName))
		}
		if err = r.buildBackupStatusForBackupTool(reqCtx, backup, backupPolicy, commonPolicy, backupTool, updateLabels); err != nil {
			if errors.Is(err, errBreakReconcile) {
				// wait for the PVC to be created
				return intctrlutil.Reconciled()
			}
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
		isStatefulSetKind = backupTool.Spec.DeployKind == dataprotectionv1alpha1.DeployKindStatefulSet
	}
	// clean cached annotations if in NEW phase
	backupCopy := backup.DeepCopy()
	if backupCopy.Annotations[dataProtectionBackupTargetPodKey] != "" {
		delete(backupCopy.Annotations, dataProtectionBackupTargetPodKey)
	}
	target, err := r.getTargetPod(reqCtx, backupCopy, targetCluster.LabelsSelector.MatchLabels)
	if err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	}

	cluster := r.getCluster(reqCtx, target)
	if hasPatch, err := r.patchBackupObjectMeta(reqCtx, backup, target, cluster, updateLabels); err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	} else if hasPatch {
		return intctrlutil.Reconciled()
	}

	// clean up failed job if backup type is logfile
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeLogFile {
		if err = r.cleanupFailedJob(reqCtx, backup); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}

	// update Phase to InProgress/Running
	if isStatefulSetKind {
		backup.Status.Phase = dataprotectionv1alpha1.BackupRunning
	} else {
		backup.Status.Phase = dataprotectionv1alpha1.BackupInProgress
	}
	backup.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	if backupPolicy.Spec.Retention != nil && backupPolicy.Spec.Retention.TTL != nil {
		backup.Status.Expiration = &metav1.Time{
			Time: backup.Status.StartTimestamp.Add(dataprotectionv1alpha1.ToDuration(backupPolicy.Spec.Retention.TTL)),
		}
	}

	if cluster != nil {
		backup.Status.SourceCluster = cluster.Name
	}
	if err = r.Client.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupReconciler) buildBackupStatusForBackupTool(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	backupTool *dataprotectionv1alpha1.BackupTool,
	updateLabels map[string]string) error {
	if backup.Status.Manifests == nil {
		backup.Status.Manifests = &dataprotectionv1alpha1.ManifestsStatus{}
	}
	if backup.Status.Manifests.BackupTool == nil {
		backup.Status.Manifests.BackupTool = &dataprotectionv1alpha1.BackupToolManifestsStatus{}
	}
	// handle the PVC used in this backup
	if backup.Status.PersistentVolumeClaimName == "" {
		pvcName, pvName, err := r.handlePersistentVolumeClaim(reqCtx, backup, backupPolicy.Name, commonPolicy, updateLabels)
		if err != nil {
			return err
		}
		// record volume name
		backup.Status.PersistentVolumeClaimName = pvcName
		backup.Status.Manifests.BackupTool.VolumeName = pvName
	}
	// save the backup message for restore
	backup.Status.BackupToolName = backupTool.Name
	backupDestinationPath := getBackupDestinationPath(backup, backupPolicy.Annotations[constant.BackupDataPathPrefixAnnotationKey])
	backup.Status.Manifests.BackupTool.FilePath = backupDestinationPath

	if backupTool.Spec.Physical.IsRelyOnLogfile() {
		if backupPolicy.Spec.Schedule.Logfile == nil || !backupPolicy.Spec.Schedule.Logfile.Enable {
			return intctrlutil.NewBackupLogfileScheduleDisabled(backupTool.Name)
		}
		logfileBackupName := getCreatedCRNameByBackupPolicy(backupPolicy, dataprotectionv1alpha1.BackupTypeLogFile)
		backup.Status.Manifests.BackupTool.LogFilePath = getBackupDestinationPath(&dataprotectionv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{Namespace: backup.Namespace, Name: logfileBackupName},
		}, backupPolicy.Annotations[constant.BackupDataPathPrefixAnnotationKey])

		logFilePvcName, _, err := r.handlePersistentVolumeClaim(reqCtx, backup, backupPolicy.Name, backupPolicy.Spec.Logfile, updateLabels)
		if err != nil {
			return err
		}
		backup.Status.LogFilePersistentVolumeClaimName = logFilePvcName
	}
	return nil
}

func (r *BackupReconciler) cleanupFailedJob(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
	jobList := batchv1.JobList{}
	if err := r.Client.List(reqCtx.Ctx, &jobList, client.InNamespace(backup.Namespace),
		client.MatchingLabels{constant.DataProtectionLabelBackupNameKey: backup.Name}); err != nil {
		return nil
	}

	for _, job := range jobList.Items {
		if !containsJobCondition(&job, batchv1.JobFailed) {
			continue
		}
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &job); err != nil {
			return err
		}
		if controllerutil.ContainsFinalizer(&job, dataProtectionFinalizerName) {
			patch := client.MergeFrom(job.DeepCopy())
			controllerutil.RemoveFinalizer(&job, dataProtectionFinalizerName)
			if err := r.Patch(reqCtx.Ctx, &job, patch); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *BackupReconciler) handlePersistentVolumeClaim(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicyName string,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	updateLabels map[string]string) (pvcName string, pvName string, err error) {
	// check the PVC from the backup repo
	pvcName, pvName, err = r.handlePVCByBackupRepo(reqCtx, backup, backupPolicyName, commonPolicy, updateLabels)
	if err == nil || !errors.Is(err, errNoDefaultBackupRepo) {
		return pvcName, pvName, err
	}

	// fallback to the legacy PVC field for compatibility
	if commonPolicy.PersistentVolumeClaim.Name != nil {
		pvcName = *commonPolicy.PersistentVolumeClaim.Name
	}
	pvName, err = r.handlePersistentVolumeClaimLegacy(reqCtx, backup.Spec.BackupType, backupPolicyName, commonPolicy)
	return pvcName, pvName, err
}

func (r *BackupReconciler) handlePVCByBackupRepo(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicyName string,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	updateLabels map[string]string) (pvcName string, pvName string, err error) {
	// check the PVC from backup repo
	repo, err := r.getBackupRepo(reqCtx, backup, commonPolicy)
	if err != nil {
		return "", "", err
	}
	pvcName = repo.Status.BackupPVCName
	if pvcName == "" {
		err = intctrlutil.NewBackupPVCNameIsEmpty(string(backup.Spec.BackupType), backupPolicyName)
		return "", "", err
	}
	pvc := &corev1.PersistentVolumeClaim{}
	err = r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: reqCtx.Req.Namespace,
		Name:      pvcName,
	}, pvc)
	if err != nil && !apierrors.IsNotFound(err) {
		// error occurred
		return "", "", err
	}
	if err == nil {
		// the PVC is already present, bind the backup to the repo
		updateLabels[dataProtectionBackupRepoKey] = repo.Name
		return pvcName, pvc.Spec.VolumeName, nil
	}
	// the PVC is not present
	// add a special label and wait for the backup repo controller to create the PVC.
	// we need to update the object meta immediately, because we are going to break the current reconciliation.
	_, err = r.patchBackupObjectLabels(reqCtx, backup, map[string]string{
		dataProtectionBackupRepoKey:  repo.Name,
		dataProtectionNeedRepoPVCKey: trueVal,
	})
	if err != nil {
		return "", "", err
	}
	return "", "", errBreakReconcile
}

// handlePersistentVolumeClaimLegacy handles the persistent volume claim for the backup, the rules are as follows
// - if CreatePolicy is "Never", it will check if the pvc exists. if not existed, then report an error.
// - if CreatePolicy is "IfNotPresent" and the pvc not existed, then create the pvc automatically.
func (r *BackupReconciler) handlePersistentVolumeClaimLegacy(reqCtx intctrlutil.RequestCtx,
	backupType dataprotectionv1alpha1.BackupType,
	backupPolicyName string,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) (string, error) {
	pvcConfig := commonPolicy.PersistentVolumeClaim
	if pvcConfig.Name == nil || len(*pvcConfig.Name) == 0 {
		return "", intctrlutil.NewBackupPVCNameIsEmpty(string(backupType), backupPolicyName)
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Namespace: reqCtx.Req.Namespace,
		Name: *pvcConfig.Name}, pvc); err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}
	if len(pvc.Name) > 0 {
		return pvc.Spec.VolumeName, nil
	}
	if pvcConfig.CreatePolicy == dataprotectionv1alpha1.CreatePVCPolicyNever {
		return "", intctrlutil.NewNotFound(`persistent volume claim "%s" not found`, *pvcConfig.Name)
	}
	if pvcConfig.PersistentVolumeConfigMap != nil &&
		(pvcConfig.StorageClassName == nil || *pvcConfig.StorageClassName == "") {
		// if the storageClassName is empty and the PersistentVolumeConfigMap is not empty,
		// create the persistentVolume with the template
		if err := r.createPersistentVolumeWithTemplate(reqCtx, backupPolicyName, &pvcConfig); err != nil {
			return "", err
		}
	}
	return "", r.createPVCWithStorageClassName(reqCtx, backupPolicyName, pvcConfig)
}

// getBackupRepo returns the backup repo specified by the backup object or the policy.
// if no backup repo specified, it will return the default one.
func (r *BackupReconciler) getBackupRepo(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) (*dataprotectionv1alpha1.BackupRepo, error) {
	// use the specified backup repo
	var repoName string
	if val := backup.Labels[dataProtectionBackupRepoKey]; val != "" {
		repoName = val
	} else if commonPolicy.BackupRepoName != nil && *commonPolicy.BackupRepoName != "" {
		repoName = *commonPolicy.BackupRepoName
	}
	if repoName != "" {
		repo := &dataprotectionv1alpha1.BackupRepo{}
		err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: repoName}, repo)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, intctrlutil.NewNotFound("backup repo %s not found", repoName)
			}
			return nil, err
		}
		return repo, nil
	}
	// fallback to use the default repo
	return getDefaultBackupRepo(reqCtx.Ctx, r.Client)
}

// createPVCWithStorageClassName creates the persistent volume claim with the storageClassName.
func (r *BackupReconciler) createPVCWithStorageClassName(reqCtx intctrlutil.RequestCtx,
	backupPolicyName string,
	pvcConfig dataprotectionv1alpha1.PersistentVolumeClaim) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        *pvcConfig.Name,
			Namespace:   reqCtx.Req.Namespace,
			Annotations: buildAutoCreationAnnotations(backupPolicyName),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: pvcConfig.StorageClassName,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: pvcConfig.InitCapacity,
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
		},
	}
	err := r.Client.Create(reqCtx.Ctx, pvc)
	return client.IgnoreAlreadyExists(err)
}

// createPersistentVolumeWithTemplate creates the persistent volume with the template.
func (r *BackupReconciler) createPersistentVolumeWithTemplate(reqCtx intctrlutil.RequestCtx,
	backupPolicyName string,
	pvcConfig *dataprotectionv1alpha1.PersistentVolumeClaim) error {
	pvConfig := pvcConfig.PersistentVolumeConfigMap
	configMap := &corev1.ConfigMap{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Namespace: pvConfig.Namespace,
		Name: pvConfig.Name}, configMap); err != nil {
		return err
	}
	pvTemplate := configMap.Data[persistentVolumeTemplateKey]
	if pvTemplate == "" {
		return intctrlutil.NewBackupPVTemplateNotFound(pvConfig.Namespace, pvConfig.Name)
	}
	pvName := fmt.Sprintf("%s-%s", *pvcConfig.Name, reqCtx.Req.Namespace)
	pvTemplate = strings.ReplaceAll(pvTemplate, "$(GENERATE_NAME)", pvName)
	pv := &corev1.PersistentVolume{}
	if err := yaml.Unmarshal([]byte(pvTemplate), pv); err != nil {
		return err
	}
	pv.Name = pvName
	pv.Spec.ClaimRef = &corev1.ObjectReference{
		Namespace: reqCtx.Req.Namespace,
		Name:      *pvcConfig.Name,
	}
	pv.Annotations = buildAutoCreationAnnotations(backupPolicyName)
	// set the storageClassName to empty for the persistentVolumeClaim to avoid the dynamic provisioning
	emptyStorageClassName := ""
	pvcConfig.StorageClassName = &emptyStorageClassName
	controllerutil.AddFinalizer(pv, dataProtectionFinalizerName)
	return r.Client.Create(reqCtx.Ctx, pv)
}

func (r *BackupReconciler) doInProgressPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (ctrl.Result, error) {
	backupPolicy, err := r.getBackupPolicyAndValidate(reqCtx, backup)
	if err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	}
	backupDestinationPath := getBackupDestinationPath(backup, backupPolicy.Annotations[constant.BackupDataPathPrefixAnnotationKey])
	patch := client.MergeFrom(backup.DeepCopy())
	var res *ctrl.Result
	switch backup.Spec.BackupType {
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		res, err = r.doSnapshotInProgressPhaseAction(reqCtx, backup, backupPolicy, backupDestinationPath)
	default:
		res, err = r.doBaseBackupInProgressPhaseAction(reqCtx, backup, backupPolicy, backupDestinationPath)
	}

	if res != nil {
		return *res, err
	} else if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	// finally, update backup status
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedBackup", "Completed backup.")
	if backup.Status.CompletionTimestamp != nil {
		// round the duration to a multiple of seconds.
		duration := backup.Status.CompletionTimestamp.Sub(backup.Status.StartTimestamp.Time).Round(time.Second)
		backup.Status.Duration = &metav1.Duration{Duration: duration}
	}
	if err := r.Client.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

// doSnapshotInProgressPhaseAction handles for snapshot backup during in progress.
func (r *BackupReconciler) doSnapshotInProgressPhaseAction(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	backupDestinationPath string) (*ctrl.Result, error) {
	// 1. create and ensure pre-command job completed
	// 2. create and ensure volume snapshot ready
	// 3. create and ensure post-command job completed
	snapshotSpec := backupPolicy.Spec.Snapshot
	isOK, err := r.createPreCommandJobAndEnsure(reqCtx, backup, snapshotSpec)
	if err != nil {
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, err))
	}
	if !isOK {
		return intctrlutil.ResultToP(intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, ""))
	}
	if err = r.createUpdatesJobs(reqCtx, backup, nil, &snapshotSpec.BasePolicy, backupDestinationPath, dataprotectionv1alpha1.PRE); err != nil {
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPreUpdatesJob", err.Error())
	}
	if err = r.createVolumeSnapshot(reqCtx, backup, backupPolicy.Spec.Snapshot); err != nil {
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, err))
	}

	key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backup.Name}
	isOK, err = r.ensureVolumeSnapshotReady(key)
	if err != nil {
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, err))
	}
	if !isOK {
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	msg := fmt.Sprintf("Created volumeSnapshot %s ready.", key.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedVolumeSnapshot", msg)

	isOK, err = r.createPostCommandJobAndEnsure(reqCtx, backup, snapshotSpec)
	if err != nil {
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, err))
	}
	if !isOK {
		return intctrlutil.ResultToP(intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, ""))
	}

	// Failure MetadataCollectionJob does not affect the backup status.
	if err = r.createUpdatesJobs(reqCtx, backup, nil, &snapshotSpec.BasePolicy, backupDestinationPath, dataprotectionv1alpha1.POST); err != nil {
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPostUpdatesJob", err.Error())
	}

	backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
	backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	snap := &snapshotv1.VolumeSnapshot{}
	exists, _ := r.snapshotCli.CheckResourceExists(key, snap)
	if exists {
		backup.Status.TotalSize = snap.Status.RestoreSize.String()
	}
	return nil, nil
}

// doBaseBackupInProgressPhaseAction handles for base backup during in progress.
func (r *BackupReconciler) doBaseBackupInProgressPhaseAction(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	backupDestinationPath string) (*ctrl.Result, error) {
	// 1. create and ensure backup tool job finished
	// 2. get job phase and update
	commonPolicy := backupPolicy.Spec.GetCommonPolicy(backup.Spec.BackupType)
	if commonPolicy == nil {
		// TODO: add error type
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, fmt.Errorf("not found the %s policy", backup.Spec.BackupType)))
	}
	// createUpdatesJobs should not affect the backup status, just need to record events when the run fails
	if err := r.createUpdatesJobs(reqCtx, backup, commonPolicy, &commonPolicy.BasePolicy, backupDestinationPath, dataprotectionv1alpha1.PRE); err != nil {
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPreUpdatesJob", err.Error())
	}
	if err := r.createBackupToolJob(reqCtx, backup, backupPolicy, commonPolicy, backupDestinationPath); err != nil {
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, err))
	}
	key := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}
	isOK, err := r.ensureBatchV1JobCompleted(reqCtx, key)
	if err != nil {
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, err))
	}
	if !isOK {
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	// createUpdatesJobs should not affect the backup status, just need to record events when the run fails
	if err = r.createUpdatesJobs(reqCtx, backup, commonPolicy, &commonPolicy.BasePolicy, backupDestinationPath, dataprotectionv1alpha1.POST); err != nil {
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPostUpdatesJob", err.Error())
	}
	// updates Phase directly to Completed because `ensureBatchV1JobCompleted` has checked job failed
	backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
	backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}

	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeLogFile {
		if backup.Status.Manifests != nil &&
			backup.Status.Manifests.BackupLog != nil &&
			backup.Status.Manifests.BackupLog.StartTime == nil {
			backup.Status.Manifests.BackupLog.StartTime = backup.Status.Manifests.BackupLog.StopTime
		}
	}
	return nil, nil
}

func (r *BackupReconciler) doInRunningPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) error {
	backupPolicy, isCompleted, err := r.checkBackupIsCompletedDuringRunning(reqCtx, backup)
	if err != nil {
		return err
	} else if isCompleted {
		return nil
	}
	commonPolicy := backupPolicy.Spec.GetCommonPolicy(backup.Spec.BackupType)
	if commonPolicy == nil {
		return fmt.Errorf(`can not find spec.%s in BackupPolicy "%s"`, strings.ToLower(string(backup.Spec.BackupType)), backupPolicy.Name)
	}
	// reconcile StatefulSet
	sts := &appsv1.StatefulSet{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, types.NamespacedName{
		Namespace: backup.Namespace,
		Name:      backup.Name,
	}, sts)
	if err != nil {
		return err
	}
	statefulSetSpec, err := r.buildStatefulSpec(reqCtx, backup, backupPolicy, commonPolicy)
	if err != nil {
		return err
	}
	// if not exists, create the statefulSet
	if !exists {
		return r.createBackupStatefulSet(reqCtx, backup, statefulSetSpec)
	}
	sts.Spec.Template = statefulSetSpec.Template
	// update the statefulSet
	if err = r.Update(reqCtx.Ctx, sts); err != nil {
		return err
	}
	// if available replicas not changed, return
	if backup.Status.AvailableReplicas != nil && *backup.Status.AvailableReplicas == sts.Status.AvailableReplicas {
		return nil
	}
	patch := client.MergeFrom(backup.DeepCopy())
	backup.Status.AvailableReplicas = &sts.Status.AvailableReplicas
	return r.Status().Patch(reqCtx.Ctx, backup, patch)
}

// checkBackupIsCompletedDuringRunning checks if backup is completed during it is running.
// it returns ture, if logfile schedule is disabled or cluster is deleted.
func (r *BackupReconciler) checkBackupIsCompletedDuringRunning(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (*dataprotectionv1alpha1.BackupPolicy, bool, error) {
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Spec.BackupPolicyName,
	}, backupPolicy)
	if err != nil {
		return backupPolicy, false, err
	}
	if exists {
		if err = backup.Spec.Validate(backupPolicy); err != nil {
			return backupPolicy, false, err
		}
		clusterName := backup.Labels[constant.AppInstanceLabelKey]
		targetClusterExists := true
		if clusterName != "" {
			cluster := &appsv1alpha1.Cluster{}
			var err error
			targetClusterExists, err = intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, types.NamespacedName{Name: clusterName, Namespace: backup.Namespace}, cluster)
			if err != nil {
				return backupPolicy, false, err
			}
		}

		schedulePolicy := backupPolicy.Spec.GetCommonSchedulePolicy(backup.Spec.BackupType)
		if schedulePolicy.Enable && targetClusterExists {
			return backupPolicy, false, nil
		}
	}
	patch := client.MergeFrom(backup.DeepCopy())
	backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
	backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	if !backup.Status.StartTimestamp.IsZero() {
		// round the duration to a multiple of seconds.
		duration := backup.Status.CompletionTimestamp.Sub(backup.Status.StartTimestamp.Time).Round(time.Second)
		backup.Status.Duration = &metav1.Duration{Duration: duration}
	}
	return backupPolicy, true, r.Client.Status().Patch(reqCtx.Ctx, backup, patch)
}

func (r *BackupReconciler) createBackupStatefulSet(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	stsSpec *appsv1.StatefulSetSpec) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
			Labels:    buildBackupWorkloadsLabels(backup),
		},
		Spec: *stsSpec,
	}
	controllerutil.AddFinalizer(sts, dataProtectionFinalizerName)
	if err := controllerutil.SetControllerReference(backup, sts, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(reqCtx.Ctx, sts)
}

func (r *BackupReconciler) buildManifestsUpdaterContainer(backup *dataprotectionv1alpha1.Backup,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	backupDestinationPath string) (corev1.Container, error) {
	container := corev1.Container{}
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("manifests_updater.cue"))
	if err != nil {
		return container, err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	optionsBytes, err := json.Marshal(map[string]string{
		"backupName":      backup.Name,
		"namespace":       backup.Namespace,
		"image":           viper.GetString(constant.KBToolsImage),
		"containerName":   manifestsUpdaterContainerName,
		"imagePullPolicy": viper.GetString(constant.KBImagePullPolicy),
	})
	if err != nil {
		return container, err
	}
	if err = cueValue.Fill("options", optionsBytes); err != nil {
		return container, err
	}
	containerBytes, err := cueValue.Lookup("container")
	if err != nil {
		return container, err
	}
	if err = json.Unmarshal(containerBytes, &container); err != nil {
		return container, err
	}
	container.VolumeMounts = []corev1.VolumeMount{
		{Name: fmt.Sprintf("backup-%s", backup.Status.PersistentVolumeClaimName), MountPath: backupPathBase},
	}
	container.Env = []corev1.EnvVar{
		{Name: constant.DPBackupInfoFile, Value: buildBackupInfoENV(backupDestinationPath)},
	}
	return container, nil
}

func (r *BackupReconciler) buildStatefulSpec(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) (*appsv1.StatefulSetSpec, error) {
	backupDestinationPath := getBackupDestinationPath(backup, backupPolicy.Annotations[constant.BackupDataPathPrefixAnnotationKey])
	toolPodSpec, err := r.buildBackupToolPodSpec(reqCtx, backup, backupPolicy, commonPolicy, backupDestinationPath)
	toolPodSpec.RestartPolicy = corev1.RestartPolicyAlways
	if err != nil {
		return nil, err
	}
	// build the manifests updater container for backup.status.manifests
	manifestsUpdaterContainer, err := r.buildManifestsUpdaterContainer(backup, commonPolicy, backupDestinationPath)
	if err != nil {
		return nil, err
	}
	// build ARCHIVE_INTERVAL env
	schedulePolicy := backupPolicy.Spec.GetCommonSchedulePolicy(backup.Spec.BackupType)
	interval := getIntervalSecondsForLogfile(backup.Spec.BackupType, schedulePolicy.CronExpression)
	if interval != "" {
		toolPodSpec.Containers[0].Env = append(toolPodSpec.Containers[0].Env, corev1.EnvVar{
			Name:  constant.DPArchiveInterval,
			Value: interval,
		})
	}
	target, _ := r.getTargetPod(reqCtx, backup, commonPolicy.Target.LabelsSelector.MatchLabels)
	if target != nil && target.Spec.ServiceAccountName != "" {
		toolPodSpec.Containers = append(toolPodSpec.Containers, manifestsUpdaterContainer)
		toolPodSpec.ServiceAccountName = target.Spec.ServiceAccountName
	}
	backupLabels := buildBackupWorkloadsLabels(backup)
	defaultReplicas := int32(1)
	return &appsv1.StatefulSetSpec{
		Replicas: &defaultReplicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: backupLabels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: backupLabels,
			},
			Spec: toolPodSpec,
		},
	}, nil
}

func (r *BackupReconciler) doCompletedPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (ctrl.Result, error) {

	if err := r.deleteReferenceBatchV1Jobs(reqCtx, backup); err != nil && !apierrors.IsNotFound(err) {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := r.deleteReferenceStatefulSet(reqCtx, backup); err != nil && !apierrors.IsNotFound(err) {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupReconciler) updateStatusIfFailed(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup, err error) (ctrl.Result, error) {
	patch := client.MergeFrom(backup.DeepCopy())
	sendWarningEventForError(r.Recorder, backup, err)
	backup.Status.Phase = dataprotectionv1alpha1.BackupFailed
	backup.Status.FailureReason = err.Error()
	if errUpdate := r.Client.Status().Patch(reqCtx.Ctx, backup, patch); errUpdate != nil {
		return intctrlutil.CheckedRequeueWithError(errUpdate, reqCtx.Log, "")
	}
	return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
}

// getCluster gets the cluster and will ignore the error.
func (r *BackupReconciler) getCluster(
	reqCtx intctrlutil.RequestCtx,
	targetPod *corev1.Pod) *appsv1alpha1.Cluster {
	clusterName := targetPod.Labels[constant.AppInstanceLabelKey]
	if len(clusterName) == 0 {
		return nil
	}
	cluster := &appsv1alpha1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: targetPod.Namespace,
		Name:      clusterName,
	}, cluster); err != nil {
		// should not affect the backup status
		return nil
	}
	return cluster
}

// patchBackupObjectLabels add missed labels to the backup object.
func (r *BackupReconciler) patchBackupObjectLabels(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	labels map[string]string) (bool, error) {
	oldBackup := backup.DeepCopy()
	if backup.Labels == nil {
		backup.Labels = make(map[string]string)
	}
	for k, v := range labels {
		backup.Labels[k] = v
	}
	if reflect.DeepEqual(oldBackup.ObjectMeta, backup.ObjectMeta) {
		return false, nil
	}
	return true, r.Client.Patch(reqCtx.Ctx, backup, client.MergeFrom(oldBackup))
}

// patchBackupObjectMeta patches backup object metaObject include cluster snapshot.
func (r *BackupReconciler) patchBackupObjectMeta(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	targetPod *corev1.Pod,
	cluster *appsv1alpha1.Cluster,
	updateLabels map[string]string) (bool, error) {
	if backup.Labels == nil {
		backup.Labels = make(map[string]string)
	}
	oldBackup := backup.DeepCopy()
	if cluster != nil {
		if err := r.setClusterSnapshotAnnotation(backup, cluster); err != nil {
			return false, err
		}
		backup.Labels[constant.DataProtectionLabelClusterUIDKey] = string(cluster.UID)
	}
	for _, v := range getClusterLabelKeys() {
		backup.Labels[v] = targetPod.Labels[v]
	}
	backup.Labels[constant.AppManagedByLabelKey] = constant.AppName
	backup.Labels[dataProtectionLabelBackupTypeKey] = string(backup.Spec.BackupType)
	for k, v := range updateLabels {
		backup.Labels[k] = v
	}
	if backup.Annotations == nil {
		backup.Annotations = make(map[string]string)
	}
	backup.Annotations[dataProtectionBackupTargetPodKey] = targetPod.Name
	controllerutil.AddFinalizer(backup, dataProtectionFinalizerName)
	if reflect.DeepEqual(oldBackup.ObjectMeta, backup.ObjectMeta) {
		return false, nil
	}
	return true, r.Client.Patch(reqCtx.Ctx, backup, client.MergeFrom(oldBackup))
}

func (r *BackupReconciler) createPreCommandJobAndEnsure(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	snapshotPolicy *dataprotectionv1alpha1.SnapshotPolicy) (bool, error) {

	emptyCmd, err := r.ensureEmptyHooksCommand(snapshotPolicy, true)
	if err != nil {
		return false, err
	}
	// if undefined commands, skip create job.
	if emptyCmd {
		return true, err
	}

	mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	key := types.NamespacedName{Namespace: mgrNS, Name: generateUniqueJobName(backup, "hook-pre")}
	if err := r.createHooksCommandJob(reqCtx, backup, snapshotPolicy, key, true); err != nil {
		return false, err
	}
	return r.ensureBatchV1JobCompleted(reqCtx, key)
}

func (r *BackupReconciler) createPostCommandJobAndEnsure(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	snapshotPolicy *dataprotectionv1alpha1.SnapshotPolicy) (bool, error) {

	emptyCmd, err := r.ensureEmptyHooksCommand(snapshotPolicy, false)
	if err != nil {
		return false, err
	}
	// if undefined commands, skip create job.
	if emptyCmd {
		return true, err
	}

	mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	key := types.NamespacedName{Namespace: mgrNS, Name: generateUniqueJobName(backup, "hook-post")}
	if err = r.createHooksCommandJob(reqCtx, backup, snapshotPolicy, key, false); err != nil {
		return false, err
	}
	return r.ensureBatchV1JobCompleted(reqCtx, key)
}

func (r *BackupReconciler) ensureBatchV1JobCompleted(
	reqCtx intctrlutil.RequestCtx, key types.NamespacedName) (bool, error) {
	job := &batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, job)
	if err != nil {
		return false, err
	}
	if exists {
		if containsJobCondition(job, batchv1.JobComplete) {
			return true, nil
		}
		if containsJobCondition(job, batchv1.JobFailed) {
			return false, intctrlutil.NewBackupJobFailed(job.Name)
		}
	}
	return false, nil
}

func (r *BackupReconciler) createVolumeSnapshot(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	snapshotPolicy *dataprotectionv1alpha1.SnapshotPolicy) error {

	snap := &snapshotv1.VolumeSnapshot{}
	exists, err := r.snapshotCli.CheckResourceExists(reqCtx.Req.NamespacedName, snap)
	if err != nil {
		return err
	}
	if exists {
		// find resource object, skip created.
		return nil
	}

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Spec.BackupPolicyName,
	}

	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		reqCtx.Log.Error(err, "Unable to get backupPolicy for backup.", "backupPolicy", backupPolicyNameSpaceName)
		return err
	}

	targetPVCs, err := r.getTargetPVCs(reqCtx, backup, snapshotPolicy.Target.LabelsSelector.MatchLabels)
	if err != nil {
		return err
	}
	for _, target := range targetPVCs {
		snapshotName := backup.Name
		vsc := snapshotv1.VolumeSnapshotClass{}
		if target.Spec.StorageClassName != nil {
			if err = r.getVolumeSnapshotClassOrCreate(reqCtx.Ctx, *target.Spec.StorageClassName, &vsc); err != nil {
				return err
			}
		}
		labels := buildBackupWorkloadsLabels(backup)
		labels[constant.VolumeTypeLabelKey] = target.Labels[constant.VolumeTypeLabelKey]
		if target.Labels[constant.VolumeTypeLabelKey] == string(appsv1alpha1.VolumeTypeLog) {
			snapshotName += "-log"
		}
		snap = &snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: reqCtx.Req.Namespace,
				Name:      snapshotName,
				Labels:    labels,
			},
			Spec: snapshotv1.VolumeSnapshotSpec{
				Source: snapshotv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &target.Name,
				},
				VolumeSnapshotClassName: &vsc.Name,
			},
		}

		controllerutil.AddFinalizer(snap, dataProtectionFinalizerName)
		if err = controllerutil.SetControllerReference(backup, snap, r.Scheme); err != nil {
			return err
		}

		reqCtx.Log.V(1).Info("create a volumeSnapshot from backup", "snapshot", snap.Name)
		if err = r.snapshotCli.Create(snap); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	msg := fmt.Sprintf("Waiting for the volume snapshot %s creation to complete in backup.", snap.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingVolumeSnapshot", msg)
	return nil
}

func (r *BackupReconciler) getVolumeSnapshotClassOrCreate(ctx context.Context, storageClassName string, vsc *snapshotv1.VolumeSnapshotClass) error {
	storageClassObj := storagev1.StorageClass{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: storageClassName}, &storageClassObj); err != nil {
		// ignore if not found storage class, use the default volume snapshot class
		return client.IgnoreNotFound(err)
	}
	vscList := snapshotv1.VolumeSnapshotClassList{}
	if err := r.snapshotCli.List(&vscList); err != nil {
		return err
	}
	for _, item := range vscList.Items {
		if item.Driver == storageClassObj.Provisioner {
			*vsc = item
			return nil
		}
	}
	// not found matched volume snapshot class, create one
	vscName := fmt.Sprintf("vsc-%s-%s", storageClassName, storageClassObj.UID[:8])
	newVSC, err := ctrlbuilder.BuildVolumeSnapshotClass(vscName, storageClassObj.Provisioner)
	if err != nil {
		return err
	}
	if err = r.snapshotCli.Create(newVSC); err != nil {
		return err
	}
	*vsc = *newVSC
	return nil
}

func (r *BackupReconciler) ensureVolumeSnapshotReady(
	key types.NamespacedName) (bool, error) {
	snap := &snapshotv1.VolumeSnapshot{}
	// not found, continue the creation process
	exists, err := r.snapshotCli.CheckResourceExists(key, snap)
	if err != nil {
		return false, err
	}
	ready := false
	if exists && snap.Status != nil {
		// check if snapshot status throws an error, e.g. csi does not support volume snapshot
		if isVolumeSnapshotConfigError(snap) {
			return false, errors.New(*snap.Status.Error.Message)
		}
		if snap.Status.ReadyToUse != nil {
			ready = *(snap.Status.ReadyToUse)
		}
	}

	return ready, nil
}

func (r *BackupReconciler) createUpdatesJobs(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	basePolicy *dataprotectionv1alpha1.BasePolicy,
	backupDestinationPath string,
	stage dataprotectionv1alpha1.BackupStatusUpdateStage) error {
	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Spec.BackupPolicyName,
	}
	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		reqCtx.Log.V(1).Error(err, "Unable to get backupPolicy for backup.", "backupPolicy", backupPolicyNameSpaceName)
		return err
	}
	for index, update := range basePolicy.BackupStatusUpdates {
		if update.UpdateStage != stage {
			continue
		}
		if err := r.createMetadataCollectionJob(reqCtx, backup, commonPolicy, basePolicy, backupDestinationPath, update, index); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) createMetadataCollectionJob(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	basePolicy *dataprotectionv1alpha1.BasePolicy,
	backupDestinationPath string,
	updateInfo dataprotectionv1alpha1.BackupStatusUpdate,
	index int) error {
	jobNamespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	// if specified to use the service account of target pod, the namespace should be the namespace of backup.
	if updateInfo.UseTargetPodServiceAccount {
		jobNamespace = backup.Namespace
	}
	key := types.NamespacedName{Namespace: jobNamespace, Name: generateUniqueJobName(backup, fmt.Sprintf("status-%d-%s", index, string(updateInfo.UpdateStage)))}
	job := &batchv1.Job{}
	// check if job is created
	if exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, job); err != nil {
		return err
	} else if exists {
		return nil
	}

	// build job and create
	jobPodSpec, err := r.buildMetadataCollectionPodSpec(reqCtx, backup, commonPolicy, basePolicy, backupDestinationPath, updateInfo)
	if err != nil {
		return err
	}
	if job, err = ctrlbuilder.BuildBackupManifestsJob(key, backup, &jobPodSpec); err != nil {
		return err
	}
	msg := fmt.Sprintf("creating job %s", key.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingJob-"+key.Name, msg)
	return client.IgnoreAlreadyExists(r.Client.Create(reqCtx.Ctx, job))
}

func (r *BackupReconciler) createDeleteBackupFileJob(
	reqCtx intctrlutil.RequestCtx,
	jobKey types.NamespacedName,
	backup *dataprotectionv1alpha1.Backup,
	backupPVCName string,
	backupFilePath string) error {

	// make sure the path has a leading slash
	if !strings.HasPrefix(backupFilePath, "/") {
		backupFilePath = "/" + backupFilePath
	}

	// this script first deletes the directory where the backup is located (including files
	// in the directory), and then traverses up the path level by level to clean up empty directories.
	deleteScript := fmt.Sprintf(`
		backupPathBase=%s;
		targetPath="${backupPathBase}%s";

		echo "removing backup files in ${targetPath}";
		rm -rf "${targetPath}";

		absBackupPathBase=$(realpath "${backupPathBase}");
		curr=$(realpath "${targetPath}");
		while true; do
			parent=$(dirname "${curr}");
			if [ "${parent}" == "${absBackupPathBase}" ]; then
				echo "reach backupPathBase ${backupPathBase}, done";
				break;
			fi;
			if [ ! "$(ls -A "${parent}")" ]; then
				echo "${parent} is empty, removing it...";
				rmdir "${parent}";
			else
				echo "${parent} is not empty, done";
				break;
			fi;
			curr="${parent}";
		done
	`, backupPathBase, backupFilePath)

	// build container
	container := corev1.Container{}
	container.Name = backup.Name
	container.Command = []string{"sh", "-c"}
	container.Args = []string{deleteScript}
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))

	allowPrivilegeEscalation := false
	runAsUser := int64(0)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                &runAsUser,
	}

	// build pod
	podSpec := corev1.PodSpec{
		Containers:    []corev1.Container{container},
		RestartPolicy: corev1.RestartPolicyNever,
	}

	// mount the backup volume to the pod
	r.appendBackupVolumeMount(backupPVCName, &podSpec, &podSpec.Containers[0])

	if err := addTolerations(&podSpec); err != nil {
		return err
	}

	// build job
	backOffLimit := int32(3)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: jobKey.Namespace,
			Name:      jobKey.Name,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: jobKey.Namespace,
					Name:      jobKey.Name,
				},
				Spec: podSpec,
			},
			BackoffLimit: &backOffLimit,
		},
	}
	if err := controllerutil.SetControllerReference(backup, job, r.Scheme); err != nil {
		return err
	}
	reqCtx.Log.V(1).Info("create a job from delete backup files", "job", job)
	return client.IgnoreAlreadyExists(r.Client.Create(reqCtx.Ctx, job))
}

func (r *BackupReconciler) createBackupToolJob(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	backupDestinationPath string) error {

	key := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}
	job := batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, &job)
	if err != nil {
		return err
	}
	if exists {
		// find resource object, skip created.
		return nil
	}

	toolPodSpec, err := r.buildBackupToolPodSpec(reqCtx, backup, backupPolicy, commonPolicy, backupDestinationPath)
	if err != nil {
		return err
	}

	if err = r.createBatchV1Job(reqCtx, key, backup, toolPodSpec); err != nil {
		return err
	}
	msg := fmt.Sprintf("Waiting for the job %s creation to complete.", key.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingJob", msg)
	return nil
}

// ensureEmptyHooksCommand determines whether it has empty commands in the hooks
func (r *BackupReconciler) ensureEmptyHooksCommand(
	snapshotPolicy *dataprotectionv1alpha1.SnapshotPolicy,
	preCommand bool) (bool, error) {
	// return true directly, means hooks commands is empty, skip subsequent hook jobs.
	if snapshotPolicy.Hooks == nil {
		return true, nil
	}

	commands := snapshotPolicy.Hooks.PostCommands
	if preCommand {
		commands = snapshotPolicy.Hooks.PreCommands
	}
	if len(commands) == 0 {
		return true, nil
	}
	return false, nil
}

func (r *BackupReconciler) createHooksCommandJob(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	snapshotPolicy *dataprotectionv1alpha1.SnapshotPolicy,
	key types.NamespacedName,
	preCommand bool) error {

	job := batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, &job)
	if err != nil {
		return err
	}
	if exists {
		// find resource object, skip created.
		return nil
	}

	jobPodSpec, err := r.buildSnapshotPodSpec(reqCtx, backup, snapshotPolicy, preCommand)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Waiting for the job %s creation to complete.", key.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingJob-"+key.Name, msg)

	return r.createBatchV1Job(reqCtx, key, backup, jobPodSpec)
}

func (r *BackupReconciler) createBatchV1Job(
	reqCtx intctrlutil.RequestCtx,
	key types.NamespacedName,
	backup *dataprotectionv1alpha1.Backup,
	templatePodSpec corev1.PodSpec) error {

	backOffLimit := int32(3)
	job := &batchv1.Job{
		// TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    buildBackupWorkloadsLabels(backup),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name},
				Spec: templatePodSpec,
			},
			BackoffLimit: &backOffLimit,
		},
	}
	controllerutil.AddFinalizer(job, dataProtectionFinalizerName)
	if backup.Namespace == job.Namespace {
		if err := controllerutil.SetControllerReference(backup, job, r.Scheme); err != nil {
			return err
		}
	}

	reqCtx.Log.V(1).Info("create a built-in job from backup", "job", job)
	return client.IgnoreAlreadyExists(r.Client.Create(reqCtx.Ctx, job))
}

func (r *BackupReconciler) deleteReferenceBatchV1Jobs(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
	jobs := &batchv1.JobList{}
	namespace := backup.Namespace
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		namespace = viper.GetString(constant.CfgKeyCtrlrMgrNS)
	}
	if err := r.Client.List(reqCtx.Ctx, jobs,
		client.InNamespace(namespace),
		client.MatchingLabels(buildBackupWorkloadsLabels(backup))); err != nil {
		return err
	}

	for _, job := range jobs.Items {
		if controllerutil.ContainsFinalizer(&job, dataProtectionFinalizerName) {
			patch := client.MergeFrom(job.DeepCopy())
			controllerutil.RemoveFinalizer(&job, dataProtectionFinalizerName)
			if err := r.Patch(reqCtx.Ctx, &job, patch); err != nil {
				return err
			}
		}

		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &job); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) deleteReferenceVolumeSnapshot(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
	snaps := &snapshotv1.VolumeSnapshotList{}

	if err := r.snapshotCli.List(snaps,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(buildBackupWorkloadsLabels(backup))); err != nil {
		return err
	}
	for _, i := range snaps.Items {
		if controllerutil.ContainsFinalizer(&i, dataProtectionFinalizerName) {
			patch := i.DeepCopy()
			controllerutil.RemoveFinalizer(&i, dataProtectionFinalizerName)
			if err := r.snapshotCli.Patch(&i, patch); err != nil {
				return err
			}
		}
		if err := r.snapshotCli.Delete(&i); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) handleDeleteBackupFiles(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) (*batchv1.Job, error) {
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		// no file to delete for this type
		return nil, nil
	}
	if backup.Status.Phase == dataprotectionv1alpha1.BackupNew {
		// nothing to delete
		return nil, nil
	}
	jobKey := buildDeleteBackupFilesJobNamespacedName(backup)
	job := &batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, jobKey, job)
	if err != nil {
		return nil, err
	}
	// create job for deleting backup files
	if !exists {
		pvcName := backup.Status.PersistentVolumeClaimName
		if pvcName == "" {
			reqCtx.Log.Info("skip deleting backup files because PersistentVolumeClaimName is empty",
				"backup", backup.Name)
			return nil, nil
		}
		// check if pvc exists
		if err = r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backup.Namespace, Name: pvcName}, &corev1.PersistentVolumeClaim{}); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}

		backupFilePath := ""
		if backup.Status.Manifests != nil && backup.Status.Manifests.BackupTool != nil {
			backupFilePath = backup.Status.Manifests.BackupTool.FilePath
		}
		if backupFilePath == "" || !strings.Contains(backupFilePath, backup.Name) {
			// For compatibility: the FilePath field is changing from time to time,
			// and it may not contain the backup name as a path component if the Backup object
			// was created in a previous version. In this case, it's dangerous to execute
			// the deletion command. For example, files belongs to other Backups can be deleted as well.
			reqCtx.Log.Info("skip deleting backup files because backupFilePath is invalid",
				"backupFilePath", backupFilePath, "backup", backup.Name)
			return nil, nil
		}
		// the job will run in the background
		return job, r.createDeleteBackupFileJob(reqCtx, jobKey, backup, pvcName, backupFilePath)
	}
	return job, nil
}

// deleteReferenceStatefulSet deletes the referenced statefulSet.
func (r *BackupReconciler) deleteReferenceStatefulSet(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
	sts := &appsv1.StatefulSet{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, types.NamespacedName{
		Namespace: backup.Namespace,
		Name:      backup.Name,
	}, sts)
	if err != nil {
		return err
	}
	if !exists && !model.IsOwnerOf(backup, sts) {
		return nil
	}
	patch := client.MergeFrom(sts.DeepCopy())
	controllerutil.RemoveFinalizer(sts, dataProtectionFinalizerName)
	if err = r.Client.Patch(reqCtx.Ctx, sts, patch); err != nil {
		return err
	}
	return intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, sts)
}

func (r *BackupReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
	if err := r.deleteReferenceStatefulSet(reqCtx, backup); err != nil {
		return err
	}
	if err := r.deleteReferenceBatchV1Jobs(reqCtx, backup); err != nil {
		return err
	}
	if err := r.deleteReferenceVolumeSnapshot(reqCtx, backup); err != nil {
		return err
	}
	return nil
}

// getTargetPod gets the target pod by label selector.
// if the backup has obtained the target pod from label selector, it will be set to the annotations.
// then get the pod from this annotation to ensure that the same pod is picked up in future.
func (r *BackupReconciler) getTargetPod(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup, labels map[string]string) (*corev1.Pod, error) {
	reqCtx.Log.V(1).Info("Get pod from label", "label", labels)
	targetPod := &corev1.PodList{}
	if err := r.Client.List(reqCtx.Ctx, targetPod,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	if len(targetPod.Items) == 0 {
		return nil, errors.New("can not find any pod to backup by labelsSelector")
	}
	sort.Sort(intctrlutil.ByPodName(targetPod.Items))
	targetPodName := backup.Annotations[dataProtectionBackupTargetPodKey]
	for _, v := range targetPod.Items {
		if targetPodName == v.Name {
			return &v, nil
		}
	}
	return &targetPod.Items[0], nil
}

func (r *BackupReconciler) getTargetPVCs(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup, podLabels map[string]string) ([]corev1.PersistentVolumeClaim, error) {
	targetPod, err := r.getTargetPod(reqCtx, backup, podLabels)
	if err != nil {
		return nil, err
	}
	tempPVC := corev1.PersistentVolumeClaim{}
	var dataPVC *corev1.PersistentVolumeClaim
	var logPVC *corev1.PersistentVolumeClaim
	for _, volume := range targetPod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		pvcKey := types.NamespacedName{Namespace: backup.Namespace, Name: volume.PersistentVolumeClaim.ClaimName}
		if err = r.Client.Get(reqCtx.Ctx, pvcKey, &tempPVC); err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		switch tempPVC.Labels[constant.VolumeTypeLabelKey] {
		case string(appsv1alpha1.VolumeTypeData):
			dataPVC = tempPVC.DeepCopy()
		case string(appsv1alpha1.VolumeTypeLog):
			logPVC = tempPVC.DeepCopy()
		}
	}

	if dataPVC == nil {
		return nil, errors.New("can not find any pvc to backup with labelsSelector")
	}

	allPVCs := []corev1.PersistentVolumeClaim{*dataPVC}
	if logPVC != nil {
		allPVCs = append(allPVCs, *logPVC)
	}

	return allPVCs, nil
}

func (r *BackupReconciler) appendBackupVolumeMount(
	pvcName string,
	podSpec *corev1.PodSpec,
	container *corev1.Container) {
	// TODO(dsj): mount multi remote backup volumes
	remoteVolumeName := fmt.Sprintf("backup-%s", pvcName)
	remoteVolume := corev1.Volume{
		Name: remoteVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	remoteVolumeMount := corev1.VolumeMount{
		Name:      remoteVolumeName,
		MountPath: backupPathBase,
	}
	podSpec.Volumes = append(podSpec.Volumes, remoteVolume)
	container.VolumeMounts = append(container.VolumeMounts, remoteVolumeMount)
}

func (r *BackupReconciler) buildBackupToolPodSpec(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	pathPrefix string) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	// get backup tool
	backupTool, err := getBackupToolByName(reqCtx, r.Client, commonPolicy.BackupToolName)
	if err != nil {
		return podSpec, err
	}
	// TODO: check if pvc exists
	clusterPod, err := r.getTargetPod(reqCtx, backup, commonPolicy.Target.LabelsSelector.MatchLabels)
	if err != nil {
		return podSpec, err
	}

	// build pod dns string
	envDBHost := corev1.EnvVar{
		Name:  constant.DPDBHost,
		Value: intctrlutil.BuildPodHostDNS(clusterPod),
	}

	container := corev1.Container{}
	container.Name = backup.Name
	container.Command = backupTool.Spec.BackupCommands
	container.Image = backupTool.Spec.Image
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	if container.Image == "" {
		// TODO(dsj): need determine container name to get, temporary use first container
		container.Image = clusterPod.Spec.Containers[0].Image
	}
	if backupTool.Spec.Resources != nil {
		container.Resources = *backupTool.Spec.Resources
	}
	container.VolumeMounts = clusterPod.Spec.Containers[0].VolumeMounts

	allowPrivilegeEscalation := false
	runAsUser := int64(0)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                &runAsUser}

	envBackupName := corev1.EnvVar{
		Name:  constant.DPBackupName,
		Value: backup.Name,
	}

	envBackupDir := corev1.EnvVar{
		Name:  constant.DPBackupDIR,
		Value: backupPathBase + pathPrefix,
	}

	container.Env = []corev1.EnvVar{envDBHost, envBackupName, envBackupDir}
	if commonPolicy.Target.Secret != nil {
		envDBUser := corev1.EnvVar{
			Name: constant.DPDBUser,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: commonPolicy.Target.Secret.Name,
					},
					Key: commonPolicy.Target.Secret.UsernameKey,
				},
			},
		}

		envDBPassword := corev1.EnvVar{
			Name: constant.DPDBPassword,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: commonPolicy.Target.Secret.Name,
					},
					Key: commonPolicy.Target.Secret.PasswordKey,
				},
			},
		}

		container.Env = append(container.Env, envDBUser, envDBPassword)
	}

	if backupPolicy.Spec.Retention != nil && backupPolicy.Spec.Retention.TTL != nil {
		ttl := backupPolicy.Spec.Retention.TTL
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  constant.DPTTL,
			Value: *ttl,
		})
		// one more day than the configured TTL for logfile backup
		logTTL := dataprotectionv1alpha1.AddTTL(ttl, 24)
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  constant.DPLogfileTTL,
			Value: logTTL,
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  constant.DPLogfileTTLSecond,
			Value: strconv.FormatInt(int64(math.Floor(dataprotectionv1alpha1.ToDuration(&logTTL).Seconds())), 10),
		})
	}

	// merge env from backup tool.
	container.Env = append(container.Env, backupTool.Spec.Env...)

	podSpec.Containers = []corev1.Container{container}
	podSpec.Volumes = clusterPod.Spec.Volumes
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	// mount the backup volume to the pod of backup tool
	pvcName := backup.Status.PersistentVolumeClaimName
	r.appendBackupVolumeMount(pvcName, &podSpec, &podSpec.Containers[0])

	// the pod of job needs to be scheduled on the same node as the workload pod, because it needs to share one pvc
	if clusterPod.Spec.NodeName != "" {
		podSpec.NodeSelector = map[string]string{
			hostNameLabelKey: clusterPod.Spec.NodeName,
		}
	}
	// ignore taints
	podSpec.Tolerations = []corev1.Toleration{
		{
			Operator: corev1.TolerationOpExists,
		},
	}
	return podSpec, nil
}

func (r *BackupReconciler) buildSnapshotPodSpec(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	snapshotPolicy *dataprotectionv1alpha1.SnapshotPolicy,
	preCommand bool) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}

	clusterPod, err := r.getTargetPod(reqCtx, backup, snapshotPolicy.Target.LabelsSelector.MatchLabels)
	if err != nil {
		return podSpec, err
	}

	container := corev1.Container{}
	container.Name = backup.Name
	container.Command = []string{"kubectl", "exec", "-n", backup.Namespace,
		"-i", clusterPod.Name, "-c", snapshotPolicy.Hooks.ContainerName, "--", "sh", "-c"}
	if preCommand {
		container.Args = snapshotPolicy.Hooks.PreCommands
	} else {
		container.Args = snapshotPolicy.Hooks.PostCommands
	}
	container.Image = snapshotPolicy.Hooks.Image
	if container.Image == "" {
		container.Image = viper.GetString(constant.KBToolsImage)
		container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	}
	allowPrivilegeEscalation := false
	runAsUser := int64(0)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                &runAsUser}

	podSpec.Containers = []corev1.Container{container}
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.ServiceAccountName = viper.GetString("KUBEBLOCKS_SERVICEACCOUNT_NAME")

	if err = addTolerations(&podSpec); err != nil {
		return podSpec, err
	}

	return podSpec, nil
}

func (r *BackupReconciler) buildMetadataCollectionPodSpec(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	basePolicy *dataprotectionv1alpha1.BasePolicy,
	backupDestinationPath string,
	updateInfo dataprotectionv1alpha1.BackupStatusUpdate) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	targetPod, err := r.getTargetPod(reqCtx, backup, basePolicy.Target.LabelsSelector.MatchLabels)
	if err != nil {
		return podSpec, err
	}

	container := corev1.Container{}
	container.Name = backup.Name
	container.Command = []string{"sh", "-c"}
	var args string
	if strings.TrimSpace(updateInfo.Script) == "" && commonPolicy != nil {
		// if not specified script, patch backup status with the json string from ${BACKUP_DIR}/backup.info.
		args = "set -o errexit; set -o nounset;" +
			"backupInfo=$(cat ${BACKUP_INFO_FILE});echo \"backupInfo:${backupInfo}\";" +
			"eval kubectl -n %s patch backup %s --subresource=status --type=merge --patch '{\\\"status\\\":${backupInfo}}';"
		args = fmt.Sprintf(args, backup.Namespace, backup.Name)
		container.Env = []corev1.EnvVar{
			{Name: "BACKUP_INFO_FILE", Value: buildBackupInfoENV(backupDestinationPath)},
		}
		r.appendBackupVolumeMount(backup.Status.PersistentVolumeClaimName, &podSpec, &container)
	} else {
		args = "set -o errexit; set -o nounset;" +
			"OUTPUT=$(kubectl -n %s exec -it pod/%s -c %s -- %s);" +
			"kubectl -n %s patch backup %s --subresource=status --type=merge --patch \"%s\";"
		statusPath := "status." + updateInfo.Path
		if updateInfo.Path == "" {
			statusPath = "status"
		}
		patchJSON := generateJSON(statusPath, "$OUTPUT")
		args = fmt.Sprintf(args, targetPod.Namespace, targetPod.Name, updateInfo.ContainerName,
			updateInfo.Script, backup.Namespace, backup.Name, patchJSON)
	}
	if updateInfo.UseTargetPodServiceAccount {
		podSpec.ServiceAccountName = targetPod.Spec.ServiceAccountName
	} else {
		podSpec.ServiceAccountName = viper.GetString("KUBEBLOCKS_SERVICEACCOUNT_NAME")
	}
	container.Args = []string{args}
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	podSpec.Containers = []corev1.Container{container}
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	if err = addTolerations(&podSpec); err != nil {
		return podSpec, err
	}
	return podSpec, nil
}

// getClusterObjectString gets the cluster object and convert it to string.
func (r *BackupReconciler) getClusterObjectString(cluster *appsv1alpha1.Cluster) (*string, error) {
	// maintain only the cluster's spec and name/namespace.
	newCluster := &appsv1alpha1.Cluster{
		Spec: cluster.Spec,
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		},
		TypeMeta: cluster.TypeMeta,
	}
	clusterBytes, err := json.Marshal(newCluster)
	if err != nil {
		return nil, err
	}
	clusterString := string(clusterBytes)
	return &clusterString, nil
}

// setClusterSnapshotAnnotation sets the snapshot of cluster to the backup's annotations.
func (r *BackupReconciler) setClusterSnapshotAnnotation(backup *dataprotectionv1alpha1.Backup, cluster *appsv1alpha1.Cluster) error {
	clusterString, err := r.getClusterObjectString(cluster)
	if err != nil {
		return err
	}
	if clusterString == nil {
		return nil
	}
	if backup.Annotations == nil {
		backup.Annotations = map[string]string{}
	}
	backup.Annotations[constant.ClusterSnapshotAnnotationKey] = *clusterString
	return nil
}
