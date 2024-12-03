/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/action"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	BackupDataJobNamePrefix = "dp-backup"
	prebackupJobNamePrefix  = "dp-prebackup"
	postbackupJobNamePrefix = "dp-postbackup"
	BackupDataContainerName = "backupdata"
	managerContainerName    = "manager"
	managerSharedVolumeName = "manager-shared-volume"
	managerSharedMountPath  = "/dp-manager"
)

// Request is a request for a backup, with all references to other objects.
type Request struct {
	*dpv1alpha1.Backup
	intctrlutil.RequestCtx

	Client               client.Client
	BackupPolicy         *dpv1alpha1.BackupPolicy
	BackupMethod         *dpv1alpha1.BackupMethod
	ActionSet            *dpv1alpha1.ActionSet
	TargetPods           []*corev1.Pod
	BackupRepoPVC        *corev1.PersistentVolumeClaim
	BackupRepo           *dpv1alpha1.BackupRepo
	ToolConfigSecret     *corev1.Secret
	WorkerServiceAccount string
	SnapshotVolumes      bool
	Target               *dpv1alpha1.BackupTarget
}

func (r *Request) GetBackupType() string {
	if r.ActionSet != nil {
		return string(r.ActionSet.Spec.BackupType)
	}
	if r.BackupMethod != nil && boolptr.IsSetToTrue(r.BackupMethod.SnapshotVolumes) {
		return string(dpv1alpha1.BackupTypeFull)
	}
	return ""
}

// BuildActions builds the actions for the backup.
func (r *Request) BuildActions() (map[string][]action.Action, error) {
	var actions = map[string][]action.Action{}

	appendIgnoreNil := func(podActions []action.Action, elems ...action.Action) []action.Action {
		for _, elem := range elems {
			if elem == nil || reflect.ValueOf(elem).IsNil() {
				continue
			}
			podActions = append(podActions, elem)
		}
		return podActions
	}

	for i := range r.TargetPods {
		var podActions []action.Action

		// 1. build pre-backup actions
		if err := r.buildPreBackupActions(&podActions, r.TargetPods[i], i); err != nil {
			return nil, err
		}

		// 2. build backup data action
		backupDataAction, err := r.buildBackupDataAction(r.TargetPods[i], fmt.Sprintf("%s-%s%d", BackupDataJobNamePrefix, r.getActionTargetPrefix(), i))
		if err != nil {
			return nil, err
		}
		podActions = appendIgnoreNil(podActions, backupDataAction)

		// 3. build create volume snapshot action
		createVolumeSnapshotAction, err := r.buildCreateVolumeSnapshotAction(r.TargetPods[i], fmt.Sprintf("createVolumeSnapshot-%s%d", r.getActionTargetPrefix(), i), i)
		if err != nil {
			return nil, err
		}
		podActions = appendIgnoreNil(podActions, createVolumeSnapshotAction)

		// 4. build post-backup actions
		if err = r.buildPostBackupActions(&podActions, r.TargetPods[i], i); err != nil {
			return nil, err
		}
		actions[r.TargetPods[i].Name] = podActions
	}

	// TODO: build backup kubernetes resources action
	return actions, nil
}

func (r *Request) getActionTargetPrefix() string {
	if r.Target != nil && r.Target.Name != "" {
		return r.Target.Name + "-"
	}
	return ""
}

func (r *Request) buildPreBackupActions(podActions *[]action.Action, targetPod *corev1.Pod, index int) error {
	if !r.backupActionSetExists() ||
		len(r.ActionSet.Spec.Backup.PreBackup) == 0 {
		return nil
	}
	for i, preBackup := range r.ActionSet.Spec.Backup.PreBackup {
		a, err := r.buildAction(targetPod, fmt.Sprintf("%s-%s%d-%d", prebackupJobNamePrefix, r.getActionTargetPrefix(), i, index), &preBackup)
		if err != nil {
			return err
		}
		*podActions = append(*podActions, a)
	}
	return nil
}

