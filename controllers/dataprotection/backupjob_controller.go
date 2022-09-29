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
	"errors"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const reQueueInterval = time.Second * 5

// BackupJobReconciler reconciles a BackupJob object
type BackupJobReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	clock    clock.RealClock
}

//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=backupjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=backupjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=backupjobs/finalizers,verbs=update

//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

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
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("backupJob", req.NamespacedName),
	}
	backupJob := &dataprotectionv1alpha1.BackupJob{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	reqCtx.Log.Info("in BackupJob Reconciler: name: " + backupJob.Name + " phase: " + string(backupJob.Status.Phase))

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupJob, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backupJob)
	})
	if err != nil {
		return *res, err
	}

	// backup job reconcile logic here
	switch backupJob.Status.Phase {
	case "", dataprotectionv1alpha1.BackupJobNew:
		return r.DoNewPhaseAction(reqCtx, backupJob)
	case dataprotectionv1alpha1.BackupJobInProgress:
		return r.DoInProgressPhaseAction(reqCtx, backupJob)
	default:
		return intctrlutil.Reconciled()
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.BackupJob{}).
		Watches(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForObject{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *BackupJobReconciler) DoNewPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) (ctrl.Result, error) {
	if backupJob.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		// 1. build pre command job
		// 2. after job completed, create volume snapshot
		if err := r.createPreCommandJob(reqCtx, backupJob); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	} else {
		// 1. get backup tool
		// 2. get backup policy
		// 3. build a job pod sec
		jobPodSpec, err := r.GetPodSpec(reqCtx, backupJob)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		// create a new job to back up
		key := types.NamespacedName{
			Namespace: reqCtx.Req.Namespace,
			Name:      backupJob.Name,
		}
		if err := r.CreateBatchV1Job(reqCtx, key, backupJob, jobPodSpec); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

	}
	// update Phase to InProgress
	backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobInProgress
	backupJob.Status.StartTimestamp = &metav1.Time{Time: r.clock.Now()}
	if err := r.Client.Status().Update(reqCtx.Ctx, backupJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.RequeueAfter(reQueueInterval, reqCtx.Log, "")
}

func (r *BackupJobReconciler) ensureBatchV1JobCompleted(
	reqCtx intctrlutil.RequestCtx, key types.NamespacedName) (bool, error) {
	job := &batchv1.Job{}
	if err := r.Client.Get(reqCtx.Ctx, key, job); err != nil {
		return false, err
	}

	jobStatusConditions := job.Status.Conditions
	if len(jobStatusConditions) > 0 {
		if jobStatusConditions[0].Type == batchv1.JobComplete ||
			jobStatusConditions[0].Type == batchv1.JobFailed {
			return true, nil
		}
	}
	return false, nil
}

func (r *BackupJobReconciler) createVolumeSnapshot(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) error {
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
	volumeSnapshotClassName := "csi-aws-vsc"

	snap = &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqCtx.Req.Namespace,
			Name:      reqCtx.Req.Name,
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &volumeSnapshotClassName,
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

	reqCtx.Log.Info("create a volume snapshot from backupJob", "snapshot", snap)
	if err := r.Client.Create(reqCtx.Ctx, snap); err != nil {
		return err
	}
	return nil
}

func (r *BackupJobReconciler) ensureVolumeSnapshotReady(
	reqCtx intctrlutil.RequestCtx,
	key types.NamespacedName) (bool, error) {
	snap := &snapshotv1.VolumeSnapshot{}

	if err := r.Get(reqCtx.Ctx, key, snap); err != nil {
		return false, err
	}
	ready := false

	if snap.Status.ReadyToUse != nil {
		ready = *(snap.Status.ReadyToUse)
	}

	return ready, nil
}

func (r *BackupJobReconciler) createPreCommandJob(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) error {

	// TODO(dsj): get namespace from self operator definition
	key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backupJob.Name + "-pre"}
	job := batchv1.Job{}
	exists, err := checkResourceExists(reqCtx.Ctx, r.Client, key, &job)
	if err != nil {
		return err
	}
	if exists {
		// find resource object, skip created.
		return nil
	}

	jobPodSpec, err := r.BuildSnapshotPodSpec(reqCtx, backupJob, true)
	if err != nil {
		return err
	}

	if err = r.CreateBatchV1Job(reqCtx, key, backupJob, jobPodSpec); err != nil {
		return err
	}

	return nil
}

