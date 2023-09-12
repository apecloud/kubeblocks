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
	"reflect"
	"strings"
	"time"

	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
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
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/dataprotection/action"
	dpbackup "github.com/apecloud/kubeblocks/internal/dataprotection/backup"
	dperrors "github.com/apecloud/kubeblocks/internal/dataprotection/errors"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
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
	Scheme     *k8sruntime.Scheme
	Recorder   record.EventRecorder
	RestConfig *rest.Config
	clock      clock.RealClock
	vsCli      *intctrlutil.VolumeSnapshotCompatClient
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
// move the current state of the backup closer to the desired state.
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backup", req.NamespacedName),
		Recorder: r.Recorder,
	}

	// initialize volume snapshot client that is compatible with both v1beta1 and v1
	r.vsCli = &intctrlutil.VolumeSnapshotCompatClient{
		Client: r.Client,
		Ctx:    ctx,
	}

	// get backup object, and return if not found
	backup := &dpv1alpha1.Backup{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backup); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	reqCtx.Log.V(1).Info("reconcile", "backup", req.NamespacedName, "phase", backup.Status.Phase)

	res, err := r.handleDeletion(reqCtx, backup)
	if res != nil {
		return *res, err
	}

	switch backup.Status.Phase {
	case "", dpv1alpha1.BackupPhaseNew:
		return r.handleNewPhase(reqCtx, backup)
	case dpv1alpha1.BackupPhaseRunning:
		return r.handleRunningPhase(reqCtx, backup)
	case dpv1alpha1.BackupPhaseCompleted:
		return r.handleCompletedPhase(reqCtx, backup)
	default:
		return intctrlutil.Reconciled()
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.Backup{}).
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
func (r *BackupReconciler) checkPodsOfStatefulSetHasDeleted(reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) (bool, error) {
	podList := &corev1.PodList{}
	if err := r.Client.List(reqCtx.Ctx, podList,
		client.InNamespace(backup.Namespace),
		client.MatchingLabels(dpbackup.BuildBackupWorkloadLabels(backup))); err != nil {
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

// deleteBackupFiles deletes the backup files stored in backup repository.
func (r *BackupReconciler) deleteBackupFiles(reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) error {
	hasDeleted, err := r.checkPodsOfStatefulSetHasDeleted(reqCtx, backup)
	if err != nil {
		return err
	}
	// wait for pods of sts clean up successfully
	if !hasDeleted {
		return fmt.Errorf("waiting for pods of statefulset %s/%s to be deleted", backup.Namespace, backup.Name)
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

// handleDeletion handles the deletion of backup. It will delete the backup CR
// and the backup workload(job/statefulset).
func (r *BackupReconciler) handleDeletion(reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) (*ctrl.Result, error) {
	// if backup phase is Deleting, delete the backup reference workloads,
	// backup data stored in backup repository and volume snapshots.
	// TODO(ldm): if backup is being used by restore, do not delete it.
	if backup.Status.Phase == dpv1alpha1.BackupPhaseDeleting {
		if err := r.deleteExternalResources(reqCtx, backup); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
		}

		if backup.Spec.DeletionPolicy == dpv1alpha1.BackupDeletionPolicyRetain {
			return intctrlutil.ResultToP(intctrlutil.Reconciled())
		}

		if err := r.deleteVolumeSnapshots(reqCtx, backup); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
		}

		if err := r.deleteBackupFiles(reqCtx, backup); err != nil {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}

	// if backup CR is being deleted, set backup phase to deleting. The backup
	// reference workloads, data and volume snapshots will be deleted by controller
	// later when the backup CR status.phase is deleting.
	if !backup.GetDeletionTimestamp().IsZero() {
		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status.Phase = dpv1alpha1.BackupPhaseDeleting
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
	backupName, ok := labels[dptypes.DataProtectionLabelBackupNameKey]
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

func (r *BackupReconciler) handleNewPhase(
	reqCtx intctrlutil.RequestCtx,
	backup *dpv1alpha1.Backup) (ctrl.Result, error) {
	request, err := r.prepareBackupRequest(reqCtx, backup)
	if err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, request.Backup, err)
	}

	// set and patch backup object meta, including labels, annotations and finalizers
	// if the backup object meta is changed, the backup object will be patched.
	if patched, err := r.patchBackupObjectMeta(backup, request); err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, request.Backup, err)
	} else if patched {
		return intctrlutil.Reconciled()
	}

	// set and patch backup status
	if err = r.patchBackupStatus(backup, request); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

// prepareBackupRequest prepares a request for a backup, with all references to
// other objects, validate them.
func (r *BackupReconciler) prepareBackupRequest(
	reqCtx intctrlutil.RequestCtx,
	backup *dpv1alpha1.Backup) (*dpbackup.Request, error) {
	request := &dpbackup.Request{
		Backup:     backup.DeepCopy(),
		RequestCtx: reqCtx,
		Client:     r.Client,
	}

	if request.Annotations == nil {
		request.Annotations = make(map[string]string)
	}

	if request.Labels == nil {
		request.Labels = make(map[string]string)
	}

	backupPolicy, err := getBackupPolicyByName(reqCtx, r.Client, backup.Spec.BackupPolicyName)
	if err != nil {
		return nil, err
	}

	targetPods, err := getTargetPods(reqCtx, r.Client,
		backup.Annotations[dataProtectionBackupTargetPodKey], backupPolicy)
	if err != nil || len(targetPods) == 0 {
		return nil, fmt.Errorf("failed to get target pods by backup policy %s/%s",
			backupPolicy.Namespace, backupPolicy.Name)
	}

	if len(targetPods) > 1 {
		return nil, fmt.Errorf("do not support more than one target pods")
	}

	backupMethod := getBackupMethodByName(backup.Spec.BackupMethod, backupPolicy)
	if backupMethod == nil {
		return nil, intctrlutil.NewNotFound("backupMethod: %s not found",
			backup.Spec.BackupMethod)
	}

	// backupMethod should specify snapshotVolumes or actionSetName, if we take
	// snapshots to back up volumes, the snapshotVolumes should be set to true
	// and the actionSetName is not required, if we do not take snapshots to back
	// up volumes, the actionSetName is required.
	snapshotVolumes := boolptr.IsSetToTrue(backupMethod.SnapshotVolumes)
	if !snapshotVolumes && backupMethod.ActionSetName == "" {
		return nil, fmt.Errorf("backup method %s should specify snapshotVolumes or actionSetName", backupMethod.Name)
	}

	if backupMethod.ActionSetName != "" {
		actionSet, err := getActionSetByName(reqCtx, r.Client, backupMethod.ActionSetName)
		if err != nil {
			return nil, err
		}
		request.ActionSet = actionSet
	}

	// TODO(ldm): validate user can not create continuous backup

	request.BackupPolicy = backupPolicy
	if err = r.handleBackupRepo(request); err != nil {
		return nil, err
	}

	request.BackupMethod = backupMethod
	request.TargetPods = targetPods
	return request, nil
}

// handleBackupRepo handles the backup repo, and get the backup repo PVC. If the
// PVC is not present, it will add a special label and wait for the backup repo
// controller to create the PVC.
func (r *BackupReconciler) handleBackupRepo(request *dpbackup.Request) error {
	repo, err := r.getBackupRepo(request.Ctx, request.Backup, request.BackupPolicy)
	if err != nil {
		return err
	}
	request.BackupRepo = repo

	pvcName := repo.Status.BackupPVCName
	if pvcName == "" {
		return dperrors.NewBackupPVCNameIsEmpty(repo.Name, request.Spec.BackupPolicyName)
	}

	pvc := &corev1.PersistentVolumeClaim{}
	pvcKey := client.ObjectKey{Namespace: request.Req.Namespace, Name: pvcName}
	if err = r.Client.Get(request.Ctx, pvcKey, pvc); client.IgnoreNotFound(err) != nil {
		return err
	}

	// backupRepo PVC exists, record the PVC name
	if err == nil {
		request.BackupRepoPVC = pvc
	}
	return nil
}

func (r *BackupReconciler) patchBackupStatus(
	original *dpv1alpha1.Backup,
	request *dpbackup.Request) error {
	request.Status.FormatVersion = dpbackup.FormatVersion
	request.Status.Path = getBackupPath(request.Backup, request.BackupPolicy.Spec.PathPrefix)
	request.Status.Target = request.BackupPolicy.Spec.Target
	request.Status.BackupMethod = request.BackupMethod
	request.Status.PersistentVolumeClaimName = request.BackupRepoPVC.Name
	request.Status.BackupRepoName = request.BackupRepo.Name

	// init action status
	actions, err := request.BuildActions()
	if err != nil {
		return err
	}
	request.Status.Actions = make([]dpv1alpha1.ActionStatus, len(actions))
	for i, act := range actions {
		request.Status.Actions[i] = dpv1alpha1.ActionStatus{
			Name:       act.GetName(),
			Phase:      dpv1alpha1.ActionPhaseNew,
			ActionType: act.Type(),
		}
	}

	// update phase to running
	request.Status.Phase = dpv1alpha1.BackupPhaseRunning
	request.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}

	if original.Spec.RetentionPeriod != "" {
		original.Status.Expiration = &metav1.Time{
			Time: request.Status.StartTimestamp.Add(original.Spec.RetentionPeriod.ToDuration()),
		}
	}
	return r.Client.Status().Patch(request.Ctx, request.Backup, client.MergeFrom(original))
}

// patchBackupObjectMeta patches backup object metaObject include cluster snapshot.
func (r *BackupReconciler) patchBackupObjectMeta(
	original *dpv1alpha1.Backup,
	request *dpbackup.Request) (bool, error) {
	targetPod := request.TargetPods[0]

	// get KubeBlocks cluster and set labels and annotations for backup
	// TODO(ldm): we should remove this dependency of cluster in the future
	cluster := getCluster(request.Ctx, r.Client, targetPod)
	if cluster != nil {
		if err := r.setClusterSnapshotAnnotation(request.Backup, cluster); err != nil {
			return false, err
		}
		request.Labels[dptypes.DataProtectionLabelClusterUIDKey] = string(cluster.UID)
	}
	for _, v := range getClusterLabelKeys() {
		request.Labels[v] = targetPod.Labels[v]
	}

	request.Labels[dataProtectionBackupRepoKey] = request.BackupRepo.Name
	request.Labels[constant.AppManagedByLabelKey] = constant.AppName
	request.Labels[dataProtectionLabelBackupTypeKey] = request.GetBackupType()

	// the backupRepo PVC is not present, add a special label and wait for the
	// backup repo controller to create the PVC.
	wait := false
	if request.BackupRepoPVC == nil {
		request.Labels[dataProtectionNeedRepoPVCKey] = trueVal
		wait = true
	}

	// set annotations
	request.Annotations[dataProtectionBackupTargetPodKey] = targetPod.Name

	// set finalizer
	controllerutil.AddFinalizer(request.Backup, dataProtectionFinalizerName)

	if reflect.DeepEqual(original.ObjectMeta, request.ObjectMeta) {
		return wait, nil
	}

	return true, r.Client.Patch(request.Ctx, request.Backup, client.MergeFrom(original))
}

// getBackupRepo returns the backup repo specified by the backup object or the policy.
// if no backup repo specified, it will return the default one.
func (r *BackupReconciler) getBackupRepo(ctx context.Context,
	backup *dpv1alpha1.Backup,
	backupPolicy *dpv1alpha1.BackupPolicy) (*dpv1alpha1.BackupRepo, error) {
	// use the specified backup repo
	var repoName string
	if val := backup.Labels[dataProtectionBackupRepoKey]; val != "" {
		repoName = val
	} else if backupPolicy.Spec.BackupRepoName != nil && *backupPolicy.Spec.BackupRepoName != "" {
		repoName = *backupPolicy.Spec.BackupRepoName
	}
	if repoName != "" {
		repo := &dpv1alpha1.BackupRepo{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: repoName}, repo); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, intctrlutil.NewNotFound("backup repo %s not found", repoName)
			}
			return nil, err
		}
		return repo, nil
	}
	// fallback to use the default repo
	return getDefaultBackupRepo(ctx, r.Client)
}

