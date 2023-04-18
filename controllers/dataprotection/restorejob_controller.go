/*
Copyright ApeCloud, Inc.

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
	"fmt"

	"github.com/spf13/viper"
	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// RestoreJobReconciler reconciles a RestoreJob object
type RestoreJobReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	clock    clock.RealClock
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restorejobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restorejobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=restorejobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RestoreJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *RestoreJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// NOTES:
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("restoreJob", req.NamespacedName),
		Recorder: r.Recorder,
	}
	restoreJob := &dataprotectionv1alpha1.RestoreJob{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, restoreJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	reqCtx.Log.Info("in RestoreJob Reconciler: name: " + restoreJob.Name + " phase: " + string(restoreJob.Status.Phase))

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, restoreJob, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, restoreJob)
	})
	if res != nil {
		return *res, err
	}

	// restore job reconcile logic here
	switch restoreJob.Status.Phase {
	case "", dataprotectionv1alpha1.RestoreJobNew:
		return r.doRestoreNewPhaseAction(reqCtx, restoreJob)
	case dataprotectionv1alpha1.RestoreJobInProgressPhy:
		return r.doRestoreInProgressPhyAction(reqCtx, restoreJob)
	default:
		return intctrlutil.Reconciled()
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.RestoreJob{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurDataProtectionReconKey),
		}).
		Complete(r)
}

func (r *RestoreJobReconciler) doRestoreNewPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	restoreJob *dataprotectionv1alpha1.RestoreJob) (ctrl.Result, error) {

	// 1. get stateful service and
	// 2. set stateful set replicate 0
	patch := []byte(`{"spec":{"replicas":0}}`)
	if err := r.patchTargetCluster(reqCtx, restoreJob, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// get backup tool
	// get backup job
	// build a job pod sec
	jobPodSpec, err := r.buildPodSpec(reqCtx, restoreJob)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: restoreJob.Namespace,
			Name:      restoreJob.Name,
			Labels:    buildRestoreJobLabels(restoreJob.Name),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: restoreJob.Namespace,
					Name:      restoreJob.Name},
				Spec: jobPodSpec,
			},
		},
	}
	reqCtx.Log.Info("create a built-in job from restoreJob", "job", job)

	if err := r.Client.Create(reqCtx.Ctx, job); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// update Phase to InProgress
	restoreJob.Status.Phase = dataprotectionv1alpha1.RestoreJobInProgressPhy
	restoreJob.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
	if err := r.Client.Status().Update(reqCtx.Ctx, restoreJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RestoreJobReconciler) doRestoreInProgressPhyAction(
	reqCtx intctrlutil.RequestCtx,
	restoreJob *dataprotectionv1alpha1.RestoreJob) (ctrl.Result, error) {
	job, err := r.getBatchV1Job(reqCtx, restoreJob)
	if err != nil {
		// not found backup job, retry create job
		reqCtx.Log.Info(err.Error())
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	jobStatusConditions := job.Status.Conditions
	if len(jobStatusConditions) == 0 {
		return intctrlutil.RequeueAfter(reconcileInterval, reqCtx.Log, "")
	}

	switch jobStatusConditions[0].Type {
	case batchv1.JobComplete:
		// update Phase to in Completed
		restoreJob.Status.Phase = dataprotectionv1alpha1.RestoreJobCompleted
		restoreJob.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now().UTC()}
		// get stateful service and
		// set stateful set replicate to 1
		patch := []byte(`{"spec":{"replicas":1}}`)
		if err := r.patchTargetCluster(reqCtx, restoreJob, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	case batchv1.JobFailed:
		restoreJob.Status.Phase = dataprotectionv1alpha1.RestoreJobFailed
		restoreJob.Status.FailureReason = job.Status.Conditions[0].Reason
	}
	if err := r.Client.Status().Update(reqCtx.Ctx, restoreJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *RestoreJobReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, restoreJob *dataprotectionv1alpha1.RestoreJob) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	// delete k8s job.
	job, err := r.getBatchV1Job(reqCtx, restoreJob)
	if err != nil {
		// not found backup job, do nothing
		reqCtx.Log.Info(err.Error())
		return nil
	}

	if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, job); err != nil {
		return err
	}
	return nil
}

func (r *RestoreJobReconciler) getBatchV1Job(reqCtx intctrlutil.RequestCtx, backup *dataprotectionv1alpha1.RestoreJob) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	jobNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Name,
	}
	if err := r.Client.Get(reqCtx.Ctx, jobNameSpaceName, job); err != nil {
		// not found backup job, do nothing
		reqCtx.Log.Info(err.Error())
		return nil, err
	}
	return job, nil
}

func (r *RestoreJobReconciler) buildPodSpec(reqCtx intctrlutil.RequestCtx, restoreJob *dataprotectionv1alpha1.RestoreJob) (corev1.PodSpec, error) {
	var podSpec corev1.PodSpec
	logger := reqCtx.Log

	// get backup job
	backup := &dataprotectionv1alpha1.Backup{}
	backupNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      restoreJob.Spec.BackupJobName,
	}
	if err := r.Get(reqCtx.Ctx, backupNameSpaceName, backup); err != nil {
		logger.Error(err, "Unable to get backup for restore.", "backup", backupNameSpaceName)
		return podSpec, err
	}

	// get backup tool
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	backupToolNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backup.Status.BackupToolName,
	}
	if err := r.Client.Get(reqCtx.Ctx, backupToolNameSpaceName, backupTool); err != nil {
		logger.Error(err, "Unable to get backupTool for backup.", "BackupTool", backupToolNameSpaceName)
		return podSpec, err
	}

	if len(backup.Status.PersistentVolumeClaimName) == 0 {
		return podSpec, nil
	}

	container := corev1.Container{}
	container.Name = restoreJob.Name
	container.Command = []string{"sh", "-c"}
	container.Args = backupTool.Spec.Physical.RestoreCommands
	container.Image = backupTool.Spec.Image
	if backupTool.Spec.Resources != nil {
		container.Resources = *backupTool.Spec.Resources
	}

	container.VolumeMounts = restoreJob.Spec.TargetVolumeMounts

	// add the volumeMounts with backup volume
	restoreVolumeName := fmt.Sprintf("restore-%s", backup.Status.PersistentVolumeClaimName)
	remoteVolume := corev1.Volume{
		Name: restoreVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: backup.Status.PersistentVolumeClaimName,
			},
		},
	}
	// add remote volumeMounts
	remoteVolumeMount := corev1.VolumeMount{}
	remoteVolumeMount.Name = restoreVolumeName
	remoteVolumeMount.MountPath = "/data"
	container.VolumeMounts = append(container.VolumeMounts, remoteVolumeMount)

	allowPrivilegeEscalation := false
	runAsUser := int64(0)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                &runAsUser}

	// build env for restore
	envBackupName := corev1.EnvVar{
		Name:  "BACKUP_NAME",
		Value: backup.Name,
	}

	container.Env = []corev1.EnvVar{envBackupName}
	// merge env from backup tool.
	container.Env = append(container.Env, backupTool.Spec.Env...)

	podSpec.Containers = []corev1.Container{container}

	podSpec.Volumes = restoreJob.Spec.TargetVolumes

	// add remote volumes
	podSpec.Volumes = append(podSpec.Volumes, remoteVolume)

	// TODO(dsj): mount readonly remote volumes for restore.
	// podSpec.Volumes[0].PersistentVolumeClaim.ReadOnly = true
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	return podSpec, nil
}

func (r *RestoreJobReconciler) patchTargetCluster(reqCtx intctrlutil.RequestCtx, restoreJob *dataprotectionv1alpha1.RestoreJob, patch []byte) error {
	// get stateful service
	clusterTarget := &appv1.StatefulSetList{}
	if err := r.Client.List(reqCtx.Ctx, clusterTarget,
		client.InNamespace(reqCtx.Req.Namespace),
		client.MatchingLabels(restoreJob.Spec.Target.LabelsSelector.MatchLabels)); err != nil {
		return err
	}
	reqCtx.Log.Info("Get cluster target finish", "target", clusterTarget)
	clusterItemsLen := len(clusterTarget.Items)
	if clusterItemsLen != 1 {
		if clusterItemsLen <= 0 {
			restoreJob.Status.FailureReason = "Can not found any stateful sets by labelsSelector."
		} else {
			restoreJob.Status.FailureReason = "Match labels result more than one, check labelsSelector."
		}
		restoreJob.Status.Phase = dataprotectionv1alpha1.RestoreJobFailed
		reqCtx.Log.Info(restoreJob.Status.FailureReason)
		if err := r.Client.Status().Update(reqCtx.Ctx, restoreJob); err != nil {
			return err
		}
		return nil
	}
	// patch stateful set
	if err := r.Client.Patch(reqCtx.Ctx, &clusterTarget.Items[0], client.RawPatch(types.StrategicMergePatchType, patch)); err != nil {
		return err
	}
	return nil
}

func buildRestoreJobLabels(jobName string) map[string]string {
	return map[string]string{
		dataProtectionLabelRestoreJobNameKey: jobName,
		constant.AppManagedByLabelKey:        constant.AppName,
	}
}
