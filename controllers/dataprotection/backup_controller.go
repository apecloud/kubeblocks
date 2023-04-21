/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	"reflect"
	"sort"
	"strings"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	ctrlbuilder "github.com/apecloud/kubeblocks/internal/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
	clock    clock.RealClock
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/finalizers,verbs=update

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots/finalizers,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// NOTES:
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backup", req.NamespacedName),
		Recorder: r.Recorder,
	}
	// Get backup obj
	backup := &dataprotectionv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backup); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	reqCtx.Log.V(1).Info("in Backup Reconciler:", "backup", backup.Name, "phase", backup.Status.Phase)

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backup, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backup)
	})
	if res != nil {
		return *res, err
	}

	// backup reconcile logic here
	switch backup.Status.Phase {
	case "", dataprotectionv1alpha1.BackupNew:
		return r.doNewPhaseAction(reqCtx, backup)
	case dataprotectionv1alpha1.BackupInProgress:
		return r.doInProgressPhaseAction(reqCtx, backup)
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
		Owns(&batchv1.Job{})

	if viper.GetBool("VOLUMESNAPSHOT") {
		b.Owns(&snapshotv1.VolumeSnapshot{}, builder.OnlyMetadata, builder.Predicates{})
	}

	return b.Complete(r)
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
		return nil, fmt.Errorf("backup policy %s not found", backupPolicyNameSpaceName)
	}

	// validate backup spec
	if err := backup.Spec.Validate(backupPolicy); err != nil {
		return nil, err
	}
	return backupPolicy, nil
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

	// TODO: get pod with matching labels to do backup.
	var targetCluster dataprotectionv1alpha1.TargetCluster
	switch backup.Spec.BackupType {
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		targetCluster = backupPolicy.Spec.Snapshot.Target
	default:
		commonPolicy := backupPolicy.Spec.GetCommonPolicy(backup.Spec.BackupType)
		if commonPolicy == nil {
			return r.updateStatusIfFailed(reqCtx, backup, intctrlutil.NewNotFound(`backup type "%s" not supported in the backupPolicy "%s"`, backup.Spec.BackupType, backupPolicy.Name))
		}
		// save the backup message for restore
		backup.Status.PersistentVolumeClaimName = commonPolicy.PersistentVolumeClaim.Name
		backup.Status.BackupToolName = commonPolicy.BackupToolName
		targetCluster = commonPolicy.Target
		if err = r.handlePersistentVolumeClaim(reqCtx, backupPolicy.Name, commonPolicy); err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
	}

	target, err := r.getTargetPod(reqCtx, backup, targetCluster.LabelsSelector.MatchLabels)
	if err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	}

	if hasPatch, err := r.patchBackupLabelsAndAnnotations(reqCtx, backup, target); err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	} else if hasPatch {
		return intctrlutil.Reconciled()
	}

	// update Phase to InProgress
	backup.Status.Phase = dataprotectionv1alpha1.BackupInProgress
	backup.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	if backupPolicy.Spec.TTL != nil {
		backup.Status.Expiration = &metav1.Time{
			Time: backup.Status.StartTimestamp.Add(dataprotectionv1alpha1.ToDuration(backupPolicy.Spec.TTL)),
		}
	}

	if err = r.Client.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

// handlePersistentVolumeClaim handles the persistent volume claim for the backup, the rules are as follows
// - if CreatePolicy is "Never", it will check if the pvc exists. if not exist, will report an error.
// - if CreatePolicy is "IfNotPresent" and the pvc not exists, will create the pvc automatically.
func (r *BackupReconciler) handlePersistentVolumeClaim(reqCtx intctrlutil.RequestCtx,
	backupPolicyName string,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) error {
	pvcConfig := commonPolicy.PersistentVolumeClaim
	if len(pvcConfig.Name) == 0 {
		return fmt.Errorf("the persistentVolumeClaim name of this policy is empty")
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Namespace: reqCtx.Req.Namespace,
		Name: pvcConfig.Name}, pvc); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if len(pvc.Name) > 0 {
		return nil
	}
	if pvcConfig.CreatePolicy == dataprotectionv1alpha1.CreatePVCPolicyNever {
		return intctrlutil.NewNotFound(`persistent volume claim "%s" not found`, pvcConfig.Name)
	}
	if pvcConfig.PersistentVolumeConfigMap != nil &&
		(pvcConfig.StorageClassName == nil || *pvcConfig.StorageClassName == "") {
		// if the storageClassName is empty and the PersistentVolumeConfigMap is not empty,
		// will create the persistentVolume with the template
		if err := r.createPersistentVolumeWithTemplate(reqCtx, backupPolicyName, &pvcConfig); err != nil {
			return err
		}
	}
	return r.createPVCWithStorageClassName(reqCtx, backupPolicyName, pvcConfig)
}

