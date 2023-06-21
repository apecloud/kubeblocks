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

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backup, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backup)
	})
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
	if backup.Name != getCreatedCRNameByBackupPolicy(backupPolicy.Name, backupPolicy.Namespace, backupType) {
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

	// TODO: get pod with matching labels to do backup.
	var targetCluster dataprotectionv1alpha1.TargetCluster
	var isStatefulSetKind bool
	switch backup.Spec.BackupType {
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		targetCluster = backupPolicy.Spec.Snapshot.Target
	default:
		commonPolicy := backupPolicy.Spec.GetCommonPolicy(backup.Spec.BackupType)
		if commonPolicy == nil {
			return r.updateStatusIfFailed(reqCtx, backup, intctrlutil.NewBackupNotSupported(string(backup.Spec.BackupType), backupPolicy.Name))
		}
		// save the backup message for restore
		backupToolName := commonPolicy.BackupToolName
		backup.Status.PersistentVolumeClaimName = commonPolicy.PersistentVolumeClaim.Name
		backup.Status.BackupToolName = backupToolName
		pathPrefix := getBackupPathPrefix(backup, backupPolicy.Annotations[constant.BackupDataPathPrefixAnnotationKey])
		if backup.Status.Manifests == nil {
			backup.Status.Manifests = &dataprotectionv1alpha1.ManifestsStatus{}
		}
		if backup.Status.Manifests.BackupTool == nil {
			backup.Status.Manifests.BackupTool = &dataprotectionv1alpha1.BackupToolManifestsStatus{}
		}
		backup.Status.Manifests.BackupTool.FilePath = pathPrefix
		targetCluster = commonPolicy.Target
		if err = r.handlePersistentVolumeClaim(reqCtx, backup.Spec.BackupType, backupPolicy.Name, commonPolicy); err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, err)
		}
		backupTool, err := getBackupToolByName(reqCtx, r.Client, backupToolName)
		if err != nil {
			return r.updateStatusIfFailed(reqCtx, backup, intctrlutil.NewNotFound("backupTool: %s not found", backupToolName))
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

	if hasPatch, err := r.patchBackupLabelsAndAnnotations(reqCtx, backup, target); err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, err)
	} else if hasPatch {
		return intctrlutil.Reconciled()
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

	if err = r.Client.Status().Patch(reqCtx.Ctx, backup, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

// handlePersistentVolumeClaim handles the persistent volume claim for the backup, the rules are as follows
// - if CreatePolicy is "Never", it will check if the pvc exists. if not existed, then report an error.
// - if CreatePolicy is "IfNotPresent" and the pvc not existed, then create the pvc automatically.
func (r *BackupReconciler) handlePersistentVolumeClaim(reqCtx intctrlutil.RequestCtx,
	backupType dataprotectionv1alpha1.BackupType,
	backupPolicyName string,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) error {
	pvcConfig := commonPolicy.PersistentVolumeClaim
	if len(pvcConfig.Name) == 0 {
		return intctrlutil.NewBackupPVCNameIsEmpty(string(backupType), backupPolicyName)
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
		// create the persistentVolume with the template
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
	pathPrefix := getBackupPathPrefix(backup, backupPolicy.Annotations[constant.BackupDataPathPrefixAnnotationKey])
	patch := client.MergeFrom(backup.DeepCopy())
	var res *ctrl.Result
	switch backup.Spec.BackupType {
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		res, err = r.doSnapshotInProgressPhaseAction(reqCtx, backup, backupPolicy, pathPrefix)
	default:
		res, err = r.doBaseBackupInProgressPhaseAction(reqCtx, backup, backupPolicy, pathPrefix)
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
	pathPrefix string) (*ctrl.Result, error) {
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
	if err = r.createUpdatesJobs(reqCtx, backup, nil, &snapshotSpec.BasePolicy, pathPrefix, dataprotectionv1alpha1.PRE); err != nil {
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
	if err = r.createUpdatesJobs(reqCtx, backup, nil, &snapshotSpec.BasePolicy, pathPrefix, dataprotectionv1alpha1.POST); err != nil {
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
	pathPrefix string) (*ctrl.Result, error) {
	// 1. create and ensure backup tool job finished
	// 2. get job phase and update
	commonPolicy := backupPolicy.Spec.GetCommonPolicy(backup.Spec.BackupType)
	if commonPolicy == nil {
		// TODO: add error type
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, fmt.Errorf("not found the %s policy", backup.Spec.BackupType)))
	}
	// createUpdatesJobs should not affect the backup status, just need to record events when the run fails
	if err := r.createUpdatesJobs(reqCtx, backup, commonPolicy, &commonPolicy.BasePolicy, pathPrefix, dataprotectionv1alpha1.PRE); err != nil {
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPreUpdatesJob", err.Error())
	}
	if err := r.createBackupToolJob(reqCtx, backup, backupPolicy, commonPolicy, pathPrefix); err != nil {
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
	job, err := getBackupBatchV1Job(reqCtx, r.Client, backup)
	if err != nil {
		return intctrlutil.ResultToP(r.updateStatusIfFailed(reqCtx, backup, err))
	}
	// createUpdatesJobs should not affect the backup status, just need to record events when the run fails
	if err = r.createUpdatesJobs(reqCtx, backup, commonPolicy, &commonPolicy.BasePolicy, pathPrefix, dataprotectionv1alpha1.POST); err != nil {
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedPostUpdatesJob", err.Error())
	}
	jobStatusConditions := job.Status.Conditions
	if jobStatusConditions[0].Type == batchv1.JobComplete {
		// update Phase to Completed
		backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
		backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	} else if jobStatusConditions[0].Type == batchv1.JobFailed {
		backup.Status.Phase = dataprotectionv1alpha1.BackupFailed
		backup.Status.FailureReason = job.Status.Conditions[0].Reason
	}
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
	backupPolicy, err := r.getBackupPolicyAndValidate(reqCtx, backup)
	if err != nil {
		return err
	}
	if isCompleted, err := r.checkBackupIsCompletedDuringRunning(reqCtx, backup, backupPolicy); err != nil {
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
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy) (bool, error) {
	clusterName := backup.Labels[constant.AppInstanceLabelKey]
	targetClusterExists := true
	if clusterName != "" {
		cluster := &appsv1alpha1.Cluster{}
		var err error
		targetClusterExists, err = intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, types.NamespacedName{Name: clusterName, Namespace: backup.Namespace}, cluster)
		if err != nil {
			return false, err
		}
	}

	schedulePolicy := backupPolicy.Spec.GetCommonSchedulePolicy(backup.Spec.BackupType)
	if schedulePolicy.Enable && targetClusterExists {
		return false, nil
	}
	patch := client.MergeFrom(backup.DeepCopy())
	backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
	backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	return true, r.Client.Status().Patch(reqCtx.Ctx, backup, patch)
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
	pathPrefix string) (corev1.Container, error) {
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
		{Name: fmt.Sprintf("backup-%s", commonPolicy.PersistentVolumeClaim.Name), MountPath: backupPathBase},
	}
	container.Env = []corev1.EnvVar{
		{Name: constant.DPBackupInfoFile, Value: buildBackupInfoENV(pathPrefix)},
	}
	return container, nil
}

func (r *BackupReconciler) buildStatefulSpec(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) (*appsv1.StatefulSetSpec, error) {
	pathPrefix := getBackupPathPrefix(backup, backupPolicy.Annotations[constant.BackupDataPathPrefixAnnotationKey])
	toolPodSpec, err := r.buildBackupToolPodSpec(reqCtx, backup, backupPolicy, commonPolicy, pathPrefix)
	toolPodSpec.RestartPolicy = corev1.RestartPolicyAlways
	if err != nil {
		return nil, err
	}
	// build the manifests updater container for backup.status.manifests
	manifestsUpdaterContainer, err := r.buildManifestsUpdaterContainer(backup, commonPolicy, pathPrefix)
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
	for _, v := range getClusterLabelKeys() {
		backup.Labels[v] = targetPod.Labels[v]
	}
	backup.Labels[constant.AppManagedByLabelKey] = constant.AppName
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
		jobStatusConditions := job.Status.Conditions
		if len(jobStatusConditions) > 0 {
			if jobStatusConditions[0].Type == batchv1.JobComplete {
				return true, nil
			} else if jobStatusConditions[0].Type == batchv1.JobFailed {
				return false, intctrlutil.NewBackupJobFailed(job.Name)
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
	pathPrefix string,
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
		if err := r.createMetadataCollectionJob(reqCtx, backup, commonPolicy, basePolicy, pathPrefix, update); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) createMetadataCollectionJob(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy,
	basePolicy *dataprotectionv1alpha1.BasePolicy,
	pathPrefix string,
	updateInfo dataprotectionv1alpha1.BackupStatusUpdate) error {
	jobNamespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	// if specified to use the service account of target pod, the namespace should be the namespace of backup.
	if updateInfo.UseTargetPodServiceAccount {
		jobNamespace = backup.Namespace
	}
	key := types.NamespacedName{Namespace: jobNamespace, Name: generateUniqueJobName(backup, "status-"+string(updateInfo.UpdateStage))}
	job := &batchv1.Job{}
	// check if job is created
	if exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, job); err != nil {
		return err
	} else if exists {
		return nil
	}

	// build job and create
	jobPodSpec, err := r.buildMetadataCollectionPodSpec(reqCtx, backup, commonPolicy, basePolicy, pathPrefix, updateInfo)
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
	ttlSecondsAfterSuccess := int32(600)
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
			BackoffLimit:            &backOffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterSuccess,
		},
	}

	reqCtx.Log.V(1).Info("create a job from delete backup files", "job", job)
	return client.IgnoreAlreadyExists(r.Client.Create(reqCtx.Ctx, job))
}

func (r *BackupReconciler) createBackupToolJob(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	backupPolicy *dataprotectionv1alpha1.BackupPolicy,
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

	toolPodSpec, err := r.buildBackupToolPodSpec(reqCtx, backup, backupPolicy, commonPolicy, pathPrefix)
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

func (r *BackupReconciler) deleteBackupFiles(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) error {
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		// no file to delete for this type
		return nil
	}
	if backup.Status.Phase == dataprotectionv1alpha1.BackupNew ||
		backup.Status.Phase == dataprotectionv1alpha1.BackupFailed {
		// nothing to delete
		return nil
	}

	jobName := deleteBackupFilesJobNamePrefix + backup.Name
	if len(jobName) > 60 {
		jobName = jobName[:60]
	}
	jobKey := types.NamespacedName{Namespace: backup.Namespace, Name: jobName}
	job := batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, jobKey, &job)
	if err != nil {
		return err
	}
	// create job for deleting backup files
	if !exists {
		pvcName := backup.Status.PersistentVolumeClaimName
		if pvcName == "" {
			reqCtx.Log.Info("skip deleting backup files because PersistentVolumeClaimName is empty",
				"backup", backup.Name)
			return nil
		}
		// check if pvc exists
		if err = r.Client.Get(reqCtx.Ctx, types.NamespacedName{Namespace: backup.Namespace, Name: pvcName}, &corev1.PersistentVolumeClaim{}); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
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
			return nil
		}
		// the job will run in the background
		if err = r.createDeleteBackupFileJob(reqCtx, jobKey, backup, pvcName, backupFilePath); err != nil {
			return err
		}
	}

	return nil
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
	return r.Client.Delete(reqCtx.Ctx, sts)
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

	// TODO: waiting for cleaning up referenced job/deploy/pod
	if err := r.deleteBackupFiles(reqCtx, backup); err != nil {
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
	pvcName := commonPolicy.PersistentVolumeClaim.Name
	r.appendBackupVolumeMount(pvcName, &podSpec, &podSpec.Containers[0])

	// the pod of job needs to be scheduled on the same node as the workload pod, because it needs to share one pvc
	podSpec.NodeSelector = map[string]string{
		hostNameLabelKey: clusterPod.Spec.NodeName,
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
	pathPrefix string,
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
			{Name: "BACKUP_INFO_FILE", Value: buildBackupInfoENV(pathPrefix)},
		}
		r.appendBackupVolumeMount(commonPolicy.PersistentVolumeClaim.Name, &podSpec, &container)
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
