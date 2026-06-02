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

	// DeleteBackupFilesJobLabelKey marks backup artifact cleanup jobs. These
	// jobs run after backup workload cleanup starts, so the controller must not
	// delete them as ordinary backup workload jobs before they finish.
	DeleteBackupFilesJobLabelKey = "dataprotection.kubeblocks.io/delete-backup-files-job"

	// externalDeleteJobTTLSecondsAfterFinished bounds how long a completed
	// backup-file cleanup Job created outside the Backup's own namespace
	// lingers before kube-controller-manager's ttl-after-finished controller
	// garbage-collects it (Job + Pod) alongside the existing best-effort
	// BackgroundDeleteObject path. Defense in depth: the Backup CR's
	// finalizer is removed as soon as DeleteBackupFiles returns Succeeded,
	// so if that async delete never propagates the Job has no other
	// cleanup signal — no cross-namespace ownerReference, no controller
	// re-reconcile. The 300s window keeps a short audit/diagnostic
	// inspection budget while bounding controller-namespace residue.
	externalDeleteJobTTLSecondsAfterFinished int32 = 300
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
	Client                               client.Client
	Scheme                               *runtime.Scheme
	WorkerServiceAccount                 string
	WorkerServiceAccountFunc             func() (string, error)
	WorkerServiceAccountForNamespaceFunc func(namespace string) (string, error)
	DeleteJobNamespace                   string
	PrepareDeleteJobBackupRepoFunc       func(repo *dpv1alpha1.BackupRepo, namespace string) error
	// RecordCleanupJobNamespaceFunc persists the namespace where the artifact
	// cleanup Job (delete or preDelete) is created onto the Backup status.
	// It is only called when the cleanup Job will run in a namespace different
	// from the Backup's own namespace (i.e. an external delete-job namespace
	// derived from the BackupRepo). The implementation must:
	//   - return nil and do nothing if backup.Status.CleanupJobNamespace
	//     already equals namespace (idempotent no-op),
	//   - return an error if backup.Status.CleanupJobNamespace is already
	//     non-empty and different from namespace (the caller must convert
	//     that into DeletionStatusUnknown and not proceed to create a job),
	//   - persist namespace via Status().Patch and also update the in-memory
	//     backup object so subsequent reads in the same reconcile see the
	//     recorded value.
	// When this field is nil the deleter degrades to legacy behavior where
	// the cleanup-job namespace is not persisted; this is intended only for
	// callers that never create external cleanup jobs.
	RecordCleanupJobNamespaceFunc func(backup *dpv1alpha1.Backup, namespace string) error

	actionSet *dpv1alpha1.ActionSet
}

func (d *Deleter) getWorkerServiceAccount(namespace string) (string, error) {
	if d.WorkerServiceAccount != "" {
		return d.WorkerServiceAccount, nil
	}
	if d.WorkerServiceAccountForNamespaceFunc != nil {
		saName, err := d.WorkerServiceAccountForNamespaceFunc(namespace)
		if err != nil {
			return "", err
		}
		if saName == "" {
			return "", fmt.Errorf("worker service account is empty")
		}
		d.WorkerServiceAccount = saName
		return saName, nil
	}
	if d.WorkerServiceAccountFunc == nil {
		return "", fmt.Errorf("worker service account is empty")
	}
	saName, err := d.WorkerServiceAccountFunc()
	if err != nil {
		return "", err
	}
	if saName == "" {
		return "", fmt.Errorf("worker service account is empty")
	}
	d.WorkerServiceAccount = saName
	return saName, nil
}

func (d *Deleter) buildDeleteBackupFilesJobKey(backup *dpv1alpha1.Backup, backupRepo *dpv1alpha1.BackupRepo, isPreDelete bool) client.ObjectKey {
	jobKey := BuildDeleteBackupFilesJobKey(backup, isPreDelete)
	if d.DeleteJobNamespace != "" && backupRepo != nil && backupRepo.AccessByTool() {
		jobKey.Namespace = d.DeleteJobNamespace
	}
	return jobKey
}