// createPVCWithStorageClassName creates the persistent volume claim with the storageClassName.
func (r *BackupReconciler) createPVCWithStorageClassName(reqCtx intctrlutil.RequestCtx,
	backupPolicyName string,
	pvcConfig dataprotectionv1alpha1.PersistentVolumeClaim) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcConfig.Name,
			Namespace:   reqCtx.Req.Namespace,
			Annotations: r.buildAutoCreationAnnotations(backupPolicyName),
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
	// add a finalizer
	controllerutil.AddFinalizer(pvc, dataProtectionFinalizerName)
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
		return intctrlutil.NewNotFound("the persistentVolume template is empty in the configMap %s/%s", pvConfig.Namespace, pvConfig.Name)
	}
	pvName := fmt.Sprintf("%s-%s", pvcConfig.Name, reqCtx.Req.Namespace)
	pvTemplate = strings.ReplaceAll(pvTemplate, "$(GENERATE_NAME)", pvName)
	pv := &corev1.PersistentVolume{}
	if err := yaml.Unmarshal([]byte(pvTemplate), pv); err != nil {
		return err
	}
	pv.Name = pvName
	pv.Spec.ClaimRef = &corev1.ObjectReference{
		Namespace: reqCtx.Req.Namespace,
		Name:      pvcConfig.Name,
	}
	pv.Annotations = r.buildAutoCreationAnnotations(backupPolicyName)
	// set the storageClassName to empty for the persistentVolumeClaim to avoid the dynamic provisioning
	emptyStorageClassName := ""
	pvcConfig.StorageClassName = &emptyStorageClassName
	controllerutil.AddFinalizer(pv, dataProtectionFinalizerName)
	return r.Client.Create(reqCtx.Ctx, pv)
}

func (r *BackupReconciler) buildAutoCreationAnnotations(backupPolicyName string) map[string]string {
	return map[string]string{
		dataProtectionAnnotationCreateByPolicyKey: "true",
		dataProtectionLabelBackupPolicyKey:        backupPolicyName,
	}
}

// getBackupPathPrefix gets the backup path prefix.
func (r *BackupReconciler) getBackupPathPrefix(req ctrl.Request, pathPrefix string) string {
	pathPrefix = strings.TrimRight(pathPrefix, "/")
	if strings.TrimSpace(pathPrefix) == "" || strings.HasPrefix(pathPrefix, "/") {
		return fmt.Sprintf("/%s%s/%s", req.Namespace, pathPrefix, req.Name)
	}
	return fmt.Sprintf("/%s/%s/%s", req.Namespace, pathPrefix, req.Name)
}