func (r *BackupReconciler) handleRunningPhase(
	reqCtx intctrlutil.RequestCtx,
	backup *dpv1alpha1.Backup) (ctrl.Result, error) {
	request, err := r.prepareBackupRequest(reqCtx, backup)
	if err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, request.Backup, err)
	}

	// there are actions not completed, continue to handle following actions
	actions, err := request.BuildActions()
	if err != nil {
		return r.updateStatusIfFailed(reqCtx, backup, request.Backup, err)
	}

	actionCtx := action.Context{
		Ctx:              reqCtx.Ctx,
		Client:           r.Client,
		Recorder:         r.Recorder,
		Scheme:           r.Scheme,
		RestClientConfig: r.RestConfig,
	}

	// check all actions status, if any action failed, update backup status to failed
	// if all actions completed, update backup status to completed, otherwise,
	// continue to handle following actions.
	for i, act := range actions {
		status, err := act.Execute(actionCtx)
		if err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "backup action", act.GetName())
		}
		request.Status.Actions[i] = mergeActionStatus(&request.Status.Actions[i], status)

		switch status.Phase {
		case dpv1alpha1.ActionPhaseCompleted:
			updateBackupStatusByActionStatus(&request.Status)
			continue
		case dpv1alpha1.ActionPhaseFailed:
			return r.updateStatusIfFailed(reqCtx, backup, request.Backup,
				fmt.Errorf("action %s failed, reason %s", act.GetName(), status.FailureReason))
		case dpv1alpha1.ActionPhaseRunning:
			// update status
			if err = r.Client.Status().Patch(reqCtx.Ctx, request.Backup, client.MergeFrom(backup)); err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		}
	}

	// all actions completed, update backup status to completed
	request.Status.Phase = dpv1alpha1.BackupPhaseCompleted
	request.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	if !request.Status.StartTimestamp.IsZero() {
		// round the duration to a multiple of seconds.
		duration := request.Status.CompletionTimestamp.Sub(request.Status.StartTimestamp.Time).Round(time.Second)
		request.Status.Duration = &metav1.Duration{Duration: duration}
	}
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedBackup", "Completed backup")
	if err := r.Client.Status().Patch(reqCtx.Ctx, request.Backup, client.MergeFrom(backup)); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func mergeActionStatus(original, new *dpv1alpha1.ActionStatus) dpv1alpha1.ActionStatus {
	as := new.DeepCopy()
	if original.StartTimestamp != nil {
		as.StartTimestamp = original.StartTimestamp
	}
	return *as
}

