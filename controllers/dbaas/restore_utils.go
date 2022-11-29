package dbaas

import (
	"encoding/json"
	"strings"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/leaanthony/debme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type initRestoreOptions struct {
	Image    string `json:"image"`
	Args     string `json:"args,omitempty"`
	Resource corev1.ResourceRequirements
}

func buildTemplatePodSpecForRestore(reqCtx intctrlutil.RequestCtx,
	params createParams, cli client.Client, spec *corev1.PodSpec) error {
	sourceBackup := params.component.SourceBackup
	if sourceBackup == "" {
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

	// get backup job
	backupJob := &dataprotectionv1alpha1.BackupJob{}
	backupJobKey := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      sourceBackup,
	}
	if err := cli.Get(reqCtx.Ctx, backupJobKey, backupJob); err != nil {
		return err
	}

	// get backup policy
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	backupPolicyKey := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupJob.Spec.BackupPolicyName,
	}
	if err := cli.Get(reqCtx.Ctx, backupPolicyKey, backupPolicy); err != nil {
		return err
	}

	// get backup tool
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	backupToolNameSpaceName := types.NamespacedName{
		Namespace: reqCtx.Req.Namespace,
		Name:      backupPolicy.Spec.BackupToolName,
	}
	if err := cli.Get(reqCtx.Ctx, backupToolNameSpaceName, backupTool); err != nil {
		return err
	}

	container.Args[0] = container.Args[0] + strings.Join(backupTool.Spec.Physical.RestoreCommands, ";")
	container.Image = backupTool.Spec.Image
	container.Resources = backupTool.Spec.Resources

	// default fetch first database container volume mounts
	container.VolumeMounts = params.component.PodSpec.Containers[0].VolumeMounts

	// add remote volumeMounts
	remoteVolumeMount := corev1.VolumeMount{}
	remoteVolumeMount.Name = backupPolicy.Spec.RemoteVolume.Name
	remoteVolumeMount.MountPath = "/backupdata"

	container.VolumeMounts = append(container.VolumeMounts, remoteVolumeMount)

	/*
		allowPrivilegeEscalation := false
		runAsUser := int64(0)
		container.SecurityContext = &corev1.SecurityContext{
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			RunAsUser:                &runAsUser}

	*/

	// build env for restore
	envBackupName := corev1.EnvVar{
		Name:  "BACKUP_NAME",
		Value: backupJob.Name,
	}

	envBackupDir := corev1.EnvVar{
		Name:  "BACKUP_DIR",
		Value: remoteVolumeMount.MountPath + "/" + backupJob.Namespace,
	}

	container.Env = []corev1.EnvVar{envBackupName, envBackupDir}
	// merge env from backup tool.
	container.Env = append(container.Env, backupTool.Spec.Env...)
	spec.InitContainers = append(spec.InitContainers, container)
	spec.Volumes = append(spec.Volumes, backupPolicy.Spec.RemoteVolume)
	return nil
}
