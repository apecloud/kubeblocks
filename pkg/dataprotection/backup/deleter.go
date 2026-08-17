/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package backup

import (
	"fmt"
	"path/filepath"
	"strings"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	deleteBackupFilesJobNamePrefix = "delete-"
	deleteContainerName            = "deleter"
)

type DeletionStatus string

const (
	DeletionStatusDeleting  DeletionStatus = "Deleting"
	DeletionStatusFailed    DeletionStatus = "Failed"
	DeletionStatusSucceeded DeletionStatus = "Succeeded"
	DeletionStatusUnknown   DeletionStatus = "Unknown"
)

type Deleter struct {
	ctrlutil.RequestCtx
	Client                     client.Client
	Scheme                     *runtime.Scheme
	WorkerServiceAccount       string
	EnsureWorkerServiceAccount func() (string, error)

	actionSet *dpv1alpha1.ActionSet
}

func (d *Deleter) ensureWorkerServiceAccount() error {
	if d.WorkerServiceAccount != "" {
		return nil
	}
	if d.EnsureWorkerServiceAccount == nil {
		d.WorkerServiceAccount = viper.GetString(dptypes.CfgKeyWorkerServiceAccountName)
		return nil
	}
	saName, err := d.EnsureWorkerServiceAccount()
	if err != nil {
		return err
	}
	d.WorkerServiceAccount = saName
	return nil
}

// DeleteBackupFiles builds a job to delete backup files, and returns the deletion status.
// If the deletion job exists, it will check the job status and return the corresponding
// deletion status.
func (d *Deleter) DeleteBackupFiles(backup *dpv1alpha1.Backup) (DeletionStatus, error) {
	backupMethod := backup.Status.BackupMethod
	if backupMethod != nil && boolptr.IsSetToTrue(backupMethod.SnapshotVolumes) {
		// if the backup is volume snapshot, ignore to delete files
		return DeletionStatusSucceeded, nil
	}
	jobKey := BuildDeleteBackupFilesJobKey(backup, false)
	job := &batchv1.Job{}
	exists, err := ctrlutil.CheckResourceExists(d.Ctx, d.Client, jobKey, job)
	if err != nil {
		return DeletionStatusUnknown, err
	}

	// if deletion job exists, check its status
	if exists {
		_, finishedType, msg := utils.IsJobFinished(job)
		switch finishedType {
		case batchv1.JobComplete:
			return DeletionStatusSucceeded, nil
		case batchv1.JobFailed:
			return DeletionStatusFailed,
				fmt.Errorf("deletion backup files job \"%s\" failed, you can delete it to re-delete the backup files, %s", job.Name, msg)
		}
		return DeletionStatusDeleting, nil
	}

	if backup.Status.BackupRepoName == "" {
		d.Log.Info("skip deleting backup files because BackupRepoName is empty", "backup", backup.Name)
		return DeletionStatusSucceeded, nil
	}
	backupRepo := &dpv1alpha1.BackupRepo{}
	if err = d.Client.Get(d.Ctx, client.ObjectKey{Name: backup.Status.BackupRepoName}, backupRepo); err != nil {
		if apierrors.IsNotFound(err) {
			return DeletionStatusSucceeded, nil
		}
		return DeletionStatusUnknown, err
	}

	backupFilePath := backup.Status.Path
	if backupFilePath == "" || (!strings.Contains(backupFilePath, backup.Name)) {
		// Refuse to delete an empty or overly broad path. Otherwise files belonging to
		// other backups could be deleted as well.
		d.Log.Info("skip deleting backup files because backup file path is invalid",
			"backupFilePath", backupFilePath, "backup", backup.Name)
		return DeletionStatusSucceeded, nil
	}

	// make sure the path has a leading slash
	if !strings.HasPrefix(backupFilePath, "/") {
		backupFilePath = "/" + backupFilePath
	}
	// do pre-delete action
	preDeleteAction, err := d.getPreDeleteAction(backup.Status.BackupMethod)
	if err != nil {
		return DeletionStatusUnknown, err
	}
	if preDeleteAction != nil {
		preJobs, err := d.doPreDeleteActions(backup, backupRepo, preDeleteAction, backupFilePath)
		if err != nil {
			return DeletionStatusUnknown, err
		}
		allCompleted := true
		for _, preJob := range preJobs {
			_, finishedType, msg := utils.IsJobFinished(preJob)
			if finishedType == batchv1.JobFailed {
				return DeletionStatusFailed,
					fmt.Errorf("pre-delete backup files job \"%s\" failed, you can delete it to re-delete the backup files, %s", preJob.Name, msg)
			}
			if finishedType != batchv1.JobComplete {
				allCompleted = false
			}
		}
		if !allCompleted {
			return DeletionStatusDeleting, nil
		}
	}
	// do delete action
	return DeletionStatusDeleting, d.createDeleteBackupFilesJob(jobKey, backup, backupRepo)
}