func updateBackupStatusByActionStatus(backupStatus *dpv1alpha1.BackupStatus) {
	for _, act := range backupStatus.Actions {
		if act.TotalSize != "" && backupStatus.TotalSize == "" {
			backupStatus.TotalSize = act.TotalSize
		}
		if act.TimeRange != nil && backupStatus.TimeRange == nil {
			backupStatus.TimeRange = act.TimeRange
		}
	}
}

// checkBackupIsCompletedDuringRunning checks if backup is completed during it is running.
// it returns ture, if logfile schedule is disabled or cluster is deleted.
func (r *BackupReconciler) checkBackupIsCompletedDuringRunning(reqCtx intctrlutil.RequestCtx,
	backup *dpv1alpha1.Backup) (*dpv1alpha1.BackupPolicy, bool, error) {
	backupPolicy := &dpv1alpha1.BackupPolicy{}
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
		clusterName := backup.Labels[dptypes.AppInstanceLabelKey]
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
	backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
	backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	if !backup.Status.StartTimestamp.IsZero() {
		// round the duration to a multiple of seconds.
		duration := backup.Status.CompletionTimestamp.Sub(backup.Status.StartTimestamp.Time).Round(time.Second)
		backup.Status.Duration = &metav1.Duration{Duration: duration}
	}
	return backupPolicy, true, r.Client.Status().Patch(reqCtx.Ctx, backup, patch)
}

