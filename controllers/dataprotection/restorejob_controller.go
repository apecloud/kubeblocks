/*
Copyright 2022 The Kubeblocks Authors

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
	"time"

	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

// RestoreJobReconciler reconciles a RestoreJob object
type RestoreJobReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	clock    clock.RealClock
}

//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=restorejobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=restorejobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=restorejobs/finalizers,verbs=update

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
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("restoreJob", req.NamespacedName),
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
	if err != nil {
		return *res, err
	}

	// restore job reconcile logic here
	if restoreJob.Status.Phase == "" || restoreJob.Status.Phase == dataprotectionv1alpha1.RestoreJobNew {
		// 1. get stateful service and
		// 2. set stateful set replicate 0
		patch := []byte(`{"spec":{"replicas":0}}`)
		if err := r.PatchTargetCluster(reqCtx, restoreJob, patch); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		// get backup tool
		// get backup job
		// build a job pod sec
		jobPodSpec, err := r.GetPodSpec(reqCtx, restoreJob)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		job := &batchv1.Job{
			//TypeMeta:   metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: restoreJob.Namespace,
				Name:      restoreJob.Name,
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

		if err := r.Client.Create(ctx, job); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		// update Phase to InProgress
		restoreJob.Status.Phase = dataprotectionv1alpha1.RestoreJobInProgressPhy
		restoreJob.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now()}
		if err := r.Client.Status().Update(ctx, restoreJob); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.RequeueAfter(5*time.Second, reqCtx.Log, "")
	}
	if restoreJob.Status.Phase == dataprotectionv1alpha1.RestoreJobInProgressPhy {
		job, err := r.GetBatchV1Job(reqCtx, restoreJob)
		if err != nil {
			// not found backup job, retry create job
			reqCtx.Log.Info(err.Error())
			restoreJob.Status.Phase = dataprotectionv1alpha1.RestoreJobNew
		} else {
			jobStatusConditions := job.Status.Conditions
			if len(jobStatusConditions) > 0 {
				if jobStatusConditions[0].Type == batchv1.JobComplete {
					// update Phase to in Completed
					restoreJob.Status.Phase = dataprotectionv1alpha1.RestoreJobCompleted
					restoreJob.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now()}
					// get stateful service and
					// set stateful set replicate 1
					patch := []byte(`{"spec":{"replicas":1}}`)
					if err := r.PatchTargetCluster(reqCtx, restoreJob, patch); err != nil {
						return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
					}
				} else if jobStatusConditions[0].Type == batchv1.JobFailed {
					restoreJob.Status.Phase = dataprotectionv1alpha1.RestoreJobFailed
					restoreJob.Status.FailureReason = job.Status.Conditions[0].Reason
				}
			}
		}
		// reconcile until status is completed or failed
		if restoreJob.Status.Phase == dataprotectionv1alpha1.RestoreJobInProgressPhy ||
			restoreJob.Status.Phase == dataprotectionv1alpha1.RestoreJobNew {
			return intctrlutil.RequeueAfter(5*time.Second, reqCtx.Log, "")
		}
		if err := r.Client.Status().Update(ctx, restoreJob); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.RestoreJob{}).
		Complete(r)
}

func (r *RestoreJobReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, restoreJob *dataprotectionv1alpha1.RestoreJob) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	// delete k8s job.
	job, err := r.GetBatchV1Job(reqCtx, restoreJob)
	if err != nil {
		// not found backup job, do nothing
		reqCtx.Log.Info(err.Error())
		return nil
	}

	// delete pod when job deleting.
	// ref: https://kubernetes.io/blog/2021/05/14/using-finalizers-to-control-deletion/
	deletePropagation := metav1.DeletePropagationBackground
	deleteOptions := &client.DeleteOptions{
		PropagationPolicy: &deletePropagation,
	}
	if err := r.Client.Delete(reqCtx.Ctx, job, deleteOptions); err != nil {
		// failed delete k8s job, return error info.
		return err
	}
	return nil
}

func (r *RestoreJobReconciler) GetBatchV1Job(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.RestoreJob) (*batchv1.Job, error) {
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

func (r *RestoreJobReconciler) GetPodSpec(reqCtx intctrlutil.RequestCtx, restoreJob *dataprotectionv1alpha1.RestoreJob) (corev1.PodSpec, error) {
	var podSpec corev1.PodSpec
	logger := reqCtx.Log

	// get backup job
	backupJob := &dataprotectionv1alpha1.BackupJob{}
	backupJobNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      restoreJob.Spec.BackupJobName,
	}
	if err := r.Get(reqCtx.Ctx, backupJobNameSpaceName, backupJob); err != nil {
		logger.Error(err, "Unable to get backupJob for restore.", "backupJob", backupJobNameSpaceName)
		return podSpec, err
	}

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupJob.Spec.BackupPolicyName,
	}
	if err := r.Client.Get(reqCtx.Ctx, backupPolicyNameSpaceName, backupPolicy); err != nil {
		logger.Error(err, "Unable to get backupPolicy for backupJob.", "BackupPolicy", backupPolicyNameSpaceName)
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

	container := corev1.Container{}
	container.Name = restoreJob.Name
	container.Command = []string{"sh", "-c"}
	container.Args = backupTool.Spec.Physical.RestoreCommands
	container.Image = backupTool.Spec.Image
	container.Resources = backupTool.Spec.Resources

	container.VolumeMounts = restoreJob.Spec.TargetVolumeMounts

	// add remote volumeMounts
	remoteVolumeMount := corev1.VolumeMount{}
	remoteVolumeMount.Name = backupPolicy.Spec.RemoteVolume.Name
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
		Value: backupJob.Name,
	}

	container.Env = []corev1.EnvVar{envBackupName}
	// merge env from backup tool.
	container.Env = append(container.Env, backupTool.Spec.Env...)

	podSpec.Containers = []corev1.Container{container}

	podSpec.Volumes = restoreJob.Spec.TargetVolumes

	// add remote volumes
	podSpec.Volumes = append(podSpec.Volumes, backupPolicy.Spec.RemoteVolume)

	// TODO(dsj): mount readonly remote volumes for restore.
	// podSpec.Volumes[0].PersistentVolumeClaim.ReadOnly = true
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	return podSpec, nil
}

func (r *RestoreJobReconciler) PatchTargetCluster(reqCtx intctrlutil.RequestCtx, restoreJob *dataprotectionv1alpha1.RestoreJob, patch []byte) error {
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