func (r *Request) buildPostBackupActions(podActions *[]action.Action, targetPod *corev1.Pod, index int) error {
	if !r.backupActionSetExists() ||
		len(r.ActionSet.Spec.Backup.PostBackup) == 0 {
		return nil
	}

	for i, postBackup := range r.ActionSet.Spec.Backup.PostBackup {
		a, err := r.buildAction(targetPod, fmt.Sprintf("%s-%s%d-%d", postbackupJobNamePrefix, r.getActionTargetPrefix(), i, index), &postBackup)
		if err != nil {
			return err
		}
		*podActions = append(*podActions, a)
	}
	return nil
}

func (r *Request) buildBackupDataAction(targetPod *corev1.Pod, name string) (action.Action, error) {
	if !r.backupActionSetExists() ||
		r.ActionSet.Spec.Backup.BackupData == nil {
		return nil, nil
	}

	backupDataAct := r.ActionSet.Spec.Backup.BackupData
	switch r.ActionSet.Spec.BackupType {
	case dpv1alpha1.BackupTypeFull, dpv1alpha1.BackupTypeSelective:
		podSpec, err := r.BuildJobActionPodSpec(targetPod, BackupDataContainerName, &backupDataAct.JobActionSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to build job action pod spec: %w", err)
		}
		r.InjectManagerContainer(podSpec, backupDataAct.SyncProgress, r.buildSyncProgressCommand())
		return &action.JobAction{
			Name:         name,
			ObjectMeta:   *buildBackupJobObjMeta(r.Backup, name),
			Owner:        r.Backup,
			PodSpec:      podSpec,
			BackOffLimit: r.BackupPolicy.Spec.BackoffLimit,
		}, nil
	case dpv1alpha1.BackupTypeContinuous:
		podSpec, err := r.BuildJobActionPodSpec(r.TargetPods[0], BackupDataContainerName, &backupDataAct.JobActionSpec)
		if err != nil {
			return nil, err
		}
		r.InjectManagerContainer(podSpec, backupDataAct.SyncProgress, r.buildContinuousSyncProgressCommand())
		return &action.StatefulSetAction{
			Name: name,
			ObjectMeta: metav1.ObjectMeta{
				Namespace: r.Namespace,
				Name:      GenerateBackupStatefulSetName(r.Backup, r.Target.Name, BackupDataJobNamePrefix),
				Labels:    BuildBackupWorkloadLabels(r.Backup),
			},
			Replicas:  pointer.Int32(int32(1)),
			Backup:    r.Backup,
			PodSpec:   podSpec,
			ActionSet: r.ActionSet,
		}, nil
	}
	return nil, fmt.Errorf("unsupported backup type %s", r.ActionSet.Spec.BackupType)
}

func (r *Request) buildCreateVolumeSnapshotAction(targetPod *corev1.Pod, name string, index int) (action.Action, error) {
	if r.BackupMethod == nil ||
		!boolptr.IsSetToTrue(r.BackupMethod.SnapshotVolumes) {
		return nil, nil
	}

	if r.BackupMethod.TargetVolumes == nil {
		return nil, fmt.Errorf("targetVolumes is required for snapshotVolumes")
	}

	if volumeSnapshotEnabled, err := utils.VolumeSnapshotEnabled(r.Ctx, r.Client, targetPod, r.BackupMethod.TargetVolumes.Volumes); err != nil {
		return nil, err
	} else if !volumeSnapshotEnabled {
		return nil, fmt.Errorf("current backup method depends on volume snapshot, but volume snapshot is not enabled")
	}

	pvcs, err := getPVCsByVolumeNames(r.Client, targetPod, r.BackupMethod.TargetVolumes.Volumes)
	if err != nil {
		return nil, err
	}

	if len(pvcs) == 0 {
		return nil, fmt.Errorf("no PVCs found for pod %s to back up", targetPod.Name)
	}

	return &action.CreateVolumeSnapshotAction{
		Name:          name,
		Index:         index,
		TargetPodName: targetPod.Name,
		TargetName:    r.Target.Name,
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.Backup.Namespace,
			Name:      r.Backup.Name,
			Labels:    BuildBackupWorkloadLabels(r.Backup),
		},
		Owner:                         r.Backup,
		PersistentVolumeClaimWrappers: pvcs,
	}, nil
}