func (r *BackupReconciler) doInProgressPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (ctrl.Result, error) {
	backupPolicy, err := r.getBackupPolicyAndValidate(reqCtx, backup)
	if err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	}
	patch := client.MergeFrom(backup.DeepCopy())
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		// 1. create and ensure pre-command job completed
		// 2. create and ensure volume snapshot ready
		// 3. create and ensure post-command job completed
		isOK, err := r.createPreCommandJobAndEnsure(reqCtx, backup, backupPolicy.Spec.Snapshot)
		if err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		if err = r.createUpdatesJobs(reqCtx, backup, &backupPolicy.Spec.Snapshot.BasePolicy, dataprotectionv1alpha1.PRE); err != nil {
			r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPreUpdatesJob", err.Error())
		}
		if err = r.createVolumeSnapshot(reqCtx, backup, backupPolicy.Spec.Snapshot); err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}

		key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backup.Name}
		isOK, err = r.ensureVolumeSnapshotReady(reqCtx, key)
		if err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		msg := fmt.Sprintf("Created volumeSnapshot %s ready.", key.Name)
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedVolumeSnapshot", msg)

		isOK, err = r.createPostCommandJobAndEnsure(reqCtx, backup, backupPolicy.Spec.Snapshot)
		if err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}

		// Failure MetadataCollectionJob does not affect the backup status.
		if err = r.createUpdatesJobs(reqCtx, backup, &backupPolicy.Spec.Snapshot.BasePolicy, dataprotectionv1alpha1.POST); err != nil {
			r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPostUpdatesJob", err.Error())
		}

		backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
		backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
		snap := &snapshotv1.VolumeSnapshot{}
		exists, _ := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, snap)
		if exists {
			backup.Status.TotalSize = snap.Status.RestoreSize.String()
		}
	} else {
		// 1. create and ensure backup tool job finished
		// 2. get job phase and update
		commonPolicy := backupPolicy.Spec.GetCommonPolicy(backup.Spec.BackupType)
		if commonPolicy == nil {
			// TODO: add error type
			return r.updateStatusIfFailed(reqCtx, backup, fmt.Errorf("not found the %s policy", backup.Spec.BackupType))
		}
		// createUpdatesJobs should not affect the backup status, just need to record events when the run fails
		if err = r.createUpdatesJobs(reqCtx, backup, &commonPolicy.BasePolicy, dataprotectionv1alpha1.PRE); err != nil {
			r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPreUpdatesJob", err.Error())
		}
		pathPrefix := r.getBackupPathPrefix(reqCtx.Req, backupPolicy.Annotations[constant.BackupDataPathPrefixAnnotationKey])
		err = r.createBackupToolJob(reqCtx, backup, commonPolicy, pathPrefix)
		if err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
		key := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}
		isOK, err := r.ensureBatchV1JobCompleted(reqCtx, key)
		if err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		job, err := r.getBatchV1Job(reqCtx, backup)
		if err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
		// createUpdatesJobs should not affect the backup status, just need to record events when the run fails
		if err = r.createUpdatesJobs(reqCtx, backup, &commonPolicy.BasePolicy, dataprotectionv1alpha1.POST); err != nil {
			r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPostUpdatesJob", err.Error())
		}
		jobStatusConditions := job.Status.Conditions
		if jobStatusConditions[0].Type == batchv1.JobComplete {
			// update Phase to in Completed
			backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
			backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
			if backup.Status.Manifests == nil {
				backup.Status.Manifests = &dataprotectionv1alpha1.ManifestsStatus{}
			}
			if backup.Status.Manifests.BackupTool == nil {
				backup.Status.Manifests.BackupTool = &dataprotectionv1alpha1.BackupToolManifestsStatus{}
			}
			backup.Status.Manifests.BackupTool.FilePath = pathPrefix
		} else if jobStatusConditions[0].Type == batchv1.JobFailed {
			backup.Status.Phase = dataprotectionv1alpha1.BackupFailed
			backup.Status.FailureReason = job.Status.Conditions[0].Reason
		}
		if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeIncremental {
			if backup.Status.Manifests != nil &&
				backup.Status.Manifests.BackupLog != nil &&
				backup.Status.Manifests.BackupLog.StartTime == nil {
				backup.Status.Manifests.BackupLog.StartTime = backup.Status.Manifests.BackupLog.StopTime
			}
		}
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

func (r *BackupReconciler) doCompletedPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (ctrl.Result, error) {

	if err := r.deleteReferenceBatchV1Jobs(reqCtx, backup); err != nil && !apierrors.IsNotFound(err) {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupReconciler) updateStatusIfFailed(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup, err error) (ctrl.Result, error) {
	patch := client.MergeFrom(backup.DeepCopy())
	r.Recorder.Eventf(backup, corev1.EventTypeWarning, "FailedCreatedBackup",
		"Failed creating backup, error: %s", err.Error())
	backup.Status.Phase = dataprotectionv1alpha1.BackupFailed
	backup.Status.FailureReason = err.Error()
	if errUpdate := r.Client.Status().Patch(reqCtx.Ctx, backup, patch); errUpdate != nil {
		return intctrlutil.CheckedRequeueWithError(errUpdate, reqCtx.Log, "")
	}
	return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
}

// patchBackupLabelsAndAnnotations patches backup labels and the annotations include cluster snapshot.
func (r *BackupReconciler) patchBackupLabelsAndAnnotations(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	targetPod *corev1.Pod) (bool, error) {
	oldBackup := backup.DeepCopy()
	clusterName := targetPod.Labels[constant.AppInstanceLabelKey]
	if len(clusterName) > 0 {
		if err := r.setClusterSnapshotAnnotation(reqCtx, backup, types.NamespacedName{Name: clusterName, Namespace: backup.Namespace}); err != nil {
			return false, err
		}
	}
	if backup.Labels == nil {
		backup.Labels = make(map[string]string)
	}
	for k, v := range targetPod.Labels {
		backup.Labels[k] = v
	}
	backup.Labels[dataProtectionLabelBackupTypeKey] = string(backup.Spec.BackupType)
	if backup.Annotations == nil {
		backup.Annotations = make(map[string]string)
	}
	backup.Annotations[dataProtectionBackupTargetPodKey] = targetPod.Name
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
	// if not defined commands, skip create job.
	if emptyCmd {
		return true, err
	}

	mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	key := types.NamespacedName{Namespace: mgrNS, Name: backup.Name + "-pre"}
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
	// if not defined commands, skip create job.
	if emptyCmd {
		return true, err
	}

	mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	key := types.NamespacedName{Namespace: mgrNS, Name: backup.Name + "-post"}
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
		jobStatusConditions := job.Status.Conditions
		if len(jobStatusConditions) > 0 {
			if jobStatusConditions[0].Type == batchv1.JobComplete {
				return true, nil
			} else if jobStatusConditions[0].Type == batchv1.JobFailed {
				return false, errors.New(errorJobFailed)
			}
		}
	}
	return false, nil
}

