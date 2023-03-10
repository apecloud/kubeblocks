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

package component

import (
	"fmt"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func getBackupObjects(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	namespace string,
	backupName string) (*dataprotectionv1alpha1.Backup, *dataprotectionv1alpha1.BackupTool, error) {
	// get backup
	backup := &dataprotectionv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: backupName, Namespace: namespace}, backup); err != nil {
		return nil, nil, err
	}

	// get backup tool
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	if backup.Spec.BackupType != dataprotectionv1alpha1.BackupTypeSnapshot {
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: backup.Status.BackupToolName}, backupTool); err != nil {
			return nil, nil, err
		}
	}
	return backup, backupTool, nil
}

// BuildRestoredInfo builds restore-related infos if it needs to restore from backup, such as init container/pvc dataSource.
func BuildRestoredInfo(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	namespace string,
	component *SynthesizedComponent,
	backupName string) error {
	backup, backupTool, err := getBackupObjects(reqCtx, cli, namespace, backupName)
	if err != nil {
		return err
	}
	if backup.Status.Phase != dataprotectionv1alpha1.BackupCompleted {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeBackupNotCompleted, "backup %s is not completed", backup.Name)
	}
	switch backup.Spec.BackupType {
	case dataprotectionv1alpha1.BackupTypeFull:
		return buildInitContainerWithFullBackup(component, backup, backupTool)
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		buildVolumeClaimTemplatesWithSnapshot(component, backup)
	}
	return nil
}

// GetRestoredInitContainerName gets the init container name for restore.
func GetRestoredInitContainerName(backupName string) string {
	return fmt.Sprintf("restore-%s", backupName)
}

// buildInitContainerWithFullBackup builds the init container if it needs to restore from full backup
func buildInitContainerWithFullBackup(
	component *SynthesizedComponent,
	backup *dataprotectionv1alpha1.Backup,
	backupTool *dataprotectionv1alpha1.BackupTool) error {
	if component.PodSpec == nil || len(component.PodSpec.Containers) == 0 {
		return nil
	}
	if backup.Status.RemoteVolume == nil {
		return fmt.Errorf("remote volume can not be empty in Backup.status.remoteVolume")
	}
	container := corev1.Container{}
	container.Name = GetRestoredInitContainerName(backup.Name)
	container.Command = []string{"sh", "-c"}
	container.Args = backupTool.Spec.Physical.RestoreCommands
	container.Image = backupTool.Spec.Image
	if backupTool.Spec.Resources != nil {
		container.Resources = *backupTool.Spec.Resources
	}
	container.VolumeMounts = component.PodSpec.Containers[0].VolumeMounts
	// add the volumeMounts with backup volume
	randomVolumeName := fmt.Sprintf("%s-%s", backup.Status.RemoteVolume.Name, rand.String(6))
	backup.Status.RemoteVolume.Name = randomVolumeName
	remoteVolumeMount := corev1.VolumeMount{}
	remoteVolumeMount.Name = randomVolumeName
	remoteVolumeMount.MountPath = "/" + backup.Name
	container.VolumeMounts = append(container.VolumeMounts, remoteVolumeMount)

	allowPrivilegeEscalation := false
	runAsUser := int64(0)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		RunAsUser:                &runAsUser}

	// build env for restore
	container.Env = []corev1.EnvVar{
		{
			Name:  "BACKUP_NAME",
			Value: backup.Name,
		}, {
			Name:  "BACKUP_DIR",
			Value: fmt.Sprintf("/%s/%s", backup.Name, backup.Namespace),
		}}
	// merge env from backup tool.
	container.Env = append(container.Env, backupTool.Spec.Env...)
	// add volume of backup data
	component.PodSpec.Volumes = append(component.PodSpec.Volumes, *backup.Status.RemoteVolume)
	component.PodSpec.InitContainers = append(component.PodSpec.InitContainers, container)
	return nil
}

// buildVolumeClaimTemplatesWithSnapshot builds the volumeClaimTemplate if it needs to restore from volumeSnapshot
func buildVolumeClaimTemplatesWithSnapshot(component *SynthesizedComponent,
	backup *dataprotectionv1alpha1.Backup) {
	if len(component.VolumeClaimTemplates) == 0 {
		return
	}
	vct := component.VolumeClaimTemplates[0]
	snapshotAPIGroup := snapshotv1.GroupName
	vct.Spec.DataSource = &corev1.TypedLocalObjectReference{
		APIGroup: &snapshotAPIGroup,
		Kind:     constant.VolumeSnapshotKind,
		Name:     backup.Name,
	}
	component.VolumeClaimTemplates[0] = vct
}
