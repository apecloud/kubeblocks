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

// BackupJobReconciler reconciles a BackupJob object
type BackupJobReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
	clock    clock.RealClock
}

//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backupjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backupjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backupjobs/finalizers,verbs=update

//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots/finalizers,verbs=update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BackupJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *BackupJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// NOTES:
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backupJob", req.NamespacedName),
		Recorder: r.Recorder,
	}
	// 1. Get backupjob obj
	// 2. if not found, get batchv1 job obj
	// 3. if not found, get volumesnapshot obj

	backupJob := &dataprotectionv1alpha1.BackupJob{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	reqCtx.Log.Info("in BackupJob Reconciler: name: " + backupJob.Name + " phase: " + string(backupJob.Status.Phase))

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupJob, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backupJob)
	})
	if res != nil {
		return *res, err
	}

	// backup job reconcile logic here
	switch backupJob.Status.Phase {
	case "", dataprotectionv1alpha1.BackupJobNew:
		return r.doNewPhaseAction(reqCtx, backupJob)
	case dataprotectionv1alpha1.BackupJobInProgress:
		return r.doInProgressPhaseAction(reqCtx, backupJob)
	default:
		return intctrlutil.Reconciled()
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {

	b := ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.BackupJob{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurDataProtectionReconKey),
		}).
		Owns(&batchv1.Job{})

	if !viper.GetBool("NO_VOLUMESNAPSHOT") {
		b.Owns(&snapshotv1.VolumeSnapshot{}, builder.OnlyMetadata, builder.Predicates{})
	}

	return b.Complete(r)
}

func (r *BackupJobReconciler) doNewPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) (ctrl.Result, error) {

	// HACK/TODO: ought to move following check to validation webhook
	if backupJob.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot && viper.GetBool("NO_VOLUMESNAPSHOT") {
		backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobFailed
		backupJob.Status.FailureReason = "VolumeSnapshot feature disabled."
		if err := r.Client.Status().Update(reqCtx.Ctx, backupJob); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	// update labels
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupJob.Spec.BackupPolicyName,
	}
	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		r.Recorder.Eventf(backupJob, corev1.EventTypeWarning, "CreatingBackupJob",
			"Unable to get backupPolicy for backupJob %s.", backupPolicyNameSpaceName)
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	labels := backupPolicy.Spec.Target.LabelsSelector.MatchLabels
	labels[dataProtectionLabelBackupTypeKey] = string(backupJob.Spec.BackupType)
	if err := r.patchBackupJobLabels(reqCtx, backupJob, labels); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// update Phase to InProgress
	backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobInProgress
	backupJob.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now()}
	if err := r.Client.Status().Update(reqCtx.Ctx, backupJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
}

func (r *BackupJobReconciler) doInProgressPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) (ctrl.Result, error) {

	if backupJob.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		// 1. create and ensure pre-command job completed
		// 2. create and ensure volume snapshot ready
		// 3. create and ensure post-command job completed
		isOK, err := r.createPreCommandJobAndEnsure(reqCtx, backupJob)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		if err = r.createVolumeSnapshot(reqCtx, backupJob); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backupJob.Name}
		isOK, err = r.ensureVolumeSnapshotReady(reqCtx, key)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		msg := fmt.Sprintf("Created volumeSnapshot %s ready.", key.Name)
		r.Recorder.Event(backupJob, corev1.EventTypeNormal, "CreatedVolumeSnapshot", msg)

		isOK, err = r.createPostCommandJobAndEnsure(reqCtx, backupJob)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}

		backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobCompleted
		backupJob.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now()}
	} else {
		// 1. create and ensure backup tool job finished
		// 2. get job phase and update
		err := r.createBackupToolJob(reqCtx, backupJob)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		key := types.NamespacedName{Namespace: backupJob.Namespace, Name: backupJob.Name}
		isOK, err := r.ensureBatchV1JobCompleted(reqCtx, key)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
		}
		job, err := r.getBatchV1Job(reqCtx, backupJob)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		jobStatusConditions := job.Status.Conditions
		if jobStatusConditions[0].Type == batchv1.JobComplete {
			// update Phase to in Completed
			backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobCompleted
			backupJob.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now()}
		} else if jobStatusConditions[0].Type == batchv1.JobFailed {
			backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobFailed
			backupJob.Status.FailureReason = job.Status.Conditions[0].Reason
		}
	}

	// finally, update backupJob status
	r.Recorder.Event(backupJob, corev1.EventTypeNormal, "CreatedBackupJob", "Completed backupJob.")
	if err := r.Client.Status().Update(reqCtx.Ctx, backupJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

// patchBackupJobLabels patch backupJob labels
func (r *BackupJobReconciler) patchBackupJobLabels(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob,
	labels map[string]string) error {

	patch := client.MergeFrom(backupJob.DeepCopy())
	if len(labels) > 0 {
		if backupJob.Labels == nil {
			backupJob.Labels = labels
		} else {
			for k, v := range labels {
				backupJob.Labels[k] = v
			}
		}
	}
	return r.Client.Patch(reqCtx.Ctx, backupJob, patch)
}

func (r *BackupJobReconciler) createPreCommandJobAndEnsure(reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) (bool, error) {

	emptyCmd, err := r.ensureEmptyHooksCommand(reqCtx, backupJob, true)
	if err != nil {
		return false, err
	}
	// if not defined commands, skip create job.
	if emptyCmd {
		return true, err
	}

	key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backupJob.Name + "-pre"}
	if err := r.createHooksCommandJob(reqCtx, backupJob, key, true); err != nil {
		return false, err
	}
	return r.ensureBatchV1JobCompleted(reqCtx, key)
}