func (r *Request) buildAction(targetPod *corev1.Pod,
	name string,
	act *dpv1alpha1.ActionSpec) (action.Action, error) {
	if act.Exec == nil && act.Job == nil {
		return nil, fmt.Errorf("action %s has no exec or job", name)
	}
	if act.Exec != nil && act.Job != nil {
		return nil, fmt.Errorf("action %s should have only one of exec or job", name)
	}
	switch {
	case act.Exec != nil:
		return r.buildExecAction(targetPod, name, act.Exec), nil
	case act.Job != nil:
		return r.buildJobAction(targetPod, name, act.Job)
	}
	return nil, nil
}

func (r *Request) buildExecAction(targetPod *corev1.Pod,
	name string,
	exec *dpv1alpha1.ExecActionSpec) action.Action {
	objectMeta := *buildBackupJobObjMeta(r.Backup, name)
	objectMeta.Labels[dptypes.BackupNamespaceLabelKey] = r.Namespace
	// create exec job in kubeblocks namespace for security
	objectMeta.Namespace = viper.GetString(constant.CfgKeyCtrlrMgrNS)
	containerName := exec.Container
	if exec.Container == "" {
		containerName = targetPod.Spec.Containers[0].Name
	}
	return &action.ExecAction{
		JobAction: action.JobAction{
			Name:       name,
			ObjectMeta: objectMeta,
			Owner:      r.Backup,
		},
		Command:            exec.Command,
		Container:          containerName,
		Namespace:          targetPod.Namespace,
		PodName:            targetPod.Name,
		Timeout:            exec.Timeout,
		ServiceAccountName: viper.GetString(dptypes.CfgKeyExecWorkerServiceAccountName),
	}
}

func (r *Request) buildJobAction(targetPod *corev1.Pod,
	name string,
	job *dpv1alpha1.JobActionSpec) (action.Action, error) {
	podSpec, err := r.BuildJobActionPodSpec(targetPod, name, job)
	if err != nil {
		return nil, err
	}
	return &action.JobAction{
		Name:         name,
		ObjectMeta:   *buildBackupJobObjMeta(r.Backup, name),
		Owner:        r.Backup,
		PodSpec:      podSpec,
		BackOffLimit: r.BackupPolicy.Spec.BackoffLimit,
	}, nil
}

