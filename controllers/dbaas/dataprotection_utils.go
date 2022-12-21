package dbaas

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/leaanthony/debme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func buildSpecForRestore(reqCtx intctrlutil.RequestCtx,
	params createParams, cli client.Client, spec *corev1.PodSpec, claims []corev1.PersistentVolumeClaim) error {
	backupSource := params.component.BackupSource
	if backupSource == "" {
		return nil
	}

	// get backup job
	backupJob := &dpv1alpha1.BackupJob{}
	backupJobKey := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupSource,
	}
	if err := cli.Get(reqCtx.Ctx, backupJobKey, backupJob); err != nil {
		return err
	}
	if backupJob.Spec.BackupType == dpv1alpha1.BackupTypeSnapshot {
		return buildSpecWithBackupSnapshot(params, claims)
	} else {
		return buildSpecWithBackupTool(reqCtx, params, cli, backupJob, spec)
	}
}

// buildSpecWithBackupSnapshot build volume template if backup type is snapshot.
func buildSpecWithBackupSnapshot(params createParams, volumeClaims []corev1.PersistentVolumeClaim) error {
	if nil == volumeClaims {
		return errors.New("PersistentVolumeClaim is empty.")
	}
	snapshotGroupName := snapshotv1.GroupName
	for i := range volumeClaims {
		if volumeClaims[i].Spec.DataSource == nil {
			volumeClaims[i].Spec.DataSource = &corev1.TypedLocalObjectReference{
				Name:     params.component.BackupSource,
				Kind:     "VolumeSnapshot",
				APIGroup: &snapshotGroupName,
			}
		}
	}
	return nil
}

// buildSpecWithBackupTool build pod init container if backup type is full.
func buildSpecWithBackupTool(reqCtx intctrlutil.RequestCtx, params createParams, cli client.Client,
	backupJob *dpv1alpha1.BackupJob, spec *corev1.PodSpec) error {
	if nil == spec {
		return nil
	}

	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueFileName := "restore_init_template.cue"
	cueTpl, err := params.getCacheCUETplValue(cueFileName, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(cueFileName))
	})
	if err != nil {
		return err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	containerByte, err := cueValue.Lookup("restoreInitContainer")
	if err != nil {
		return err
	}

	container := corev1.Container{}
	if err = json.Unmarshal(containerByte, &container); err != nil {
		return err
	}

	// get backup policy
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	backupPolicyKey := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupJob.Spec.BackupPolicyName,
	}
	if err := cli.Get(reqCtx.Ctx, backupPolicyKey, backupPolicy); err != nil {
		return err
	}

	// get backup tool
	backupTool := &dpv1alpha1.BackupTool{}
	backupToolNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupPolicy.Spec.BackupToolName,
	}
	if err := cli.Get(reqCtx.Ctx, backupToolNameSpaceName, backupTool); err != nil {
		return err
	}

	container.Args[0] += strings.Join(backupTool.Spec.Physical.RestoreCommands, ";")
	container.Image = backupTool.Spec.Image
	if nil != backupTool.Spec.Resources {
		container.Resources = *backupTool.Spec.Resources
	}

	// default fetch first database container volume mounts
	container.VolumeMounts = spec.Containers[0].VolumeMounts

	// add remote volumeMounts
	remoteVolumeMount := corev1.VolumeMount{}
	remoteVolumeMount.Name = backupPolicy.Spec.RemoteVolume.Name
	remoteVolumeMount.MountPath = "/backupdata"

	container.VolumeMounts = append(container.VolumeMounts, remoteVolumeMount)

	// build and merge env from backup tool.
	envBackupName := corev1.EnvVar{
		Name:  "BACKUP_NAME",
		Value: backupJob.Name,
	}

	envBackupDir := corev1.EnvVar{
		Name:  "BACKUP_DIR",
		Value: remoteVolumeMount.MountPath + "/" + backupJob.Namespace,
	}

	container.Env = []corev1.EnvVar{envBackupName, envBackupDir}
	container.Env = append(container.Env, backupTool.Spec.Env...)

	// build InitContainers and volumes
	spec.InitContainers = append(spec.InitContainers, container)
	spec.Volumes = append(spec.Volumes, backupPolicy.Spec.RemoteVolume)
	return nil
}