// handleCompletedPhase handles the backup object in completed phase.
// It will delete the reference workloads.
func (r *BackupReconciler) handleCompletedPhase(
	reqCtx intctrlutil.RequestCtx,
	backup *dpv1alpha1.Backup) (ctrl.Result, error) {
	if err := r.deleteExternalResources(reqCtx, backup); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupReconciler) updateStatusIfFailed(
	reqCtx intctrlutil.RequestCtx,
	original *dpv1alpha1.Backup,
	backup *dpv1alpha1.Backup,
	err error) (ctrl.Result, error) {
	sendWarningEventForError(r.Recorder, backup, err)
	backup.Status.Phase = dpv1alpha1.BackupPhaseFailed
	backup.Status.FailureReason = err.Error()
	if errUpdate := r.Client.Status().Patch(reqCtx.Ctx, backup, client.MergeFrom(original)); errUpdate != nil {
		return intctrlutil.CheckedRequeueWithError(errUpdate, reqCtx.Log, "")
	}
	return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
}

func (r *BackupReconciler) createDeleteBackupFileJob(
	reqCtx intctrlutil.RequestCtx,
	jobKey types.NamespacedName,
	backup *dpv1alpha1.Backup,
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
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&container)

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

// deleteExternalJobs deletes the external jobs.
func (r *BackupReconciler) deleteExternalJobs(reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) error {
	jobs := &batchv1.JobList{}
	if err := r.Client.List(reqCtx.Ctx, jobs,
		client.InNamespace(backup.Namespace),
		client.MatchingLabels(dpbackup.BuildBackupWorkloadLabels(backup))); err != nil {
		return client.IgnoreNotFound(err)
	}

	deleteJob := func(job *batchv1.Job) error {
		if controllerutil.ContainsFinalizer(job, dataProtectionFinalizerName) {
			patch := client.MergeFrom(job.DeepCopy())
			controllerutil.RemoveFinalizer(job, dataProtectionFinalizerName)
			if err := r.Patch(reqCtx.Ctx, job, patch); err != nil {
				return err
			}
		}
		if !job.DeletionTimestamp.IsZero() {
			return nil
		}
		reqCtx.Log.V(1).Info("delete job", "job", job)
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, job); err != nil {
			return err
		}
		return nil
	}

	for i := range jobs.Items {
		if err := deleteJob(&jobs.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) deleteVolumeSnapshots(reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) error {
	snaps := &snapshotv1.VolumeSnapshotList{}
	if err := r.vsCli.List(snaps, client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(buildBackupWorkloadLabels(backup))); err != nil {
		return client.IgnoreNotFound(err)
	}

	deleteVolumeSnapshot := func(vs *snapshotv1.VolumeSnapshot) error {
		if controllerutil.ContainsFinalizer(vs, dataProtectionFinalizerName) {
			patch := vs.DeepCopy()
			controllerutil.RemoveFinalizer(vs, dataProtectionFinalizerName)
			if err := r.vsCli.Patch(vs, patch); err != nil {
				return err
			}
		}
		if !vs.DeletionTimestamp.IsZero() {
			return nil
		}
		reqCtx.Log.V(1).Info("delete volume snapshot", "volume snapshot", vs)
		if err := r.vsCli.Delete(vs); err != nil {
			return err
		}
		return nil
	}

	for i := range snaps.Items {
		if err := deleteVolumeSnapshot(&snaps.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupReconciler) handleDeleteBackupFiles(reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) (*batchv1.Job, error) {
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

		backupFilePath := backup.Status.Path
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

// deleteExternalStatefulSet deletes the external statefulSet.
func (r *BackupReconciler) deleteExternalStatefulSet(reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) error {
	key := client.ObjectKey{
		Namespace: backup.Namespace,
		Name:      backup.Name,
	}
	sts := &appsv1.StatefulSet{}
	if err := r.Client.Get(reqCtx.Ctx, key, sts); err != nil {
		return client.IgnoreNotFound(err)
	} else if !model.IsOwnerOf(backup, sts) {
		return nil
	}

	patch := client.MergeFrom(sts.DeepCopy())
	controllerutil.RemoveFinalizer(sts, dataProtectionFinalizerName)
	if err := r.Client.Patch(reqCtx.Ctx, sts, patch); err != nil {
		return err
	}

	if !sts.DeletionTimestamp.IsZero() {
		return nil
	}

	reqCtx.Log.V(1).Info("delete statefulSet", "statefulSet", sts)
	return intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, sts)
}

// deleteExternalResources deletes the external workloads that execute backup.
// Currently, it only supports two types of workloads: statefulSet and job.
func (r *BackupReconciler) deleteExternalResources(
	reqCtx intctrlutil.RequestCtx, backup *dpv1alpha1.Backup) error {
	if err := r.deleteExternalStatefulSet(reqCtx, backup); err != nil {
		return err
	}
	return r.deleteExternalJobs(reqCtx, backup)
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
func (r *BackupReconciler) setClusterSnapshotAnnotation(backup *dpv1alpha1.Backup, cluster *appsv1alpha1.Cluster) error {
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