func (r *BackupReconciler) createVolumeSnapshot(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	snapshotPolicy *dataprotectionv1alpha1.SnapshotPolicy) error {

	snap := &snapshotv1.VolumeSnapshot{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, reqCtx.Req.NamespacedName, snap)
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
		labels := buildBackupLabels(backup)
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
			},
		}

		controllerutil.AddFinalizer(snap, dataProtectionFinalizerName)

		scheme, _ := dataprotectionv1alpha1.SchemeBuilder.Build()
		if err = controllerutil.SetOwnerReference(backup, snap, scheme); err != nil {
			return err
		}

		reqCtx.Log.V(1).Info("create a volumeSnapshot from backup", "snapshot", snap.Name)
		if err = r.Client.Create(reqCtx.Ctx, snap); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	msg := fmt.Sprintf("Waiting for a volume snapshot %s to be created by the backup.", snap.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingVolumeSnapshot", msg)
	return nil
}

func (r *BackupReconciler) ensureVolumeSnapshotReady(reqCtx intctrlutil.RequestCtx,
	key types.NamespacedName) (bool, error) {

	snap := &snapshotv1.VolumeSnapshot{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, snap)
	if err != nil {
		return false, err
	}
	ready := false
	if exists && snap.Status != nil {
		// check if snapshot status throw error, e.g. csi does not support volume snapshot
		if snap.Status.Error != nil && snap.Status.Error.Message != nil {
			return ready, errors.New(*snap.Status.Error.Message)
		}
		if snap.Status.ReadyToUse != nil {
			ready = *(snap.Status.ReadyToUse)
		}
	}

	return ready, nil
}

