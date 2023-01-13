/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dataprotection

import (
	"context"
	"errors"
	"fmt"
	"strings"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
	clock    clock.RealClock
}

//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups/finalizers,verbs=update

//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots/finalizers,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Backup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
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
	reqCtx.Log.Info("in Backup Reconciler: name: " + backup.Name + " phase: " + string(backup.Status.Phase))

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

func (r *BackupReconciler) doNewPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (ctrl.Result, error) {

	// HACK/TODO: ought to move following check to validation webhook
	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot && !viper.GetBool("VOLUMESNAPSHOT") {
		backup.Status.Phase = dataprotectionv1alpha1.BackupFailed
		backup.Status.FailureReason = "VolumeSnapshot feature disabled."
		if err := r.Client.Status().Update(reqCtx.Ctx, backup); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	// update labels
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Spec.BackupPolicyName,
	}
	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		r.Recorder.Eventf(backup, corev1.EventTypeWarning, "CreatingBackup",
			"Unable to get backupPolicy for backup %s.", backupPolicyNameSpaceName)
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	labels := backupPolicy.Spec.Target.LabelsSelector.MatchLabels
	labels[dataProtectionLabelBackupTypeKey] = string(backup.Spec.BackupType)
	if err := r.patchBackupLabels(reqCtx, backup, labels); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// update Phase to InProgress
	backup.Status.Phase = dataprotectionv1alpha1.BackupInProgress
	backup.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	if backup.Spec.TTL != nil {
		backup.Status.Expiration = &metav1.Time{
			Time: backup.Status.StartTimestamp.Add(backup.Spec.TTL.Duration),
		}
	}
	if err := r.Client.Status().Update(reqCtx.Ctx, backup); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
}

func (r *BackupReconciler) doInProgressPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (ctrl.Result, error) {

	if backup.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		// 1. create and ensure pre-command job completed
		// 2. create and ensure volume snapshot ready
		// 3. create and ensure post-command job completed
		isOK, err := r.createPreCommandJobAndEnsure(reqCtx, backup)
		if err != nil {
			return r.updateStatusIfJobFailed(reqCtx, backup, err)
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		if err = r.createVolumeSnapshot(reqCtx, backup); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backup.Name}
		isOK, err = r.ensureVolumeSnapshotReady(reqCtx, key)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		msg := fmt.Sprintf("Created volumeSnapshot %s ready.", key.Name)
		r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedVolumeSnapshot", msg)

		isOK, err = r.createPostCommandJobAndEnsure(reqCtx, backup)
		if err != nil {
			return r.updateStatusIfJobFailed(reqCtx, backup, err)
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}

		backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
		backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
		snap := &snapshotv1.VolumeSnapshot{}
		exists, _ := checkResourceExists(reqCtx.Ctx, r.Client, key, snap)
		if exists {
			backup.Status.TotalSize = snap.Status.RestoreSize.String()
		}
	} else {
		// 1. create and ensure backup tool job finished
		// 2. get job phase and update
		err := r.createBackupToolJob(reqCtx, backup)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		key := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}
		isOK, err := r.ensureBatchV1JobCompleted(reqCtx, key)
		if err != nil {
			return r.updateStatusIfJobFailed(reqCtx, backup, err)
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		job, err := r.getBatchV1Job(reqCtx, backup)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		jobStatusConditions := job.Status.Conditions
		if jobStatusConditions[0].Type == batchv1.JobComplete {
			// update Phase to in Completed
			backup.Status.Phase = dataprotectionv1alpha1.BackupCompleted
			backup.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
		} else if jobStatusConditions[0].Type == batchv1.JobFailed {
			backup.Status.Phase = dataprotectionv1alpha1.BackupFailed
			backup.Status.FailureReason = job.Status.Conditions[0].Reason
		}
	}

	// finally, update backup status
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatedBackup", "Completed backup.")
	if backup.Status.CompletionTimestamp != nil {
		backup.Status.Duration = &metav1.Duration{
			Duration: backup.Status.CompletionTimestamp.Sub(backup.Status.StartTimestamp.Time)}
	}
	if err := r.Client.Status().Update(reqCtx.Ctx, backup); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupReconciler) updateStatusIfJobFailed(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup, err error) (ctrl.Result, error) {
	if err.Error() == errorJobFailed {
		r.Recorder.Event(backup, corev1.EventTypeWarning, "FailedCreatedBackup", "Failed creating backup.")
		backup.Status.Phase = dataprotectionv1alpha1.BackupFailed
		backup.Status.FailureReason = err.Error()
		if err := r.Client.Status().Update(reqCtx.Ctx, backup); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
}

// patchBackupLabels patch backup labels
func (r *BackupReconciler) patchBackupLabels(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	labels map[string]string) error {

	patch := client.MergeFrom(backup.DeepCopy())
	if len(labels) > 0 {
		if backup.Labels == nil {
			backup.Labels = labels
		} else {
			for k, v := range labels {
				backup.Labels[k] = v
			}
		}
	}
	return r.Client.Patch(reqCtx.Ctx, backup, patch)
}

func (r *BackupReconciler) createPreCommandJobAndEnsure(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (bool, error) {

	emptyCmd, err := r.ensureEmptyHooksCommand(reqCtx, backup, true)
	if err != nil {
		return false, err
	}
	// if not defined commands, skip create job.
	if emptyCmd {
		return true, err
	}

	key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backup.Name + "-pre"}
	if err := r.createHooksCommandJob(reqCtx, backup, key, true); err != nil {
		return false, err
	}
	return r.ensureBatchV1JobCompleted(reqCtx, key)
}

func (r *BackupReconciler) createPostCommandJobAndEnsure(reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) (bool, error) {

	emptyCmd, err := r.ensureEmptyHooksCommand(reqCtx, backup, false)
	if err != nil {
		return false, err
	}
	// if not defined commands, skip create job.
	if emptyCmd {
		return true, err
	}

	key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backup.Name + "-post"}
	if err := r.createHooksCommandJob(reqCtx, backup, key, false); err != nil {
		return false, err
	}
	return r.ensureBatchV1JobCompleted(reqCtx, key)
}