func isExternalDeleteJob(jobKey types.NamespacedName, backup *dpv1alpha1.Backup) bool {
	return jobKey.Namespace != backup.Namespace
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

	var backupRepo *dpv1alpha1.BackupRepo
	var err error
	if backup.Status.BackupRepoName != "" {
		backupRepo = &dpv1alpha1.BackupRepo{}
		if err = d.Client.Get(d.Ctx, client.ObjectKey{Name: backup.Status.BackupRepoName}, backupRepo); err != nil {
			if apierrors.IsNotFound(err) {
				// The BackupRepo is gone but an external cleanup Job may
				// still exist in a namespace recorded on the Backup status.
				// We must not return Succeeded just because we cannot
				// rebuild the BackupRepo-derived job key here.
				return d.checkExistingCleanupJobsAfterRepoGone(backup)
			}
			return DeletionStatusUnknown, err
		}
	}

	jobKey := d.buildDeleteBackupFilesJobKey(backup, backupRepo, false)
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
			if isExternalDeleteJob(client.ObjectKeyFromObject(job), backup) {
				if err := ctrlutil.BackgroundDeleteObject(d.Client, d.Ctx, job); err != nil {
					return DeletionStatusUnknown, err
				}
			}
			return DeletionStatusSucceeded, nil
		case batchv1.JobFailed:
			return DeletionStatusFailed,
				fmt.Errorf("deletion backup files job \"%s\" failed, you can delete it to re-delete the backup files, %s", job.Name, msg)
		}
		return DeletionStatusDeleting, nil
	}

	// if backupRepo is nil (likely because it's a legacy backup object), check the backup PVC
	var legacyPVCName string
	if backupRepo == nil {
		legacyPVCName = backup.Status.PersistentVolumeClaimName
		if legacyPVCName == "" {
			d.Log.Info("skip deleting backup files because PersistentVolumeClaimName is empty",
				"backup", backup.Name)
			return DeletionStatusSucceeded, nil
		}

		// check if the backup PVC exists, if not, skip to delete backup files
		pvcKey := client.ObjectKey{Namespace: backup.Namespace, Name: legacyPVCName}
		if err = d.Client.Get(d.Ctx, pvcKey, &corev1.PersistentVolumeClaim{}); err != nil {
			if apierrors.IsNotFound(err) {
				return DeletionStatusSucceeded, nil
			}
			return DeletionStatusUnknown, err
		}
	}

	backupFilePath := backup.Status.Path
	if backupFilePath == "" || (!strings.Contains(backupFilePath, backup.Name)) {
		// For compatibility: the FilePath field is changing from time to time,
		// and it may not contain the backup name as a path component if the Backup object
		// was created in a previous version. In this case, it's dangerous to execute
		// the deletion command. For example, files belongs to other Backups can be deleted as well.
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
		preJob, err := d.doPreDeleteAction(backup, backupRepo, preDeleteAction, legacyPVCName, backupFilePath)
		if err != nil {
			return DeletionStatusUnknown, err
		}
		_, finishedType, msg := utils.IsJobFinished(preJob)
		if finishedType == batchv1.JobFailed {
			return DeletionStatusFailed,
				fmt.Errorf("pre-delete backup files job \"%s\" failed, you can delete it to re-delete the backup files, %s", job.Name, msg)
		} else if finishedType != batchv1.JobComplete {
			return DeletionStatusDeleting, nil
		}
	}
	// do delete action
	if err := d.createDeleteBackupFilesJob(jobKey, backup, backupRepo, legacyPVCName); err != nil {
		// Anything that prevents the Job from being created — including
		// failure to persist the cleanup-job namespace via the callback —
		// is reported as Unknown so the controller does not advance the
		// deletion state machine and the finalizer is not removed.
		return DeletionStatusUnknown, err
	}
	return DeletionStatusDeleting, nil
}