func (r *Request) BuildJobActionPodSpec(targetPod *corev1.Pod,
	name string,
	job *dpv1alpha1.JobActionSpec) (*corev1.PodSpec, error) {

	// build environment variables, include built-in envs, envs from backupMethod
	// and envs from actionSet. Latter will override former for the same name.
	// env from backupMethod has the highest priority.
	buildEnv := func() ([]corev1.EnvVar, error) {
		envVars := targetPod.Spec.Containers[0].Env
		envVars = append(envVars, []corev1.EnvVar{
			{
				Name:  dptypes.DPBackupName,
				Value: r.Backup.Name,
			},
			{
				Name:  dptypes.DPParentBackupName,
				Value: r.Backup.Spec.ParentBackupName,
			},
			{
				Name:  dptypes.DPTargetPodName,
				Value: targetPod.Name,
			},
			{
				Name:  dptypes.DPTargetPodRole,
				Value: targetPod.Labels[constant.RoleLabelKey],
			},
			{
				Name: dptypes.DPBackupBasePath,
				Value: BuildBackupPathByTarget(r.Backup, r.Target,
					r.BackupRepo.Spec.PathPrefix, r.BackupPolicy.Spec.PathPrefix, targetPod.Name),
			},
			{
				Name:  dptypes.DPBackupInfoFile,
				Value: managerSharedMountPath + "/" + BackupInfoFileName,
			},
			{
				Name:  dptypes.DPTTL,
				Value: r.Spec.RetentionPeriod.String(),
			},
		}...)
		envFromTarget, err := utils.BuildEnvByTarget(targetPod, r.Target.ConnectionCredential, r.Target.ContainerPort)
		if err != nil {
			return nil, err
		}
		envVars = append(envVars, envFromTarget...)
		if r.ActionSet != nil {
			envVars = append(envVars, r.ActionSet.Spec.Env...)
			envVars = append(envVars, utils.BuildEnvByParameters(r.Backup.Spec.Parameters)...)
		}
		// build envs for kb cluster
		setKBClusterEnv := func(labelKey, envName string) {
			if v, ok := r.Backup.Labels[labelKey]; ok {
				envVars = append(envVars, corev1.EnvVar{Name: envName, Value: v})
			}
		}
		setKBClusterEnv(dptypes.ClusterUIDLabelKey, constant.KBEnvClusterUID)
		setKBClusterEnv(constant.AppInstanceLabelKey, constant.KBEnvClusterName)
		setKBClusterEnv(constant.KBAppComponentLabelKey, constant.KBEnvCompName)
		envVars = append(envVars, corev1.EnvVar{Name: constant.KBEnvNamespace, Value: r.Namespace})
		return utils.MergeEnv(envVars, r.BackupMethod.Env), nil
	}

	runOnTargetPodNode := func() bool {
		return boolptr.IsSetToTrue(job.RunOnTargetPodNode)
	}

	buildVolumes := func() []corev1.Volume {
		volumes := []corev1.Volume{
			{
				Name: managerSharedVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
		// only mount the volumes when the backup pod is running on the target pod node.
		if runOnTargetPodNode() {
			volumes = append(volumes, getVolumesByVolumeInfo(targetPod, r.BackupMethod.TargetVolumes)...)
		}
		return volumes
	}

	buildVolumeMounts := func() []corev1.VolumeMount {
		volumesMount := []corev1.VolumeMount{
			{
				Name:      managerSharedVolumeName,
				MountPath: managerSharedMountPath,
			},
		}
		// only mount the volumes when the backup pod is running on the target pod node.
		if runOnTargetPodNode() {
			volumesMount = append(volumesMount, getVolumeMountsByVolumeInfo(targetPod, r.BackupMethod.TargetVolumes)...)
		}
		return volumesMount
	}

	runAsUser := int64(0)
	env, err := buildEnv()
	if err != nil {
		return nil, err
	}
	container := corev1.Container{
		Name: name,
		// expand the image value with the env variables.
		Image:           common.Expand(job.Image, common.MappingFuncFor(utils.CovertEnvToMap(env))),
		Command:         job.Command,
		Env:             env,
		EnvFrom:         targetPod.Spec.Containers[0].EnvFrom,
		VolumeMounts:    buildVolumeMounts(),
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolptr.False(),
			RunAsUser:                &runAsUser,
		},
	}

	if r.BackupMethod.RuntimeSettings != nil {
		container.Resources = r.BackupMethod.RuntimeSettings.Resources
	}

	if r.ActionSet != nil {
		container.EnvFrom = append(container.EnvFrom, r.ActionSet.Spec.EnvFrom...)
	}

	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&container)

	podSpec := &corev1.PodSpec{
		Containers:         []corev1.Container{container},
		Volumes:            buildVolumes(),
		ServiceAccountName: r.WorkerServiceAccount,
		RestartPolicy:      corev1.RestartPolicyNever,
	}

	// if run on target pod node, set backup pod tolerations same as the target pod,
	// that will make sure the backup pod can be scheduled to the target pod node.
	// If not, just use the tolerations built by the environment variables.
	if runOnTargetPodNode() {
		podSpec.Tolerations = targetPod.Spec.Tolerations
		podSpec.NodeSelector = map[string]string{
			corev1.LabelHostname: targetPod.Spec.NodeName,
		}
	} else {
		if err := utils.AddTolerations(podSpec); err != nil {
			return nil, err
		}
	}

	utils.InjectDatasafed(podSpec, r.BackupRepo, RepoVolumeMountPath,
		r.Status.EncryptionConfig, r.Status.KopiaRepoPath)
	return podSpec, nil
}