func (r *BackupReconciler) ensureBatchV1JobCompleted(
	reqCtx intctrlutil.RequestCtx, key types.NamespacedName) (bool, error) {
	job := &batchv1.Job{}
	exists, err := checkResourceExists(reqCtx.Ctx, r.Client, key, job)
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
	backup *dataprotectionv1alpha1.Backup) error {

	snap := &snapshotv1.VolumeSnapshot{}
	exists, err := checkResourceExists(reqCtx.Ctx, r.Client, reqCtx.Req.NamespacedName, snap)
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

	// build env value for access target cluster
	target, err := r.GetTargetCluster(reqCtx, backupPolicy)
	if err != nil {
		return err
	}

	// TODO(dsj): build pvc name 0
	pvcTemplate := []string{target.Spec.VolumeClaimTemplates[0].Name, target.Name, "0"}
	pvcName := strings.Join(pvcTemplate, "-")

	snap = &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqCtx.Req.Namespace,
			Name:      reqCtx.Req.Name,
			Labels:    buildBackupLabels(backup),
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvcName,
			},
		},
	}

	controllerutil.AddFinalizer(snap, dataProtectionFinalizerName)

	scheme, _ := dataprotectionv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(backup, snap, scheme); err != nil {
		return err
	}

	reqCtx.Log.Info("create a volumeSnapshot from backup", "snapshot", snap)
	if err := r.Client.Create(reqCtx.Ctx, snap); err != nil {
		return err
	}
	msg := fmt.Sprintf("Waiting for a volume snapshot %s to be created by the backup.", snap.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingVolumeSnapshot", msg)
	return nil
}

func (r *BackupReconciler) ensureVolumeSnapshotReady(reqCtx intctrlutil.RequestCtx,
	key types.NamespacedName) (bool, error) {

	snap := &snapshotv1.VolumeSnapshot{}
	exists, err := checkResourceExists(reqCtx.Ctx, r.Client, key, snap)
	if err != nil {
		return false, err
	}
	ready := false
	if exists && snap.Status != nil {
		if snap.Status.ReadyToUse != nil {
			ready = *(snap.Status.ReadyToUse)
		}
	}

	return ready, nil
}

func (r *BackupReconciler) createBackupToolJob(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup) error {

	key := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}
	job := batchv1.Job{}
	exists, err := checkResourceExists(reqCtx.Ctx, r.Client, key, &job)
	if err != nil {
		return err
	}
	if exists {
		// find resource object, skip created.
		return nil
	}

	toolPodSpec, err := r.BuildBackupToolPodSpec(reqCtx, backup)
	if err != nil {
		return err
	}

	if err = r.CreateBatchV1Job(reqCtx, key, backup, toolPodSpec); err != nil {
		return err
	}
	msg := fmt.Sprintf("Waiting for a job %s to be created.", key.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingJob", msg)
	return nil
}

