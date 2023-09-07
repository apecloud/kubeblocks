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

	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/dataprotection/action"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
)

// Request is a request for a backup, with all references to other objects.
type Request struct {
	*dpv1alpha1.Backup
	intctrlutil.RequestCtx

	BackupPolicy  *dpv1alpha1.BackupPolicy
	BackupMethod  *dpv1alpha1.BackupMethod
	ActionSet     *dpv1alpha1.ActionSet
	TargetPods    []*corev1.Pod
	BackupRepoPVC *corev1.PersistentVolumeClaim
	BackupRepo    *dpv1alpha1.BackupRepo
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
func (r *Request) BuildActions() ([]action.Action, error) {
	var actions []action.Action

	// build pre-backup actions
	preBackupActions, err := r.buildPreBackupActions()
	if err != nil {
		return nil, err
	}

	// build backup data action
	backupDataAction, err := r.buildBackupDataAction()
	if err != nil {
		return nil, err
	}

	// build create volume snapshot action
	createVolumeSnapshotAction, err := r.buildCreateVolumeSnapshotAction()
	if err != nil {
		return nil, err
	}

	// build backup kubernetes resources action
	backupKubeResourcesAction, err := r.buildBackupKubeResourcesAction()
	if err != nil {
		return nil, err
	}

	// build post-backup actions
	postBackupActions, err := r.buildPostBackupActions()
	if err != nil {
		return nil, err
	}

	actions = append(actions, preBackupActions...)
	actions = append(actions, backupDataAction, createVolumeSnapshotAction, backupKubeResourcesAction)
	actions = append(actions, postBackupActions...)
	return actions, nil
}

func (r *Request) buildPreBackupActions() ([]action.Action, error) {
	var actions []action.Action
	if r.ActionSet == nil ||
		r.ActionSet.Spec.Backup == nil ||
		len(r.ActionSet.Spec.Backup.PreBackup) == 0 {
		return actions, nil
	}

	for i, preBackup := range r.ActionSet.Spec.Backup.PreBackup {
		a, err := r.buildAction(fmt.Sprintf("prebackup-%d", i), &preBackup)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, nil
}

func (r *Request) buildPostBackupActions() ([]action.Action, error) {
	var actions []action.Action
	if r.ActionSet == nil ||
		r.ActionSet.Spec.Backup == nil ||
		len(r.ActionSet.Spec.Backup.PostBackup) == 0 {
		return actions, nil
	}

	for i, postBackup := range r.ActionSet.Spec.Backup.PostBackup {
		a, err := r.buildAction(fmt.Sprintf("postbackup-%d", i), &postBackup)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, nil
}

func (r *Request) buildBackupDataAction() (action.Action, error) {
	return nil, nil
}

func (r *Request) buildCreateVolumeSnapshotAction() (action.Action, error) {
	return nil, nil
}

func (r *Request) buildBackupKubeResourcesAction() (action.Action, error) {
	return nil, nil
}

func (r *Request) buildAction(name string, act *dpv1alpha1.Action) (action.Action, error) {
	if act.Exec == nil && act.Job == nil {
		return nil, fmt.Errorf("action %s has no exec or job", name)
	}
	if act.Exec != nil && act.Job != nil {
		return nil, fmt.Errorf("action %s should have only one of exec or job", name)
	}
	switch {
	case act.Exec != nil:
		return r.buildExecAction(name, act.Exec), nil
	case act.Job != nil:
		return r.buildJobAction(name, act.Job)
	}
	return nil, nil
}

func (r *Request) buildExecAction(name string, exec *dpv1alpha1.ExecAction) action.Action {
	targetPod := r.TargetPods[0]
	return &action.KubeExec{
		Name:      name,
		Command:   exec.Command,
		Container: exec.Container,
		Namespace: targetPod.Namespace,
		PodName:   targetPod.Name,
		Timeout:   exec.Timeout,
	}
}

func (r *Request) buildJobAction(name string, job *dpv1alpha1.JobAction) (action.Action, error) {
	targetPod := r.TargetPods[0]

	// build environment variables, include built-in envs, envs from backupMethod
	// and envs from actionSet. Latter will override former for the same name
	// env from backupMethod has the highest priority.
	buildEnv := func() []corev1.EnvVar {
		envVars := []corev1.EnvVar{
			{
				Name:  dptypes.DPBackupName,
				Value: r.Backup.Name,
			},
			{
				Name:  dptypes.DPBackupDIR,
				Value: dptypes.BackupPathBase + r.BackupPolicy.Spec.PathPrefix,
			},
			{
				Name:  dptypes.DPTargetPodName,
				Value: targetPod.Name,
			},
			{
				Name:  dptypes.DPTTL,
				Value: r.Spec.RetentionPeriod.String(),
			},
		}
		envVars = append(envVars, utils.BuildEnvVarsByCredential(r.BackupPolicy.Spec.Target.ConnectionCredential)...)
		if r.ActionSet != nil {
			envVars = append(envVars, r.ActionSet.Spec.Env...)
		}
		envVars = append(envVars, r.BackupMethod.Env...)
		return envVars
	}

	buildVolumes := func() []corev1.Volume {
		volumes := []corev1.Volume{buildBackupRepoVolume(r.BackupRepoPVC.Name)}
		volumes = append(volumes,
			getVolumesByVolumeInfo(targetPod, r.BackupMethod.TargetVolumes)...)
		return volumes
	}

	buildVolumeMounts := func() []corev1.VolumeMount {
		volumeMounts := []corev1.VolumeMount{buildBackupRepoVolumeMount(r.BackupRepoPVC.Name)}
		if r.BackupMethod.TargetVolumes != nil {
			volumeMounts = append(volumeMounts, r.BackupMethod.TargetVolumes.VolumeMounts...)
		}
		return volumeMounts
	}

	builder := &action.JobActionBuilder{
		Name:            name,
		Env:             buildEnv(),
		Job:             job,
		Volumes:         buildVolumes(),
		VolumeMounts:    buildVolumeMounts(),
		RuntimeSettings: r.BackupMethod.RuntimeSettings,
	}
	if r.ActionSet != nil {
		builder.EnvFrom = r.ActionSet.Spec.EnvFrom
	}
	if boolptr.IsSetToTrue(job.ScheduleToTargetPodNode) {
		builder.NodeName = targetPod.Spec.NodeName
	}
	return builder.Build(), nil
}
