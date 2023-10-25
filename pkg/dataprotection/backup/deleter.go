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
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	deleteBackupFilesJobNamePrefix = "delete-"
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
	Client client.Client
	Scheme *runtime.Scheme
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
	jobKey := BuildDeleteBackupFilesJobKey(backup)
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

	var backupRepo *dpv1alpha1.BackupRepo
	if backup.Status.BackupRepoName != "" {
		backupRepo = &dpv1alpha1.BackupRepo{}
		if err = d.Client.Get(d.Ctx, client.ObjectKey{Name: backup.Status.BackupRepoName}, backupRepo); err != nil {
			if apierrors.IsNotFound(err) {
				return DeletionStatusSucceeded, nil
			}
			return DeletionStatusUnknown, err
		}
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
	if backupFilePath == "" || !strings.Contains(backupFilePath, backup.Name) {
		// For compatibility: the FilePath field is changing from time to time,
		// and it may not contain the backup name as a path component if the Backup object
		// was created in a previous version. In this case, it's dangerous to execute
		// the deletion command. For example, files belongs to other Backups can be deleted as well.
		d.Log.Info("skip deleting backup files because backup file path is invalid",
			"backupFilePath", backupFilePath, "backup", backup.Name)
		return DeletionStatusSucceeded, nil
	}
	return DeletionStatusDeleting, d.createDeleteBackupFilesJob(jobKey, backup, backupRepo, legacyPVCName, backup.Status.Path)
}

func (d *Deleter) createDeleteBackupFilesJob(
	jobKey types.NamespacedName,
	backup *dpv1alpha1.Backup,
	backupRepo *dpv1alpha1.BackupRepo,
	legacyPVCName string,
	backupFilePath string) error {
	// make sure the path has a leading slash
	if !strings.HasPrefix(backupFilePath, "/") {
		backupFilePath = "/" + backupFilePath
	}

	// this script first deletes the directory where the backup is located (including files
	// in the directory), and then traverses up the path level by level to clean up empty directories.
	deleteScript := fmt.Sprintf(`
set -x
export PATH="$PATH:$%s";
targetPath="%s";

echo "removing backup files in ${targetPath}";
datasafed rm -r "${targetPath}";

curr="${targetPath}";
while true; do
	parent=$(dirname "${curr}");
	if [ "${parent}" == "/" ]; then
		echo "reach to root, done";
		break;
	fi;
	result=$(datasafed list "${parent}");
	if [ -z "$result" ]; then
		echo "${parent} is empty, removing it...";
		datasafed rmdir "${parent}";
	else
		echo "${parent} is not empty, done";
		break;
	fi;
	curr="${parent}";
done
	`, dptypes.DPDatasafedBinPath, backupFilePath)

	runAsUser := int64(0)
	container := corev1.Container{
		Name:            backup.Name,
		Command:         []string{"sh", "-c"},
		Args:            []string{deleteScript},
		Image:           viper.GetString(constant.KBToolsImage),
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolptr.False(),
			RunAsUser:                &runAsUser,
		},
	}
	ctrlutil.InjectZeroResourcesLimitsIfEmpty(&container)

	// build pod
	podSpec := corev1.PodSpec{
		Containers:    []corev1.Container{container},
		RestartPolicy: corev1.RestartPolicyNever,
	}
	if err := utils.AddTolerations(&podSpec); err != nil {
		return err
	}
	if backupRepo != nil {
		utils.InjectDatasafed(&podSpec, backupRepo, RepoVolumeMountPath, backupFilePath)
	} else {
		utils.InjectDatasafedWithPVC(&podSpec, legacyPVCName, RepoVolumeMountPath, backupFilePath)
	}

	// build job
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
			BackoffLimit: &dptypes.DefaultBackOffLimit,
		},
	}
	if err := utils.SetControllerReference(backup, job, d.Scheme); err != nil {
		return err
	}
	d.Log.V(1).Info("create a job to delete backup files", "job", job)
	return client.IgnoreAlreadyExists(d.Client.Create(d.Ctx, job))
}

func (d *Deleter) DeleteVolumeSnapshots(backup *dpv1alpha1.Backup) error {
	// initialize volume snapshot client that is compatible with both v1beta1 and v1
	vsCli := &ctrlutil.VolumeSnapshotCompatClient{
		Client: d.Client,
		Ctx:    d.Ctx,
	}

	snaps := &vsv1.VolumeSnapshotList{}
	if err := vsCli.List(snaps, client.InNamespace(backup.Namespace),
		client.MatchingLabels(map[string]string{
			dptypes.BackupNameLabelKey: backup.Name,
		})); err != nil {
		return client.IgnoreNotFound(err)
	}

	deleteVolumeSnapshot := func(vs *vsv1.VolumeSnapshot) error {
		if controllerutil.ContainsFinalizer(vs, dptypes.DataProtectionFinalizerName) {
			patch := vs.DeepCopy()
			controllerutil.RemoveFinalizer(vs, dptypes.DataProtectionFinalizerName)
			if err := vsCli.Patch(vs, patch); err != nil {
				return err
			}
		}
		if !vs.DeletionTimestamp.IsZero() {
			return nil
		}
		d.Log.V(1).Info("delete volume snapshot", "volume snapshot", vs)
		if err := vsCli.Delete(vs); err != nil {
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

func BuildDeleteBackupFilesJobKey(backup *dpv1alpha1.Backup) client.ObjectKey {
	jobName := fmt.Sprintf("%s-%s%s", backup.UID[:8], deleteBackupFilesJobNamePrefix, backup.Name)
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}
	return client.ObjectKey{Namespace: backup.Namespace, Name: jobName}
}