func (d *Deleter) buildDeleteBackupFilesScript(backupPath string) string {

	// this script first deletes the directory where the backup is located (including files
	// in the directory), and then traverses up the path level by level to clean up empty directories.
	deleteScript := fmt.Sprintf(`
set -x
export PATH="$PATH:$%s"
targetPath="%s"

echo "removing backup files in ${targetPath}"
DATASAFED_KOPIA_MAINTENANCE=true datasafed rm -r "${targetPath}"

# remove empty dirs from leaf to root
function rmdirs() {
	curr="$1"
	while true; do
		curr=$(dirname "${curr}")
		if [ "${curr}" == "/" ]; then
			echo "reach to root, done"
			break
		fi
		result=$(datasafed list "${curr}")
		if [ -z "$result" ]; then
			echo "${curr} is empty, removing it..."
			datasafed rmdir "${curr}"
		else
			echo "${curr} is not empty, done"
			break
		fi
	done
}

if [ "${DATASAFED_KOPIA_REPO_ROOT}" == "" ]; then
	# kopia is not used, simply remove empty dirs from the storage
	rmdirs "${targetPath}"
else
	# remove empty dirs from the kopia repository
	rmdirs "${targetPath}"

	# remove the kopia repository itself from the storage if it's empty
	result=$(datasafed list "/")
	if [ -z "$result" ]; then
		kopiaRepoPath="${DATASAFED_KOPIA_REPO_ROOT}"
		unset DATASAFED_KOPIA_REPO_ROOT
		echo "kopia repository at '${kopiaRepoPath}' is empty, removing it from the storage..."
		datasafed rm -r "${kopiaRepoPath}"
		datasafed rm -r "${kopiaRepoPath}.meta"

		# remove empty dirs from the storage
		rmdirs "${kopiaRepoPath}"
	fi
fi
	`, dptypes.DPDatasafedBinPath, backupPath)

	return deleteScript
}

func (d *Deleter) createDeleteBackupFilesJob(
	jobKey types.NamespacedName,
	backup *dpv1alpha1.Backup,
	backupRepo *dpv1alpha1.BackupRepo) error {

	runAsUser := int64(0)
	container := corev1.Container{
		Name:            deleteContainerName,
		Command:         []string{"sh", "-c"},
		Args:            []string{d.buildDeleteBackupFilesScript(backup.Status.Path)},
		Image:           viper.GetString(constant.KBToolsImage),
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolptr.False(),
			RunAsUser:                &runAsUser,
		},
	}
	return d.createDeleteJob(container, jobKey, backup, backupRepo)
}

func (d *Deleter) createDeleteJob(container corev1.Container,
	jobKey types.NamespacedName,
	backup *dpv1alpha1.Backup,
	backupRepo *dpv1alpha1.BackupRepo) error {
	if err := d.ensureWorkerServiceAccount(); err != nil {
		return err
	}
	ctrlutil.InjectZeroResourcesLimitsForDataProtection(&container)

	// build pod
	podSpec := corev1.PodSpec{
		Containers:         []corev1.Container{container},
		RestartPolicy:      corev1.RestartPolicyNever,
		ServiceAccountName: d.WorkerServiceAccount,
	}
	if err := utils.AddTolerations(&podSpec); err != nil {
		return err
	}
	kopiaRepoPath := backup.Status.KopiaRepoPath
	encryptionConfig := backup.Status.EncryptionConfig
	utils.InjectDatasafed(&podSpec, backupRepo, RepoVolumeMountPath, encryptionConfig, kopiaRepoPath)

	// build job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: jobKey.Namespace,
			Name:      jobKey.Name,
			Labels: map[string]string{
				constant.AppManagedByLabelKey: dptypes.AppName,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: jobKey.Namespace,
					Name:      jobKey.Name,
				},
				Spec: podSpec,
			},
			BackoffLimit: &dptypes.DefaultBackOffLimit,
		},
	}
	if err := utils.SetControllerReference(backup, job, d.Scheme); err != nil {
		return err
	}
	d.Log.V(1).Info("create a job to delete backup files", "job", job)
	return client.IgnoreAlreadyExists(d.Client.Create(d.Ctx, job))
}