func (r *BackupReconciler) ensureEmptyHooksCommand(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	preCommand bool) (bool, error) {

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyKey := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Spec.BackupPolicyName,
	}

	policyExists, err := checkResourceExists(reqCtx.Ctx, r.Client, backupPolicyKey, backupPolicy)
	if err != nil {
		msg := fmt.Sprintf("Failed to get backupPolicy %s .", backupPolicyKey.Name)
		r.Recorder.Event(backup, corev1.EventTypeWarning, "BackupPolicyFailed", msg)
		return false, err
	}

	if !policyExists {
		msg := fmt.Sprintf("Not Found backupPolicy %s .", backupPolicyKey.Name)
		r.Recorder.Event(backup, corev1.EventTypeWarning, "BackupPolicyFailed", msg)
		return false, errors.New(msg)
	}
	commands := backupPolicy.Spec.Hooks.PostCommands
	if preCommand {
		commands = backupPolicy.Spec.Hooks.PreCommands
	}
	if len(commands) == 0 {
		return true, nil
	}
	return false, nil
}

func (r *BackupReconciler) createHooksCommandJob(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	key types.NamespacedName,
	preCommand bool) error {

	job := batchv1.Job{}
	exists, err := checkResourceExists(reqCtx.Ctx, r.Client, key, &job)
	if err != nil {
		return err
	}
	if exists {
		// find resource object, skip created.
		return nil
	}

	jobPodSpec, err := r.BuildSnapshotPodSpec(reqCtx, backup, preCommand)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Waiting for a job %s to be created.", key.Name)
	r.Recorder.Event(backup, corev1.EventTypeNormal, "CreatingJob-"+key.Name, msg)

	return r.CreateBatchV1Job(reqCtx, key, backup, jobPodSpec)
}

func buildBackupLabels(backup *dataprotectionv1alpha1.Backup) map[string]string {
	labels := backup.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels[dataProtectionLabelBackupNameKey] = backup.Name
	return labels
}

func (r *BackupReconciler) CreateBatchV1Job(
	reqCtx intctrlutil.RequestCtx,
	key types.NamespacedName,
	backup *dataprotectionv1alpha1.Backup,
	templatePodSpec corev1.PodSpec) error {

	job := &batchv1.Job{
		//TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
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
		},
	}
	controllerutil.AddFinalizer(job, dataProtectionFinalizerName)

	scheme, _ := dataprotectionv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(backup, job, scheme); err != nil {
		return err
	}

	reqCtx.Log.Info("create a built-in job from backup", "job", job)
	if err := r.Client.Create(reqCtx.Ctx, job); err != nil {
		return err
	}
	return nil
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

	if err := r.Client.List(reqCtx.Ctx, jobs,
		client.InNamespace(reqCtx.Req.Namespace),
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

		// delete pod when job deleting.
		// ref: https://kubernetes.io/blog/2021/05/14/using-finalizers-to-control-deletion/
		deletePropagation := metav1.DeletePropagationBackground
		deleteOptions := &client.DeleteOptions{
			PropagationPolicy: &deletePropagation,
		}
		if err := r.Client.Delete(reqCtx.Ctx, &job, deleteOptions); err != nil {
			// failed delete k8s job, return error info.
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
		deletePropagation := metav1.DeletePropagationBackground
		deleteOptions := &client.DeleteOptions{
			PropagationPolicy: &deletePropagation,
		}
		if err := r.Client.Delete(reqCtx.Ctx, &i, deleteOptions); err != nil {
			// failed delete k8s job, return error info.
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

func (r *BackupReconciler) GetTargetCluster(
	reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) (*appv1.StatefulSet, error) {
	// get stateful service
	reqCtx.Log.Info("Get cluster from label", "label", backupPolicy.Spec.Target.LabelsSelector.MatchLabels)
	clusterTarget := &appv1.StatefulSetList{}
	if err := r.Client.List(reqCtx.Ctx, clusterTarget,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(backupPolicy.Spec.Target.LabelsSelector.MatchLabels)); err != nil {
		return nil, err
	}
	reqCtx.Log.Info("Get cluster target finish", "target", clusterTarget)
	clusterItemsLen := len(clusterTarget.Items)
	if clusterItemsLen != 1 {
		if clusterItemsLen <= 0 {
			return nil, errors.New("can not found any stateful sets by labelsSelector")
		}
		return nil, errors.New("match labels result more than one, check labelsSelector")
	}
	return &clusterTarget.Items[0], nil
}

func (r *BackupReconciler) GetTargetClusterPod(
	reqCtx intctrlutil.RequestCtx, clusterStatefulSet *appv1.StatefulSet) (*corev1.Pod, error) {
	// get stateful service
	clusterPod := &corev1.Pod{}
	if err := r.Client.Get(reqCtx.Ctx,
		types.NamespacedName{
			Namespace: clusterStatefulSet.Namespace,
			// TODO(dsj): dependency ConsensusSet defined "follower" label to filter
			// Temporary get first pod to build backup volume info
			Name: clusterStatefulSet.Name + "-0",
		}, clusterPod); err != nil {
		return nil, err
	}
	reqCtx.Log.Info("Get cluster pod finish", "target pod", clusterPod)
	return clusterPod, nil
}

func (r *BackupReconciler) BuildBackupToolPodSpec(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.Backup) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	logger := reqCtx.Log

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Spec.BackupPolicyName,
	}

	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		logger.Error(err, "Unable to get backupPolicy for backup.", "backupPolicy", backupPolicyNameSpaceName)
		return podSpec, err
	}

	// get backup tool
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	backupToolNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupPolicy.Spec.BackupToolName,
	}
	if err := r.Client.Get(reqCtx.Ctx, backupToolNameSpaceName, backupTool); err != nil {
		logger.Error(err, "Unable to get backupTool for backup.", "BackupTool", backupToolNameSpaceName)
		return podSpec, err
	}

	// build env value for access target cluster
	clusterStatefulset, err := r.GetTargetCluster(reqCtx, backupPolicy)
	if err != nil {
		return podSpec, err
	}

	clusterPod, err := r.GetTargetClusterPod(reqCtx, clusterStatefulset)
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

	envDBUser := corev1.EnvVar{
		Name: "DB_USER",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: backupPolicy.Spec.Target.Secret.Name,
				},
				Key: backupPolicy.Spec.Target.Secret.UserKeyword,
			},
		},
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
	remoteVolumeMount := corev1.VolumeMount{
		Name:      backupPolicy.Spec.RemoteVolume.Name,
		MountPath: remoteBackupPath,
	}
	container.VolumeMounts = clusterPod.Spec.Containers[0].VolumeMounts
	container.VolumeMounts = append(container.VolumeMounts, remoteVolumeMount)
	allowPrivilegeEscalation := false
	runAsUser := int64(0)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                &runAsUser}

	envDBPassword := corev1.EnvVar{
		Name: "DB_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: backupPolicy.Spec.Target.Secret.Name,
				},
				Key: backupPolicy.Spec.Target.Secret.PasswordKeyword,
			},
		},
	}

	envBackupName := corev1.EnvVar{
		Name:  "BACKUP_NAME",
		Value: backup.Name,
	}

	envBackupDirPrefix := corev1.EnvVar{
		Name: "BACKUP_DIR_PREFIX",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}

	envBackupDir := corev1.EnvVar{
		Name:  "BACKUP_DIR",
		Value: remoteBackupPath + "/$(BACKUP_DIR_PREFIX)",
	}

	container.Env = []corev1.EnvVar{envDBHost, envDBUser, envDBPassword, envBackupName, envBackupDirPrefix, envBackupDir}
	// merge env from backup tool.
	container.Env = append(container.Env, backupTool.Spec.Env...)

	podSpec.Containers = []corev1.Container{container}

	podSpec.Volumes = clusterPod.Spec.Volumes
	podSpec.Volumes = append(podSpec.Volumes, backupPolicy.Spec.RemoteVolume)
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	return podSpec, nil
}