func (r *BackupJobReconciler) createPostCommandJobAndEnsure(reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) (bool, error) {

	emptyCmd, err := r.ensureEmptyHooksCommand(reqCtx, backupJob, false)
	if err != nil {
		return false, err
	}
	// if not defined commands, skip create job.
	if emptyCmd {
		return true, err
	}

	key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backupJob.Name + "-post"}
	if err := r.createHooksCommandJob(reqCtx, backupJob, key, false); err != nil {
		return false, err
	}
	return r.ensureBatchV1JobCompleted(reqCtx, key)
}

func (r *BackupJobReconciler) ensureBatchV1JobCompleted(
	reqCtx intctrlutil.RequestCtx, key types.NamespacedName) (bool, error) {
	job := &batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, job)
	if err != nil {
		return false, err
	}
	if exists {
		jobStatusConditions := job.Status.Conditions
		if len(jobStatusConditions) > 0 {
			if jobStatusConditions[0].Type == batchv1.JobComplete ||
				jobStatusConditions[0].Type == batchv1.JobFailed {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *BackupJobReconciler) createVolumeSnapshot(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) error {

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
		Name:      backupJob.Spec.BackupPolicyName,
	}

	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		reqCtx.Log.Error(err, "Unable to get backupPolicy for backupJob.", "backupPolicy", backupPolicyNameSpaceName)
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
			Labels:    buildBackupJobLabels(backupJob.Name),
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvcName,
			},
		},
	}

	controllerutil.AddFinalizer(snap, dataProtectionFinalizerName)

	scheme, _ := dataprotectionv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(backupJob, snap, scheme); err != nil {
		return err
	}

	reqCtx.Log.Info("create a volumeSnapshot from backupJob", "snapshot", snap)
	if err := r.Client.Create(reqCtx.Ctx, snap); err != nil {
		return err
	}
	msg := fmt.Sprintf("Waiting for a volume snapshot %s to be created by the backupJob.", snap.Name)
	r.Recorder.Event(backupJob, corev1.EventTypeNormal, "CreatingVolumeSnapshot", msg)
	return nil
}

func (r *BackupJobReconciler) ensureVolumeSnapshotReady(reqCtx intctrlutil.RequestCtx,
	key types.NamespacedName) (bool, error) {

	snap := &snapshotv1.VolumeSnapshot{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, snap)
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

func (r *BackupJobReconciler) createBackupToolJob(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) error {

	key := types.NamespacedName{Namespace: backupJob.Namespace, Name: backupJob.Name}
	job := batchv1.Job{}
	exists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, key, &job)
	if err != nil {
		return err
	}
	if exists {
		// find resource object, skip created.
		return nil
	}

	toolPodSpec, err := r.BuildBackupToolPodSpec(reqCtx, backupJob)
	if err != nil {
		return err
	}

	if err = r.CreateBatchV1Job(reqCtx, key, backupJob, toolPodSpec); err != nil {
		return err
	}
	msg := fmt.Sprintf("Waiting for a job %s to be created.", key.Name)
	r.Recorder.Event(backupJob, corev1.EventTypeNormal, "CreatingJob", msg)
	return nil
}