func (r *Request) buildSyncProgressCommand() string {
	// sync progress script will wait for the backup info file to be created,
	// if the file is created, it will update the backup status and exit.
	// If an exit file named with the backup info file with .exit suffix exists,
	// it indicates that the container for backing up data exited abnormally,
	// this script will exit.
	return fmt.Sprintf(`
set -o errexit
set -o nounset

export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"

backup_info_file="${%s}"
sleep_seconds="${%s}"
namespace="%s"
backup_name="%s"

if [ "$sleep_seconds" -le 0 ]; then
  sleep_seconds=30
fi

exit_file="${backup_info_file}.exit"
while true; do
  if [ -f "$exit_file" ]; then
    echo "exit file $exit_file exists, exit"
    exit 1
  fi
  if [ -f "$backup_info_file" ]; then
    break
  fi
  echo "backup info file not exists, wait for ${sleep_seconds}s"
  sleep "$sleep_seconds"
done

backup_info=$(cat "$backup_info_file")
echo "backupInfo:${backup_info}"

status="{\"status\":${backup_info}}"
kubectl -n "$namespace" patch backups.dataprotection.kubeblocks.io "$backup_name" --subresource=status --type=merge --patch "${status}"

# save the backup CR object to the backup repo
kubectl -n "$namespace" get backups.dataprotection.kubeblocks.io "$backup_name" -o json | datasafed push - "/kubeblocks-backup.json"
`, dptypes.DPBackupInfoFile, dptypes.DPCheckInterval, r.Backup.Namespace, r.Backup.Name)
}

func (r *Request) buildContinuousSyncProgressCommand() string {
	// sync progress script will wait for the backup info file to be created,
	// if the file is created, it will update the backup status and exit.
	// If an exit file named with the backup info file with .exit suffix exists,
	// it indicates that the container for backing up data exited abnormally,
	// this script will exit.
	return fmt.Sprintf(`
set -o errexit
set -o nounset

retryTimes=0
oldBackupInfo=
backupInfoFile=${%s}
trap "echo 'Terminating...' && exit" TERM
while true; do
  sleep ${%s};
  if [ ! -f ${backupInfoFile} ]; then
    continue
  fi
  backupInfo=$(cat ${backupInfoFile})
  if [ "${oldBackupInfo}" == "${backupInfo}" ]; then
    continue
  fi
  echo "start to patch backupInfo: ${backupInfo}"
  status="{\"status\":${backupInfo}}"
  kubectl -n %s patch backups.dataprotection.kubeblocks.io %s --subresource=status --type=merge --patch "${status}"
  if [ $? -ne 0 ]; then
    retryTimes=$(($retryTimes+1))
  else
    echo "update backup status successfully"
    retryTimes=0
    oldBackupInfo=${backupInfo}
  fi
  if [ $retryTimes -ge 3 ]; then
    echo "ERROR: update backup status failed, 3 attempts have been made!"
    exit 1
  fi
done
`, dptypes.DPBackupInfoFile, dptypes.DPCheckInterval, r.Backup.Namespace, r.Backup.Name)
}

// InjectManagerContainer injects a sidecar that will sync the backup status
// or push the backup CR object to the backup repo.
func (r *Request) InjectManagerContainer(podSpec *corev1.PodSpec,
	sync *dpv1alpha1.SyncProgress, command string) {

	// build container to sync backup progress that will update the backup status
	container := podSpec.Containers[0].DeepCopy()
	container.Name = managerContainerName
	container.Image = viper.GetString(constant.KBToolsImage)
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy))
	container.Resources = corev1.ResourceRequirements{Limits: nil, Requests: nil}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
	container.Command = []string{"sh", "-c"}

	// append some envs
	checkIntervalSeconds := int32(5)
	if sync != nil && sync.IntervalSeconds != nil && *sync.IntervalSeconds > 0 {
		checkIntervalSeconds = *sync.IntervalSeconds
	}
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  dptypes.DPCheckInterval,
			Value: fmt.Sprintf("%d", checkIntervalSeconds)},
	)
	container.Args = []string{command}
	podSpec.Containers = append(podSpec.Containers, *container)
}

func (r *Request) backupActionSetExists() bool {
	return r.ActionSet != nil && r.ActionSet.Spec.Backup != nil
}