func (d *Deleter) getPreDeleteAction(backupMethod *dpv1alpha1.BackupMethod) (*dpv1alpha1.BaseJobActionSpec, error) {
	if backupMethod == nil || backupMethod.ActionSetName == "" {
		return nil, nil
	}
	actionSet, err := utils.GetActionSetByName(d.RequestCtx, d.Client, backupMethod.ActionSetName)
	if err != nil {
		return nil, err
	}
	d.actionSet = actionSet
	return actionSet.Spec.Backup.PreDeleteBackup, nil
}

func (d *Deleter) doPreDeleteActions(
	backup *dpv1alpha1.Backup,
	backupRepo *dpv1alpha1.BackupRepo,
	preDeleteAction *dpv1alpha1.BaseJobActionSpec,
	backupFilePath string) ([]*batchv1.Job, error) {
	if backup.Status.Target != nil || len(backup.Status.Targets) == 0 {
		job, err := d.doPreDeleteAction(backup, backup.Status.Target, BuildDeleteBackupFilesJobKey(backup, true),
			backupRepo, preDeleteAction, backupFilePath)
		if err != nil {
			return nil, err
		}
		return []*batchv1.Job{job}, nil
	}

	jobs := make([]*batchv1.Job, 0, len(backup.Status.Targets))
	for i := range backup.Status.Targets {
		target := &backup.Status.Targets[i]
		jobKey := buildTargetPreDeleteJobKey(backup, target.Name, i)
		targetBackupPath := filepath.Join(backupFilePath, target.Name)
		job, err := d.doPreDeleteAction(backup, target, jobKey, backupRepo,
			preDeleteAction, targetBackupPath)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (d *Deleter) doPreDeleteAction(
	backup *dpv1alpha1.Backup,
	target *dpv1alpha1.BackupStatusTarget,
	preJobKey client.ObjectKey,
	backupRepo *dpv1alpha1.BackupRepo,
	preDeleteAction *dpv1alpha1.BaseJobActionSpec,
	backupFilePath string) (*batchv1.Job, error) {
	preJob := &batchv1.Job{}
	if exists, err := ctrlutil.CheckResourceExists(d.Ctx, d.Client, preJobKey, preJob); err != nil {
		return nil, err
	} else if exists {
		return preJob, nil
	}
	// create pre-delete action
	runAsUser := int64(0)
	envVars := []corev1.EnvVar{
		{Name: dptypes.DPBackupBasePath, Value: backupFilePath},
		{Name: dptypes.DPBackupName, Value: backup.Name},
	}
	connectionEnv, err := d.buildEnvFromTarget(backup, target)
	if err != nil {
		return nil, err
	}
	envVars = append(envVars, connectionEnv...)
	if d.actionSet != nil {
		envVars = append(envVars, d.actionSet.Spec.Env...)
	}
	if backup.Status.BackupMethod != nil {
		envVars = append(envVars, backup.Status.BackupMethod.Env...)
	}
	image := common.Expand(preDeleteAction.Image, common.MappingFuncFor(utils.CovertEnvToMap(envVars)))
	container := corev1.Container{
		Name:            deleteContainerName,
		Command:         preDeleteAction.Command,
		Image:           ctrlutil.ReplaceImageRegistry(image),
		Env:             envVars,
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolptr.False(),
			RunAsUser:                &runAsUser,
		},
	}
	return preJob, d.createDeleteJob(container, preJobKey, backup, backupRepo)
}

func (d *Deleter) buildEnvFromTarget(
	backup *dpv1alpha1.Backup,
	target *dpv1alpha1.BackupStatusTarget) ([]corev1.EnvVar, error) {
	clusterName := backup.Labels[constant.AppInstanceLabelKey]
	clusterUID := backup.Labels[dptypes.ClusterUIDLabelKey]
	if clusterName == "" || clusterUID == "" {
		return nil, nil
	}

	cluster := &appsv1.Cluster{}
	err := d.Client.Get(d.Ctx, client.ObjectKey{Namespace: backup.Namespace, Name: clusterName}, cluster)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if string(cluster.UID) != clusterUID {
		return nil, nil
	}

	if target == nil || target.PodSelector == nil || target.PodSelector.LabelSelector == nil {
		return nil, nil
	}

	targetPod, err := d.selectAvailableTargetPod(backup.Namespace, target.PodSelector)
	if err != nil || targetPod == nil {
		return nil, err
	}
	envVars := targetPod.Spec.Containers[0].Env
	envVarsFromTarget, err := utils.BuildEnvByTarget(targetPod, target.ConnectionCredential, target.ContainerPort)
	if err != nil {
		return nil, err
	}
	return append(envVars, envVarsFromTarget...), nil
}

func (d *Deleter) selectAvailableTargetPod(namespace string, selector *dpv1alpha1.PodSelector) (*corev1.Pod, error) {
	selectPod := func(labelSelector *metav1.LabelSelector) (*corev1.Pod, error) {
		reqCtx := d.RequestCtx
		reqCtx.Req.Namespace = namespace
		pods, err := utils.GetPodListByLabelSelector(reqCtx, d.Client, labelSelector)
		if err != nil {
			return nil, err
		}
		return utils.GetFirstIndexRunningPod(pods), nil
	}

	targetPod, err := selectPod(selector.LabelSelector)
	if err != nil || targetPod != nil || selector.Strategy != dpv1alpha1.PodSelectionStrategyAny || selector.FallbackLabelSelector == nil {
		return targetPod, err
	}
	return selectPod(selector.FallbackLabelSelector)
}

func (d *Deleter) DeleteVolumeSnapshots(backup *dpv1alpha1.Backup) error {
	// initialize volume snapshot client that is compatible with both v1beta1 and v1
	vsCli := utils.NewCompatClient(d.Client)
	snaps := &vsv1.VolumeSnapshotList{}
	if err := vsCli.List(d.Ctx, snaps, client.InNamespace(backup.Namespace),
		client.MatchingLabels(map[string]string{
			dptypes.BackupNameLabelKey: backup.Name,
		})); err != nil {
		return client.IgnoreNotFound(err)
	}

	deleteVolumeSnapshot := func(vs *vsv1.VolumeSnapshot) error {
		if controllerutil.ContainsFinalizer(vs, dptypes.DataProtectionFinalizerName) {
			patch := client.MergeFrom(vs.DeepCopy())
			controllerutil.RemoveFinalizer(vs, dptypes.DataProtectionFinalizerName)
			if err := vsCli.Patch(d.Ctx, vs, patch); err != nil {
				return err
			}
		}
		if !vs.DeletionTimestamp.IsZero() {
			return nil
		}
		d.Log.V(1).Info("delete volume snapshot", "volume snapshot", vs)
		if err := vsCli.Delete(d.Ctx, vs); err != nil {
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

func BuildDeleteBackupFilesJobKey(backup *dpv1alpha1.Backup, isPreDelete bool) client.ObjectKey {
	var preDeletePrefix string
	if isPreDelete {
		preDeletePrefix = "pre"
	}
	jobName := fmt.Sprintf("%s-%s%s%s", backup.UID[:8], preDeletePrefix, deleteBackupFilesJobNamePrefix, backup.Name)
	if len(jobName) > 63 {
		jobName = strings.TrimSuffix(jobName[:63], "-")
	}
	return client.ObjectKey{Namespace: backup.Namespace, Name: jobName}
}

func buildTargetPreDeleteJobKey(backup *dpv1alpha1.Backup, targetName string, targetIndex int) client.ObjectKey {
	jobName := fmt.Sprintf("%s-pre%s%d-%s-%s", backup.UID[:8], deleteBackupFilesJobNamePrefix,
		targetIndex, targetName, backup.Name)
	if len(jobName) > 63 {
		jobName = strings.TrimSuffix(jobName[:63], "-")
	}
	return client.ObjectKey{Namespace: backup.Namespace, Name: jobName}
}