// checkExistingCleanupJobsAfterRepoGone resolves the deletion status when
// BackupRepo has already been removed. It looks up both the delete-backup-files
// Job and the preDelete Job, aggregates their states, and never returns
// Succeeded based on a preDelete-only success — only an actual delete-backup-files
// Job completion proves the backup artifact was removed.
//
// Authoritative-lookup rule:
//   - When backup.Status.CleanupJobNamespace is non-empty the recorded
//     namespace is the authoritative location. If no cleanup Job is found in
//     it (or in any legacy fallback namespace) the function returns Unknown
//     rather than Succeeded. The recorded namespace records the system's
//     prior intent to create an external cleanup Job; a missing Job in that
//     case is "evidence is gone", not "no cleanup needed".
//   - When backup.Status.CleanupJobNamespace is empty the backup predates
//     namespace persistence. The function falls back to scanning the Backup's
//     own namespace plus the Deleter's d.DeleteJobNamespace default. If no
//     Job is found there either, the legacy Succeeded semantic is preserved.
func (d *Deleter) checkExistingCleanupJobsAfterRepoGone(backup *dpv1alpha1.Backup) (DeletionStatus, error) {
	recorded := backup.Status.CleanupJobNamespace
	namespaces := d.candidateCleanupNamespaces(backup, recorded)

	deleteState, err := d.aggregateJobState(backup, namespaces, false /* isPreDelete */)
	if err != nil {
		return DeletionStatusUnknown, err
	}
	preDeleteState, err := d.aggregateJobState(backup, namespaces, true /* isPreDelete */)
	if err != nil {
		return DeletionStatusUnknown, err
	}

	// Failure on either Job blocks finalizer removal.
	if deleteState.status == DeletionStatusFailed {
		return DeletionStatusFailed, deleteState.err
	}
	if preDeleteState.status == DeletionStatusFailed {
		return DeletionStatusFailed, preDeleteState.err
	}

	// Any unfinished Job means cleanup is still in progress.
	if deleteState.status == DeletionStatusDeleting || preDeleteState.status == DeletionStatusDeleting {
		return DeletionStatusDeleting, nil
	}

	// Only the delete-backup-files Job's completion implies the backup
	// artifact has actually been removed; a preDelete completion alone is
	// not sufficient.
	if deleteState.completedJob != nil {
		if isExternalDeleteJob(client.ObjectKeyFromObject(deleteState.completedJob), backup) {
			if err := ctrlutil.BackgroundDeleteObject(d.Client, d.Ctx, deleteState.completedJob); err != nil {
				return DeletionStatusUnknown, err
			}
		}
		if preDeleteState.completedJob != nil &&
			isExternalDeleteJob(client.ObjectKeyFromObject(preDeleteState.completedJob), backup) {
			if err := ctrlutil.BackgroundDeleteObject(d.Client, d.Ctx, preDeleteState.completedJob); err != nil {
				return DeletionStatusUnknown, err
			}
		}
		return DeletionStatusSucceeded, nil
	}

	// No Job was found anywhere.
	if recorded != "" {
		// The system previously committed to an external cleanup Job in a
		// known namespace. The Job is no longer observable, but we have no
		// proof that the backup artifact was deleted. Refuse to release
		// the finalizer.
		return DeletionStatusUnknown,
			fmt.Errorf("recorded cleanup job namespace %q has no cleanup job, "+
				"cannot confirm backup artifact deletion; manual intervention may be required",
				recorded)
	}

	// Only a preDelete Job completed, no delete Job exists anywhere, and we
	// have no recorded namespace to fall back to: preDelete success alone
	// must not be treated as artifact-cleanup success.
	if preDeleteState.completedJob != nil {
		return DeletionStatusUnknown,
			fmt.Errorf("pre-delete job %q completed but no delete-backup-files job exists; "+
				"cannot confirm backup artifact deletion",
				preDeleteState.completedJob.Name)
	}

	// Legacy backup: no recorded namespace and no Job in any candidate
	// namespace. Preserve the prior Succeeded semantic.
	return DeletionStatusSucceeded, nil
}