func (r *BackupReconciler) createUpdatesJobs(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	basePolicy *dataprotectionv1alpha1.BasePolicy,
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
	for _, update := range basePolicy.BackupStatusUpdates {
		if update.UpdateStage != stage {
			continue
		}
		if err := r.createMetadataCollectionJob(reqCtx, backup, basePolicy, update); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) createMetadataCollectionJob(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	basePolicy *dataprotectionv1alpha1.BasePolicy,
	updateInfo dataprotectionv1alpha1.BackupStatusUpdate) error {
	mgrNS := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	jobName := backup.Name
	if len(backup.Name) > 30 {
		jobName = backup.Name[:30]
	}
	key := types.NamespacedName{Namespace: mgrNS, Name: jobName + "-" + strings.ToLower(updateInfo.Path)}
	job := &batchv1.Job{}
	// check if job is created
	if exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, job); err != nil {
		return err
	} else if exists {
		return nil
	}

	// build job and create
	jobPodSpec, err := r.buildMetadataCollectionPodSpec(reqCtx, backup, basePolicy, updateInfo)
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

func (r *BackupReconciler) createBackupToolJob(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	pathPrefix string) error {

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

	toolPodSpec, err := r.buildBackupToolPodSpec(reqCtx, backup, commonPolicy, pathPrefix)
	if err != nil {
		return err
	}

	if err = r.createBatchV1Job(reqCtx, key, backup, toolPodSpec); err != nil {
		return err
	}
	msg := fmt.Sprintf("Waiting for a job %s to be created.", key.Name)
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

	msg := fmt.Sprintf("Waiting for a job %s to be created.", key.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingJob-"+key.Name, msg)

	return r.createBatchV1Job(reqCtx, key, backup, jobPodSpec)
}

func buildBackupLabels(backup *dataprotectionv1alpha1.Backup) map[string]string {
	labels := backup.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels[dataProtectionLabelBackupNameKey] = backup.Name
	return labels
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
			Labels:    buildBackupLabels(backup),
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

	reqCtx.Log.V(1).Info("create a built-in job from backup", "job", job)
	return client.IgnoreAlreadyExists(r.Client.Create(reqCtx.Ctx, job))
}

func (r *BackupReconciler) getBatchV1Job(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	jobNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Name,
	}
	if err := r.Client.Get(reqCtx.Ctx, jobNameSpaceName, job); err != nil {
		// not found backup, do nothing
		reqCtx.Log.Info(err.Error())
		return nil, err
	}
	return job, nil
}

func (r *BackupReconciler) deleteReferenceBatchV1Jobs(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
	jobs := &batchv1.JobList{}
	namespace := backup.Namespace
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		namespace = viper.GetString(constant.CfgKeyCtrlrMgrNS)
	}
	if err := r.Client.List(reqCtx.Ctx, jobs,
		client.InNamespace(namespace),
		client.MatchingLabels(buildBackupLabels(backup))); err != nil {
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

	if err := r.Client.List(reqCtx.Ctx, snaps,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(buildBackupLabels(backup))); err != nil {
		return err
	}
	for _, i := range snaps.Items {
		if controllerutil.ContainsFinalizer(&i, dataProtectionFinalizerName) {
			patch := client.MergeFrom(i.DeepCopy())
			controllerutil.RemoveFinalizer(&i, dataProtectionFinalizerName)
			if err := r.Patch(reqCtx.Ctx, &i, patch); err != nil {
				return err
			}
		}
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &i); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
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
// then get the pod from this annotation to ensure that the same pod is picked in following up .
func (r *BackupReconciler) getTargetPod(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup, labels map[string]string) (*corev1.Pod, error) {
	if targetPodName, ok := backup.Annotations[dataProtectionBackupTargetPodKey]; ok {
		targetPod := &corev1.Pod{}
		targetPodKey := types.NamespacedName{
			Name:      targetPodName,
			Namespace: backup.Namespace,
		}
		if err := r.Client.Get(reqCtx.Ctx, targetPodKey, targetPod); err != nil {
			return nil, err
		}
		return targetPod, nil
	}
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
		return nil, errors.New("can not find any pvc to backup by labelsSelector")
	}

	allPVCs := []corev1.PersistentVolumeClaim{*dataPVC}
	if logPVC != nil {
		allPVCs = append(allPVCs, *logPVC)
	}

	return allPVCs, nil
}

func (r *BackupReconciler) buildBackupToolPodSpec(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	pathPrefix string) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	// get backup tool
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	backupToolNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      commonPolicy.BackupToolName,
	}
	if err := r.Client.Get(reqCtx.Ctx, backupToolNameSpaceName, backupTool); err != nil {
		reqCtx.Log.Error(err, "Unable to get backupTool for backup.", "BackupTool", backupToolNameSpaceName)
		return podSpec, err
	}
	// TODO: check if pvc exists
	clusterPod, err := r.getTargetPod(reqCtx, backup, commonPolicy.Target.LabelsSelector.MatchLabels)
	if err != nil {
		return podSpec, err
	}

	// build pod dns string
	// ref: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/
	hostDNS := []string{clusterPod.Name}
	if clusterPod.Spec.Hostname != "" {
		hostDNS[0] = clusterPod.Spec.Hostname
	}
	if clusterPod.Spec.Subdomain != "" {
		hostDNS = append(hostDNS, clusterPod.Spec.Subdomain)
	}
	envDBHost := corev1.EnvVar{
		Name:  "DB_HOST",
		Value: strings.Join(hostDNS, "."),
	}

	container := corev1.Container{}
	container.Name = backup.Name
	container.Command = []string{"sh", "-c"}
	container.Args = backupTool.Spec.BackupCommands
	container.Image = backupTool.Spec.Image
	if backupTool.Spec.Resources != nil {
		container.Resources = *backupTool.Spec.Resources
	}

	remoteBackupPath := "/backupdata"

	// TODO(dsj): mount multi remote backup volumes
	remoteVolumeName := fmt.Sprintf("backup-%s", commonPolicy.PersistentVolumeClaim.Name)
	remoteVolume := corev1.Volume{
		Name: remoteVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: commonPolicy.PersistentVolumeClaim.Name,
			},
		},
	}
	remoteVolumeMount := corev1.VolumeMount{
		Name:      remoteVolumeName,
		MountPath: remoteBackupPath,
	}
	container.VolumeMounts = clusterPod.Spec.Containers[0].VolumeMounts
	container.VolumeMounts = append(container.VolumeMounts, remoteVolumeMount)
	allowPrivilegeEscalation := false
	runAsUser := int64(0)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                &runAsUser}

	envBackupName := corev1.EnvVar{
		Name:  "BACKUP_NAME",
		Value: backup.Name,
	}

	envBackupDir := corev1.EnvVar{
		Name:  "BACKUP_DIR",
		Value: remoteBackupPath + pathPrefix,
	}

	container.Env = []corev1.EnvVar{envDBHost, envBackupName, envBackupDir}
	if commonPolicy.Target.Secret != nil {
		envDBUser := corev1.EnvVar{
			Name: "DB_USER",
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
			Name: "DB_PASSWORD",
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

	// merge env from backup tool.
	container.Env = append(container.Env, backupTool.Spec.Env...)

	podSpec.Containers = []corev1.Container{container}

	podSpec.Volumes = clusterPod.Spec.Volumes
	podSpec.Volumes = append(podSpec.Volumes, remoteVolume)
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	// the pod of job needs to be scheduled on the same node as the workload pod, because it needs to share one pvc
	// see: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodename
	podSpec.NodeName = clusterPod.Spec.NodeName

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

func generateJSON(path string, value string) string {
	segments := strings.Split(path, ".")
	jsonString := value
	for i := len(segments) - 1; i >= 0; i-- {
		jsonString = fmt.Sprintf(`{\"%s\":%s}`, segments[i], jsonString)
	}
	return jsonString
}

func addTolerations(podSpec *corev1.PodSpec) (err error) {
	if cmTolerations := viper.GetString(constant.CfgKeyCtrlrMgrTolerations); cmTolerations != "" {
		if err = json.Unmarshal([]byte(cmTolerations), &podSpec.Tolerations); err != nil {
			return err
		}
	}
	if cmAffinity := viper.GetString(constant.CfgKeyCtrlrMgrAffinity); cmAffinity != "" {
		if err = json.Unmarshal([]byte(cmAffinity), &podSpec.Affinity); err != nil {
			return err
		}
	}
	if cmNodeSelector := viper.GetString(constant.CfgKeyCtrlrMgrNodeSelector); cmNodeSelector != "" {
		if err = json.Unmarshal([]byte(cmNodeSelector), &podSpec.NodeSelector); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) buildMetadataCollectionPodSpec(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	basePolicy *dataprotectionv1alpha1.BasePolicy,
	updateInfo dataprotectionv1alpha1.BackupStatusUpdate) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	targetPod, err := r.getTargetPod(reqCtx, backup, basePolicy.Target.LabelsSelector.MatchLabels)
	if err != nil {
		return podSpec, err
	}

	container := corev1.Container{}
	container.Name = backup.Name
	container.Command = []string{"sh", "-c"}
	args := "set -o errexit; set -o nounset;" +
		"OUTPUT=$(kubectl -n %s exec -it pod/%s -c %s -- %s);" +
		"kubectl -n %s patch backup %s --subresource=status --type=merge --patch \"%s\";"
	patchJSON := generateJSON("status."+updateInfo.Path, "$OUTPUT")
	args = fmt.Sprintf(args, targetPod.Namespace, targetPod.Name, updateInfo.ContainerName,
		updateInfo.Script, backup.Namespace, backup.Name, patchJSON)
	container.Args = []string{args}
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	podSpec.Containers = []corev1.Container{container}
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.ServiceAccountName = viper.GetString("KUBEBLOCKS_SERVICEACCOUNT_NAME")

	if err = addTolerations(&podSpec); err != nil {
		return podSpec, err
	}

	return podSpec, nil
}

// getClusterObjectString gets the cluster object and convert it to string.
func (r *BackupReconciler) getClusterObjectString(reqCtx intctrlutil.RequestCtx, name types.NamespacedName) (*string, error) {
	cluster := &appsv1alpha1.Cluster{}
	// cluster snapshot is optional, so we don't return error if it doesn't exist.
	if err := r.Client.Get(reqCtx.Ctx, name, cluster); err != nil {
		return nil, nil
	}
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
func (r *BackupReconciler) setClusterSnapshotAnnotation(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup, name types.NamespacedName) error {
	clusterString, err := r.getClusterObjectString(reqCtx, name)
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