func (r *BackupReconciler) BuildSnapshotPodSpec(
	reqCtx intctrlutil.RequestCtx,
	backup *dataprotectionv1alpha1.Backup,
	preCommand bool) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	logger := reqCtx.Log

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Spec.BackupPolicyName,
	}

	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		logger.Error(err, "Unable to get backupPolicy for backup.", "backupPolicy", backupPolicyNameSpaceName)
		return podSpec, err
	}

	// build env value for access target cluster
	clusterStatefulset, err := r.GetTargetCluster(reqCtx, backupPolicy)
	if err != nil {
		return podSpec, err
	}

	clusterPod, err := r.GetTargetClusterPod(reqCtx, clusterStatefulset)
	if err != nil {
		return podSpec, err
	}

	container := corev1.Container{}
	container.Name = backup.Name
	container.Command = []string{"kubectl", "exec", "-i", clusterPod.Name, "-c", backupPolicy.Spec.Hooks.ContainerName, "--", "sh", "-c"}
	if preCommand {
		container.Args = backupPolicy.Spec.Hooks.PreCommands
	} else {
		container.Args = backupPolicy.Spec.Hooks.PostCommands
	}
	container.Image = backupPolicy.Spec.Hooks.Image
	container.VolumeMounts = clusterPod.Spec.Containers[0].VolumeMounts
	allowPrivilegeEscalation := false
	runAsUser := int64(0)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                &runAsUser}
	// container.Env = backupTool.Spec.Env

	podSpec.Containers = []corev1.Container{container}

	podSpec.Volumes = clusterPod.Spec.Volumes
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.ServiceAccountName = "kubeblocks"

	return podSpec, nil
}