// candidateCleanupNamespaces returns the ordered list of namespaces to scan
// for an existing cleanup Job after BackupRepo is gone. The recorded
// namespace, when non-empty, is the authoritative first lookup; the Backup's
// own namespace and the Deleter's d.DeleteJobNamespace are added only as
// best-effort legacy fallbacks. Duplicates are removed in-order to keep the
// authoritative slot first.
func (d *Deleter) candidateCleanupNamespaces(backup *dpv1alpha1.Backup, recorded string) []string {
	candidates := make([]string, 0, 3)
	seen := map[string]struct{}{}
	add := func(ns string) {
		if ns == "" {
			return
		}
		if _, ok := seen[ns]; ok {
			return
		}
		seen[ns] = struct{}{}
		candidates = append(candidates, ns)
	}
	add(recorded)
	add(backup.Namespace)
	add(d.DeleteJobNamespace)
	return candidates
}

// cleanupJobState summarizes a single Job-kind lookup across candidate
// namespaces. Failed wins over Deleting wins over Completed wins over absent.
type cleanupJobState struct {
	status       DeletionStatus // Failed | Deleting | Succeeded (= completed) | "" (= absent)
	err          error          // populated when status == Failed
	completedJob *batchv1.Job   // populated when status == Succeeded
}

// aggregateJobState scans candidate namespaces for a cleanup Job of the
// requested kind and folds the results: Failed if any failed, else Deleting
// if any is running, else Succeeded if any completed, else absent.
func (d *Deleter) aggregateJobState(
	backup *dpv1alpha1.Backup,
	namespaces []string,
	isPreDelete bool,
) (cleanupJobState, error) {
	baseKey := BuildDeleteBackupFilesJobKey(backup, isPreDelete)
	state := cleanupJobState{}
	for _, ns := range namespaces {
		key := baseKey
		key.Namespace = ns
		job := &batchv1.Job{}
		exists, err := ctrlutil.CheckResourceExists(d.Ctx, d.Client, key, job)
		if err != nil {
			return cleanupJobState{}, err
		}
		if !exists {
			continue
		}
		_, finishedType, msg := utils.IsJobFinished(job)
		switch finishedType {
		case batchv1.JobFailed:
			return cleanupJobState{
				status: DeletionStatusFailed,
				err: fmt.Errorf("deletion backup files job %q failed, you can delete it to re-delete the backup files, %s",
					job.Name, msg),
			}, nil
		case batchv1.JobComplete:
			if state.status != DeletionStatusDeleting && state.completedJob == nil {
				state.status = DeletionStatusSucceeded
				state.completedJob = job.DeepCopy()
			}
		default:
			state.status = DeletionStatusDeleting
			state.completedJob = nil
		}
	}
	return state, nil
}

// recordCleanupJobNamespace persists the namespace of an external cleanup Job
// onto Backup status via the configured callback. It must be invoked before
// creating the Job. When the recorded namespace already equals namespace the
// callback is a no-op; when it is non-empty and differs the callback returns
// an error and the caller must convert that to DeletionStatusUnknown.
//
// Returns nil if the cleanup Job will run in the Backup's own namespace
// (legacy in-namespace path) — no external namespace to persist in that case.
func (d *Deleter) recordCleanupJobNamespace(backup *dpv1alpha1.Backup, jobKey client.ObjectKey) error {
	if !isExternalDeleteJob(jobKey, backup) {
		return nil
	}
	if d.RecordCleanupJobNamespaceFunc == nil {
		// Callers that create external cleanup jobs must wire the
		// callback. Refuse rather than silently leaving the namespace
		// unrecorded — silence here re-introduces the very orphan-Job
		// risk this field was added to close.
		return fmt.Errorf("cannot create external cleanup job in namespace %q: "+
			"RecordCleanupJobNamespaceFunc is not set", jobKey.Namespace)
	}
	return d.RecordCleanupJobNamespaceFunc(backup, jobKey.Namespace)
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
	backupRepo *dpv1alpha1.BackupRepo,
	legacyPVCName string) error {

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
	return d.createDeleteJob(container, jobKey, backup, backupRepo, legacyPVCName)
}