func (r *BackupJobReconciler) createPostCommandJob(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) error {

	key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backupJob.Name + "-post"}
	job := batchv1.Job{}
	exists, err := checkResourceExists(reqCtx.Ctx, r.Client, key, &job)
	if err != nil {
		return err
	}
	if exists {
		// find resource object, skip created.
		return nil
	}

	jobPodSpec, err := r.BuildSnapshotPodSpec(reqCtx, backupJob, false)
	if err != nil {
		return err
	}
	if err = r.CreateBatchV1Job(reqCtx, key, backupJob, jobPodSpec); err != nil {
		return err
	}

	return nil
}

func (r *BackupJobReconciler) DoInProgressPhaseAction(
	reqCtx intctrlutil.RequestCtx,
	backupJob *dataprotectionv1alpha1.BackupJob) (ctrl.Result, error) {

	if backupJob.Spec.BackupType == dataprotectionv1alpha1.BackupTypeSnapshot {
		// 1. ensure pre-command job completed
		// 2. create volume snapshot cr
		// 3. ensure volume snapshot cr ready
		// 4. create post-command job
		// 5. ensure post-command job completed
		key := types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backupJob.Name + "-pre"}
		isOK, err := r.ensureBatchV1JobCompleted(reqCtx, key)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reQueueInterval, reqCtx.Log, "")
		}
		if err = r.createVolumeSnapshot(reqCtx, backupJob); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		key = types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backupJob.Name}
		isOK, err = r.ensureVolumeSnapshotReady(reqCtx, key)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reQueueInterval, reqCtx.Log, "")
		}

		if err := r.createPostCommandJob(reqCtx, backupJob); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		key = types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: backupJob.Name + "-post"}
		isOK, err = r.ensureBatchV1JobCompleted(reqCtx, key)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		if !isOK {
			return intctrlutil.RequeueAfter(reQueueInterval, reqCtx.Log, "")
		}
		backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobCompleted
		backupJob.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now()}
		if err := r.Client.Status().Update(reqCtx.Ctx, backupJob); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	job, err := r.GetBatchV1Job(reqCtx, backupJob)
	if err != nil {
		// not found backup job, retry create job
		backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobNew
	} else {
		jobStatusConditions := job.Status.Conditions
		if len(jobStatusConditions) > 0 {
			if jobStatusConditions[0].Type == batchv1.JobComplete {
				// update Phase to in Completed
				backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobCompleted
				backupJob.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now()}
			} else if jobStatusConditions[0].Type == batchv1.JobFailed {
				backupJob.Status.Phase = dataprotectionv1alpha1.BackupJobFailed
				backupJob.Status.FailureReason = job.Status.Conditions[0].Reason
			}
		}
	}
	// reconcile until status is completed or failed
	if backupJob.Status.Phase == dataprotectionv1alpha1.BackupJobInProgress ||
		backupJob.Status.Phase == dataprotectionv1alpha1.BackupJobNew {
		return intctrlutil.RequeueAfter(reQueueInterval, reqCtx.Log, "")
	}

	if err := r.Client.Status().Update(reqCtx.Ctx, backupJob); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
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

func (r *BackupJobReconciler) GetBatchV1Job(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.BackupJob) (*batchv1.Job, error) {
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

func (r *BackupJobReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.BackupJob) error {

	// delete k8s job.
	job, err := r.GetBatchV1Job(reqCtx, backupJob)
	if err != nil {
		// not found backup job, do nothing
		reqCtx.Log.Info(err.Error())
		return nil
	}

	if controllerutil.ContainsFinalizer(job, dataProtectionFinalizerName) {
		patch := client.MergeFrom(job.DeepCopy())
		controllerutil.RemoveFinalizer(job, dataProtectionFinalizerName)
		if err := r.Patch(reqCtx.Ctx, job, patch); err != nil {
			return err
		}
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

func (r *BackupJobReconciler) GetPodSpec(reqCtx intctrlutil.RequestCtx, backupJob *dataprotectionv1alpha1.BackupJob) (corev1.PodSpec, error) {
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

	envBackupDir := corev1.EnvVar{
		Name:  "BACKUP_DIR",
		Value: remoteBackupPath,
	}

	container.Env = []corev1.EnvVar{envDBHost, envDBUser, envDBPassword, envBackupName, envBackupDir}
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
	forPreCommand bool) (corev1.PodSpec, error) {
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
	container.Command = []string{"kubectl", "exec", clusterPod.Name, "-c", backupPolicy.Spec.Hooks.ContainerName}
	if forPreCommand {
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
	podSpec.Volumes = append(podSpec.Volumes, backupPolicy.Spec.RemoteVolume)
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.ServiceAccountName = "opendbaas-core"

	return podSpec, nil
}
