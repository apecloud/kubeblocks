package dataprotection

import (
	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

func MockBackupStatusMethod(backup *dpv1alpha1.Backup, targetVolume string) {
	snapshot := viper.GetBool("VOLUMESNAPSHOT")
	backupMethod := BackupMethodName
	if snapshot {
		backupMethod = VSBackupMethodName
	}
	backup.Status.BackupMethod = &dpv1alpha1.BackupMethod{
		Name:            backupMethod,
		SnapshotVolumes: &snapshot,
		TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
			Volumes: []string{targetVolume},
			VolumeMounts: []corev1.VolumeMount{
				{Name: targetVolume, MountPath: "/"},
			},
		},
	}
}