func (d *Deleter) createDeleteJob(container corev1.Container,
	jobKey types.NamespacedName,
	backup *dpv1alpha1.Backup,
	backupRepo *dpv1alpha1.BackupRepo,
	legacyPVCName string) error {
	ctrlutil.InjectZeroResourcesLimitsForDataProtection(&container)
	externalDeleteJob := isExternalDeleteJob(jobKey, backup)
	if externalDeleteJob {
		if backupRepo == nil || !backupRepo.AccessByTool() {
			return fmt.Errorf("backup file delete job in namespace %q requires a tool-access BackupRepo", jobKey.Namespace)
		}
		if d.PrepareDeleteJobBackupRepoFunc == nil {
			return fmt.Errorf("backup file delete job in namespace %q requires BackupRepo preparation", jobKey.Namespace)
		}
		// Persist the cleanup-job namespace on Backup status BEFORE any
		// side effects that create the Job. This closes the window where
		// the Job exists but its namespace was never recorded; if the
		// BackupRepo is later deleted we can still find the Job via the
		// recorded namespace.
		if err := d.recordCleanupJobNamespace(backup, jobKey); err != nil {
			return fmt.Errorf("failed to record cleanup job namespace %q for backup %q: %w",
				jobKey.Namespace, backup.Name, err)
		}
		if err := d.PrepareDeleteJobBackupRepoFunc(backupRepo, jobKey.Namespace); err != nil {
			return fmt.Errorf("failed to prepare BackupRepo %q in namespace %q for backup file deletion: %w",
				backupRepo.Name, jobKey.Namespace, err)
		}
	}
	serviceAccountName, err := d.getWorkerServiceAccount(jobKey.Namespace)
	if err != nil {
		return err
	}

	// build pod
	podSpec := corev1.PodSpec{
		Containers:         []corev1.Container{container},
		RestartPolicy:      corev1.RestartPolicyNever,
		ServiceAccountName: serviceAccountName,
	}
	if err := utils.AddTolerations(&podSpec); err != nil {
		return err
	}
	kopiaRepoPath := backup.Status.KopiaRepoPath
	encryptionConfig := backup.Status.EncryptionConfig
	if backupRepo != nil {
		utils.InjectDatasafed(&podSpec, backupRepo, RepoVolumeMountPath, encryptionConfig, kopiaRepoPath)
	} else {
		utils.InjectDatasafedWithPVC(&podSpec, legacyPVCName, RepoVolumeMountPath, kopiaRepoPath)
	}

	// build job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: jobKey.Namespace,
			Name:      jobKey.Name,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:   dptypes.AppName,
				dptypes.BackupNameLabelKey:      backup.Name,
				dptypes.BackupNamespaceLabelKey: backup.Namespace,
				DeleteBackupFilesJobLabelKey:    "true",
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
	if externalDeleteJob {
		// External Jobs live outside the Backup's namespace, so no
		// cross-namespace ownerReference can wire them to the Backup.
		// Set a TTL so kube-controller-manager garbage-collects the
		// completed Job + Pod even when the existing best-effort
		// BackgroundDeleteObject path does not propagate (e.g. the
		// Backup CR's finalizer is removed before the async delete
		// lands and no future reconcile is triggered).
		ttl := externalDeleteJobTTLSecondsAfterFinished
		job.Spec.TTLSecondsAfterFinished = &ttl
	}
	if !externalDeleteJob {
		if err := utils.SetControllerReference(backup, job, d.Scheme); err != nil {
			return err
		}
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

func (d *Deleter) doPreDeleteAction(
	backup *dpv1alpha1.Backup,
	backupRepo *dpv1alpha1.BackupRepo,
	preDeleteAction *dpv1alpha1.BaseJobActionSpec,
	legacyPVCName string,
	backupFilePath string) (*batchv1.Job, error) {
	preJobKey := d.buildDeleteBackupFilesJobKey(backup, backupRepo, true)
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
	return preJob, d.createDeleteJob(container, preJobKey, backup, backupRepo, legacyPVCName)
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