func (r *BackupJobReconciler) ensureEmptyHooksCommand(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob,
	preCommand bool) (bool, error) {

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyKey := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupJob.Spec.BackupPolicyName,
	}

	policyExists, err := intctrlutil.CheckResourceExists(reqCtx.Ctx, r.Client, backupPolicyKey, backupPolicy)
	if err != nil {
		msg := fmt.Sprintf("Failed to get backupPolicy %s .", backupPolicyKey.Name)
		r.Recorder.Event(backupJob, corev1.EventTypeWarning, "BackupPolicyFailed", msg)
		return false, err
	}

	if !policyExists {
		msg := fmt.Sprintf("Not Found backupPolicy %s .", backupPolicyKey.Name)
		r.Recorder.Event(backupJob, corev1.EventTypeWarning, "BackupPolicyFailed", msg)
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

func (r *BackupJobReconciler) createHooksCommandJob(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob,
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

	jobPodSpec, err := r.BuildSnapshotPodSpec(reqCtx, backupJob, preCommand)
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("Waiting for a job %s to be created.", key.Name)
	r.Recorder.Event(backupJob, corev1.EventTypeNormal, "CreatingJob-"+key.Name, msg)

	return r.CreateBatchV1Job(reqCtx, key, backupJob, jobPodSpec)
}

func buildBackupJobLabels(backupJobName string) map[string]string {
	return map[string]string{
		"backupjobs.dataprotection.kubeblocks.io/name": backupJobName,
	}
}

func (r *BackupJobReconciler) CreateBatchV1Job(
	reqCtx intctrlutil.RequestCtx,
	key types.NamespacedName,
	backupJob *dataprotectionv1alpha1.BackupJob,
	templatePodSpec corev1.PodSpec) error {

	job := &batchv1.Job{
		//TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    buildBackupJobLabels(backupJob.Name),
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
	if err := controllerutil.SetOwnerReference(backupJob, job, scheme); err != nil {
		return err
	}

	reqCtx.Log.Info("create a built-in job from backupJob", "job", job)
	if err := r.Client.Create(reqCtx.Ctx, job); err != nil {
		return err
	}
	return nil
}

func (r *BackupJobReconciler) getBatchV1Job(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.BackupJob) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	jobNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupJob.Name,
	}
	if err := r.Client.Get(reqCtx.Ctx, jobNameSpaceName, job); err != nil {
		// not found backup job, do nothing
		reqCtx.Log.Info(err.Error())
		return nil, err
	}
	return job, nil
}

func (r *BackupJobReconciler) deleteReferenceBatchV1Jobs(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.BackupJob) error {
	jobs := &batchv1.JobList{}

	if err := r.Client.List(reqCtx.Ctx, jobs,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(buildBackupJobLabels(backupJob.Name))); err != nil {
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

func (r *BackupJobReconciler) deleteReferenceVolumeSnapshot(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.BackupJob) error {
	snaps := &snapshotv1.VolumeSnapshotList{}

	if err := r.Client.List(reqCtx.Ctx, snaps,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(buildBackupJobLabels(backupJob.Name))); err != nil {
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

func (r *BackupJobReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.BackupJob) error {
	if err := r.deleteReferenceBatchV1Jobs(reqCtx, backupJob); err != nil {
		return err
	}
	if err := r.deleteReferenceVolumeSnapshot(reqCtx, backupJob); err != nil {
		return err
	}
	return nil
}

func (r *BackupJobReconciler) GetTargetCluster(
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

func (r *BackupJobReconciler) GetTargetClusterPod(
	reqCtx intctrlutil.RequestCtx, clusterStatefulSet *appv1.StatefulSet) (*corev1.Pod, error) {
	// get stateful service
	clusterPod := &corev1.Pod{}
	if err := r.Client.Get(reqCtx.Ctx,
		types.NamespacedName{
			Namespace: clusterStatefulSet.Namespace,
			// TODO(dsj): dependency ConsensusSet defined "follower" label to filter
			// Temporary get first pod to build backup job volume info
			Name: clusterStatefulSet.Name + "-0",
		}, clusterPod); err != nil {
		return nil, err
	}
	reqCtx.Log.Info("Get cluster pod finish", "target pod", clusterPod)
	return clusterPod, nil
}

func (r *BackupJobReconciler) BuildBackupToolPodSpec(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.BackupJob) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	logger := reqCtx.Log

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupJob.Spec.BackupPolicyName,
	}

	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		logger.Error(err, "Unable to get backupPolicy for backupJob.", "backupPolicy", backupPolicyNameSpaceName)
		return podSpec, err
	}

	// get backup tool
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	backupToolNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupPolicy.Spec.BackupToolName,
	}
	if err := r.Client.Get(reqCtx.Ctx, backupToolNameSpaceName, backupTool); err != nil {
		logger.Error(err, "Unable to get backupTool for backupJob.", "BackupTool", backupToolNameSpaceName)
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

	envDBHost := corev1.EnvVar{
		Name:  "DB_HOST",
		Value: clusterPod.Name,
	}

	envDBUser := corev1.EnvVar{
		Name: "DB_USER",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: backupPolicy.Spec.Target.Secret.Name,
				},
				Key: backupPolicy.Spec.Target.Secret.KeyUser,
			},
		},
	}

	container := corev1.Container{}
	container.Name = backupJob.Name
	container.Command = []string{"sh", "-c"}
	container.Args = backupTool.Spec.BackupCommands
	container.Image = backupTool.Spec.Image
	container.Resources = backupTool.Spec.Resources

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
				Key: backupPolicy.Spec.Target.Secret.KeyPassword,
			},
		},
	}

	envBackupName := corev1.EnvVar{
		Name:  "BACKUP_NAME",
		Value: backupJob.Name,
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

func (r *BackupJobReconciler) BuildSnapshotPodSpec(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob,
	preCommand bool) (corev1.PodSpec, error) {
	podSpec := corev1.PodSpec{}
	logger := reqCtx.Log

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupJob.Spec.BackupPolicyName,
	}

	if err := r.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		logger.Error(err, "Unable to get backupPolicy for backupJob.", "backupPolicy", backupPolicyNameSpaceName)
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
	container.Name = backupJob.Name
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
