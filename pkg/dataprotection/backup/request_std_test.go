/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/action"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// --- GetBackupType ---

func TestGetBackupType_WithActionSet(t *testing.T) {
	r := &Request{
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeFull,
			},
		},
	}
	assert.Equal(t, string(dpv1alpha1.BackupTypeFull), r.GetBackupType())
}

func TestGetBackupType_SnapshotVolumes(t *testing.T) {
	trueVal := true
	r := &Request{
		BackupMethod: &dpv1alpha1.BackupMethod{
			SnapshotVolumes: &trueVal,
		},
	}
	assert.Equal(t, string(dpv1alpha1.BackupTypeFull), r.GetBackupType())
}

func TestGetBackupType_NoActionSetNoSnapshot(t *testing.T) {
	r := &Request{}
	assert.Equal(t, "", r.GetBackupType())
}

func TestGetBackupType_SnapshotVolumesFalse(t *testing.T) {
	r := &Request{
		BackupMethod: &dpv1alpha1.BackupMethod{
			SnapshotVolumes: boolptr.False(),
		},
	}
	assert.Equal(t, "", r.GetBackupType())
}

func TestGetBackupType_ActionSetTakesPrecedence(t *testing.T) {
	trueVal := true
	r := &Request{
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeIncremental,
			},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{
			SnapshotVolumes: &trueVal,
		},
	}
	assert.Equal(t, string(dpv1alpha1.BackupTypeIncremental), r.GetBackupType())
}

// --- getActionTargetPrefix ---

func TestGetActionTargetPrefix_WithTarget(t *testing.T) {
	r := &Request{
		Target: &dpv1alpha1.BackupTarget{Name: "target-1"},
	}
	assert.Equal(t, "target-1-", r.getActionTargetPrefix())
}

func TestGetActionTargetPrefix_EmptyTarget(t *testing.T) {
	r := &Request{
		Target: &dpv1alpha1.BackupTarget{Name: ""},
	}
	assert.Equal(t, "", r.getActionTargetPrefix())
}

func TestGetActionTargetPrefix_NilTarget(t *testing.T) {
	r := &Request{}
	assert.Equal(t, "", r.getActionTargetPrefix())
}

// --- backupActionSetExists ---

func TestBackupActionSetExists_True(t *testing.T) {
	r := &Request{
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				Backup: &dpv1alpha1.BackupActionSpec{},
			},
		},
	}
	assert.True(t, r.backupActionSetExists())
}

func TestBackupActionSetExists_NilActionSet(t *testing.T) {
	r := &Request{}
	assert.False(t, r.backupActionSetExists())
}

func TestBackupActionSetExists_NilBackup(t *testing.T) {
	r := &Request{
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{},
		},
	}
	assert.False(t, r.backupActionSetExists())
}

// --- buildAction ---

func TestBuildAction_NoExecNoJob(t *testing.T) {
	r := newMinimalRequest()
	act := &dpv1alpha1.ActionSpec{}
	_, err := r.buildAction(&corev1.Pod{}, "test-action", act)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has no exec or job")
}

func TestBuildAction_BothExecAndJob(t *testing.T) {
	r := newMinimalRequest()
	act := &dpv1alpha1.ActionSpec{
		Exec: &dpv1alpha1.ExecActionSpec{Command: []string{"echo"}},
		Job:  &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "img"}},
	}
	_, err := r.buildAction(&corev1.Pod{}, "test-action", act)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "should have only one")
}

func TestBuildAction_Exec(t *testing.T) {
	r := newMinimalRequest()
	act := &dpv1alpha1.ActionSpec{
		Exec: &dpv1alpha1.ExecActionSpec{
			Command:   []string{"echo", "hello"},
			Container: "main",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "img"}},
		},
	}
	a, err := r.buildAction(pod, "test-exec", act)
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, "test-exec", a.GetName())
}

// --- buildExecAction ---

func TestBuildExecAction_DefaultContainer(t *testing.T) {
	r := newMinimalRequest()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "first-container", Image: "img"}},
		},
	}
	exec := &dpv1alpha1.ExecActionSpec{
		Command:   []string{"pg_dump"},
		Container: "",
	}
	a := r.buildExecAction(pod, "test-exec", exec)
	require.NotNil(t, a)
	assert.Equal(t, "test-exec", a.GetName())
}

func TestBuildExecAction_ExplicitContainer(t *testing.T) {
	r := newMinimalRequest()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "ns1"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "sidecar"},
				{Name: "db"},
			},
		},
	}
	exec := &dpv1alpha1.ExecActionSpec{
		Command:   []string{"backup"},
		Container: "db",
	}
	a := r.buildExecAction(pod, "test-exec", exec)
	require.NotNil(t, a)
}

// --- buildPreBackupActions / buildPostBackupActions ---

func TestBuildPreBackupActions_NoActionSet(t *testing.T) {
	r := &Request{}
	var podActions []action_noop
	err := r.buildPreBackupActions(nil, &corev1.Pod{}, 0)
	require.NoError(t, err)
	_ = podActions
}

func TestBuildPostBackupActions_NoActionSet(t *testing.T) {
	r := &Request{}
	err := r.buildPostBackupActions(nil, &corev1.Pod{}, 0)
	require.NoError(t, err)
}

func TestBuildPreBackupActions_EmptyPreBackup(t *testing.T) {
	r := &Request{
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				Backup: &dpv1alpha1.BackupActionSpec{
					PreBackup: []dpv1alpha1.ActionSpec{},
				},
			},
		},
	}
	err := r.buildPreBackupActions(nil, &corev1.Pod{}, 0)
	require.NoError(t, err)
}

func TestBuildPostBackupActions_EmptyPostBackup(t *testing.T) {
	r := &Request{
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				Backup: &dpv1alpha1.BackupActionSpec{
					PostBackup: []dpv1alpha1.ActionSpec{},
				},
			},
		},
	}
	err := r.buildPostBackupActions(nil, &corev1.Pod{}, 0)
	require.NoError(t, err)
}

// --- buildSyncProgressCommand ---

func TestBuildSyncProgressCommand(t *testing.T) {
	r := newMinimalRequest()
	cmd := r.buildSyncProgressCommand()
	assert.Contains(t, cmd, dptypes.DPBackupInfoFile)
	assert.Contains(t, cmd, dptypes.DPCheckInterval)
	assert.Contains(t, cmd, "default")    // namespace
	assert.Contains(t, cmd, "test-backup") // backup name
	assert.Contains(t, cmd, "kubectl")
	assert.Contains(t, cmd, "datasafed push")
}

// --- buildContinuousSyncProgressCommand ---

func TestBuildContinuousSyncProgressCommand(t *testing.T) {
	r := newMinimalRequest()
	cmd := r.buildContinuousSyncProgressCommand()
	assert.Contains(t, cmd, dptypes.DPBackupInfoFile)
	assert.Contains(t, cmd, dptypes.DPCheckInterval)
	assert.Contains(t, cmd, "default")
	assert.Contains(t, cmd, "test-backup")
	assert.Contains(t, cmd, "retryTimes")
}

// --- InjectManagerContainer ---

func TestInjectManagerContainer_DefaultInterval(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := newMinimalRequest()
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:    "worker",
				Image:   "busybox",
				Command: []string{"sh"},
				Env: []corev1.EnvVar{
					{Name: "FOO", Value: "bar"},
				},
			},
		},
	}
	r.InjectManagerContainer(podSpec, nil, "echo hello")
	require.Len(t, podSpec.Containers, 2)
	mgr := podSpec.Containers[1]
	assert.Equal(t, managerContainerName, mgr.Name)
	assert.Equal(t, "apecloud/kubeblocks-tools:latest", mgr.Image)
	assert.Equal(t, []string{"sh", "-c"}, mgr.Command)
	assert.Equal(t, []string{"echo hello"}, mgr.Args)

	// check default interval
	found := false
	for _, env := range mgr.Env {
		if env.Name == dptypes.DPCheckInterval {
			assert.Equal(t, "5", env.Value)
			found = true
		}
	}
	assert.True(t, found, "DPCheckInterval env should be present")
}

func TestInjectManagerContainer_CustomInterval(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := newMinimalRequest()
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "worker", Image: "busybox"},
		},
	}
	interval := int32(30)
	sync := &dpv1alpha1.SyncProgress{IntervalSeconds: &interval}
	r.InjectManagerContainer(podSpec, sync, "cmd")
	mgr := podSpec.Containers[1]
	for _, env := range mgr.Env {
		if env.Name == dptypes.DPCheckInterval {
			assert.Equal(t, "30", env.Value)
		}
	}
}

func TestInjectManagerContainer_ZeroInterval(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := newMinimalRequest()
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "worker", Image: "busybox"},
		},
	}
	interval := int32(0)
	sync := &dpv1alpha1.SyncProgress{IntervalSeconds: &interval}
	r.InjectManagerContainer(podSpec, sync, "cmd")
	mgr := podSpec.Containers[1]
	for _, env := range mgr.Env {
		if env.Name == dptypes.DPCheckInterval {
			// zero is not > 0, so should use default 5
			assert.Equal(t, "5", env.Value)
		}
	}
}

// --- buildBackupDataAction ---

func TestBuildBackupDataAction_NoActionSet(t *testing.T) {
	r := &Request{}
	a, err := r.buildBackupDataAction(&corev1.Pod{}, "test")
	require.NoError(t, err)
	assert.Nil(t, a)
}

func TestBuildBackupDataAction_NilBackupData(t *testing.T) {
	r := &Request{
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				Backup: &dpv1alpha1.BackupActionSpec{
					BackupData: nil,
				},
			},
		},
	}
	a, err := r.buildBackupDataAction(&corev1.Pod{}, "test")
	require.NoError(t, err)
	assert.Nil(t, a)
}

// --- buildCreateVolumeSnapshotAction ---

func TestBuildCreateVolumeSnapshotAction_NoSnapshotVolumes(t *testing.T) {
	r := &Request{
		BackupMethod: &dpv1alpha1.BackupMethod{
			SnapshotVolumes: boolptr.False(),
		},
	}
	a, err := r.buildCreateVolumeSnapshotAction(&corev1.Pod{}, "test", 0)
	require.NoError(t, err)
	assert.Nil(t, a)
}

func TestBuildCreateVolumeSnapshotAction_NilMethod(t *testing.T) {
	r := &Request{}
	a, err := r.buildCreateVolumeSnapshotAction(&corev1.Pod{}, "test", 0)
	require.NoError(t, err)
	assert.Nil(t, a)
}

func TestBuildCreateVolumeSnapshotAction_MissingTargetVolumes(t *testing.T) {
	trueVal := true
	r := &Request{
		BackupMethod: &dpv1alpha1.BackupMethod{
			SnapshotVolumes: &trueVal,
			TargetVolumes:   nil,
		},
	}
	_, err := r.buildCreateVolumeSnapshotAction(&corev1.Pod{}, "test", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "targetVolumes is required")
}

// --- BuildActions ---

func TestBuildActions_NoPods(t *testing.T) {
	r := &Request{
		TargetPods: []*corev1.Pod{},
	}
	actions, err := r.BuildActions()
	require.NoError(t, err)
	assert.Empty(t, actions)
}

// --- BuildActions with pods ---

func TestBuildActions_WithPodAndActionSet(t *testing.T) {
	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bk-1",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		Target: &dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{},
		},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		BackupRepo: &dpv1alpha1.BackupRepo{
			Status: dpv1alpha1.BackupRepoStatus{BackupPVCName: "repo-pvc"},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeFull,
				Backup: &dpv1alpha1.BackupActionSpec{
					BackupData: &dpv1alpha1.BackupDataActionSpec{
						JobActionSpec: dpv1alpha1.JobActionSpec{
							BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
								Image:   "backup:v1",
								Command: []string{"backup"},
							},
						},
					},
				},
			},
		},
		TargetPods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "app:v1", Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
					},
				},
			},
		},
	}

	actions, err := r.BuildActions()
	require.NoError(t, err)
	require.Contains(t, actions, "pod-0")
	assert.Len(t, actions["pod-0"], 1) // just the backup data action
}

func TestBuildActions_WithPreAndPostBackup(t *testing.T) {
	viper.Set(constant.KBToolsImage, "tools:latest")
	viper.Set(constant.CfgKeyCtrlrMgrNS, "kb-system")
	defer func() {
		viper.Set(constant.KBToolsImage, "")
		viper.Set(constant.CfgKeyCtrlrMgrNS, "")
	}()

	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bk-1",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		Target: &dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{},
		},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		BackupRepo: &dpv1alpha1.BackupRepo{
			Status: dpv1alpha1.BackupRepoStatus{BackupPVCName: "repo-pvc"},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeFull,
				Backup: &dpv1alpha1.BackupActionSpec{
					BackupData: &dpv1alpha1.BackupDataActionSpec{
						JobActionSpec: dpv1alpha1.JobActionSpec{
							BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "backup:v1", Command: []string{"backup"}},
						},
					},
					PreBackup: []dpv1alpha1.ActionSpec{
						{Exec: &dpv1alpha1.ExecActionSpec{Command: []string{"pre-hook"}}},
					},
					PostBackup: []dpv1alpha1.ActionSpec{
						{Exec: &dpv1alpha1.ExecActionSpec{Command: []string{"post-hook"}}},
					},
				},
			},
		},
		TargetPods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main", Image: "app:v1", Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
					},
				},
			},
		},
	}

	actions, err := r.BuildActions()
	require.NoError(t, err)
	require.Contains(t, actions, "pod-0")
	assert.Len(t, actions["pod-0"], 3) // pre + data + post
}

// --- Constants ---

func TestRequestConstants(t *testing.T) {
	assert.Equal(t, "dp-backup", BackupDataJobNamePrefix)
	assert.Equal(t, "backupdata", BackupDataContainerName)
}

// --- helpers ---

// action_noop is needed just to declare variable type for the unused test setup
type action_noop = interface{}

func newMinimalRequest() *Request {
	return &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backup",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
	}
}

// --- BuildJobActionPodSpec ---

func TestBuildJobActionPodSpec_Basic(t *testing.T) {
	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bk-1",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels: map[string]string{
					constant.AppInstanceLabelKey: "my-cluster",
				},
			},
			Spec: dpv1alpha1.BackupSpec{
				RetentionPeriod: "24h",
			},
		},
		Target: &dpv1alpha1.BackupTarget{
			Name:        "tgt",
			PodSelector: &dpv1alpha1.PodSelector{},
		},
		BackupPolicy: &dpv1alpha1.BackupPolicy{
			Spec: dpv1alpha1.BackupPolicySpec{
				PathPrefix: "policy-prefix",
			},
		},
		BackupRepo: &dpv1alpha1.BackupRepo{
			Spec: dpv1alpha1.BackupRepoSpec{
				PathPrefix: "repo-prefix",
			},
			Status: dpv1alpha1.BackupRepoStatus{
				BackupPVCName: "repo-pvc",
			},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
	}

	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-0",
			Namespace: "default",
			Labels:    map[string]string{constant.RoleLabelKey: "leader"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "app:v1",
					Ports: []corev1.ContainerPort{{ContainerPort: 3306}},
				},
			},
		},
	}

	job := &dpv1alpha1.JobActionSpec{
		BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
			Image:   "backup-tool:v1",
			Command: []string{"backup", "--full"},
		},
	}

	podSpec, err := r.BuildJobActionPodSpec(targetPod, "backup-container", job)
	require.NoError(t, err)
	require.NotNil(t, podSpec)
	require.Len(t, podSpec.Containers, 1)

	container := podSpec.Containers[0]
	assert.Equal(t, "backup-container", container.Name)
	assert.Equal(t, []string{"backup", "--full"}, container.Command)
	assert.Equal(t, corev1.RestartPolicyNever, podSpec.RestartPolicy)

	// check envs
	envMap := map[string]string{}
	for _, e := range container.Env {
		envMap[e.Name] = e.Value
	}
	assert.Equal(t, "bk-1", envMap[dptypes.DPBackupName])
	assert.Equal(t, "pod-0", envMap[dptypes.DPTargetPodName])
	assert.Equal(t, "leader", envMap[dptypes.DPTargetPodRole])
	assert.Contains(t, envMap[dptypes.DPBackupBasePath], "bk-1")
	assert.Equal(t, "default", envMap[constant.KBEnvNamespace])
	assert.Equal(t, "my-cluster", envMap[constant.KBEnvClusterName])

	// check volumes include manager-shared
	found := false
	for _, v := range podSpec.Volumes {
		if v.Name == managerSharedVolumeName {
			found = true
		}
	}
	assert.True(t, found)
}

func TestBuildJobActionPodSpec_RunOnTargetNode(t *testing.T) {
	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	trueVal := true
	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bk-1",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		Target: &dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{},
		},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		BackupRepo: &dpv1alpha1.BackupRepo{
			Status: dpv1alpha1.BackupRepoStatus{BackupPVCName: "repo-pvc"},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{
			TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
				Volumes: []string{"data"},
			},
		},
	}

	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Containers: []corev1.Container{
				{Name: "main", Image: "app:v1", Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
			},
			Tolerations: []corev1.Toleration{{Key: "special", Effect: corev1.TaintEffectNoSchedule}},
			Volumes: []corev1.Volume{
				{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
	}

	job := &dpv1alpha1.JobActionSpec{
		BaseJobActionSpec:  dpv1alpha1.BaseJobActionSpec{Image: "tool:v1", Command: []string{"backup"}},
		RunOnTargetPodNode: &trueVal,
	}

	podSpec, err := r.BuildJobActionPodSpec(targetPod, "container", job)
	require.NoError(t, err)
	assert.Equal(t, "node-1", podSpec.NodeSelector[corev1.LabelHostname])
	assert.Len(t, podSpec.Tolerations, 1)
	assert.Equal(t, "special", podSpec.Tolerations[0].Key)
}

func TestBuildJobActionPodSpec_WithIncrementalBackup(t *testing.T) {
	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bk-incr",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		Target: &dpv1alpha1.BackupTarget{
			Name:        "tgt",
			PodSelector: &dpv1alpha1.PodSelector{},
		},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		BackupRepo: &dpv1alpha1.BackupRepo{
			Status: dpv1alpha1.BackupRepoStatus{BackupPVCName: "repo-pvc"},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
		ParentBackup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: "parent-bk"},
		},
		BaseBackup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{Name: "base-bk"},
		},
	}

	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Image: "app:v1", Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
			},
		},
	}

	job := &dpv1alpha1.JobActionSpec{
		BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "tool:v1", Command: []string{"backup"}},
	}

	podSpec, err := r.BuildJobActionPodSpec(targetPod, "container", job)
	require.NoError(t, err)
	envMap := map[string]string{}
	for _, e := range podSpec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	assert.Equal(t, "parent-bk", envMap[dptypes.DPParentBackupName])
	assert.Equal(t, "base-bk", envMap[dptypes.DPBaseBackupName])
}

func TestBuildJobActionPodSpec_WithRuntimeSettings(t *testing.T) {
	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bk-1",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		Target: &dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{},
		},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		BackupRepo: &dpv1alpha1.BackupRepo{
			Status: dpv1alpha1.BackupRepoStatus{BackupPVCName: "repo-pvc"},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{
			RuntimeSettings: &dpv1alpha1.RuntimeSettings{
				Resources: corev1.ResourceRequirements{},
			},
		},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				Env: []corev1.EnvVar{{Name: "EXTRA", Value: "val"}},
			},
		},
	}

	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Image: "app:v1", Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
			},
		},
	}

	job := &dpv1alpha1.JobActionSpec{
		BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "tool:v1", Command: []string{"backup"}},
	}

	podSpec, err := r.BuildJobActionPodSpec(targetPod, "container", job)
	require.NoError(t, err)
	require.NotNil(t, podSpec)
	// actionSet envs should be present
	envMap := map[string]string{}
	for _, e := range podSpec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	assert.Equal(t, "val", envMap["EXTRA"])
}

// --- buildJobAction ---

func TestBuildJobAction(t *testing.T) {
	r := newMinimalRequest()
	r.BackupPolicy = &dpv1alpha1.BackupPolicy{}
	r.BackupRepo = &dpv1alpha1.BackupRepo{
		Status: dpv1alpha1.BackupRepoStatus{BackupPVCName: "repo-pvc"},
	}
	r.BackupMethod = &dpv1alpha1.BackupMethod{}
	r.Target = &dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{}}

	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main", Image: "app:v1", Ports: []corev1.ContainerPort{{ContainerPort: 3306}}},
			},
		},
	}
	job := &dpv1alpha1.JobActionSpec{
		BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "tool:v1", Command: []string{"backup"}},
	}
	a, err := r.buildJobAction(targetPod, "test-job", job)
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.Equal(t, "test-job", a.GetName())
}

func TestBuildBackupDataAction_UnsupportedType(t *testing.T) {
	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bk",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupType("unsupported"),
				Backup: &dpv1alpha1.BackupActionSpec{
					BackupData: &dpv1alpha1.BackupDataActionSpec{},
				},
			},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		Target:       &dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{}},
		BackupRepo:   &dpv1alpha1.BackupRepo{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c", Image: "img"}},
		},
	}
	_, err := r.buildBackupDataAction(pod, "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("unsupported backup type %s", "unsupported"))
}

func TestBuildBackupDataAction_FullType(t *testing.T) {
	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backup",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeFull,
				Backup: &dpv1alpha1.BackupActionSpec{
					BackupData: &dpv1alpha1.BackupDataActionSpec{
						JobActionSpec: dpv1alpha1.JobActionSpec{
							BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
								Image:   "backup-img:latest",
								Command: []string{"backup"},
							},
						},
					},
				},
			},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		Target:       &dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{}},
		BackupRepo:   &dpv1alpha1.BackupRepo{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c", Image: "img"}},
		},
	}
	act, err := r.buildBackupDataAction(pod, "data-action")
	require.NoError(t, err)
	require.NotNil(t, act)
}

func TestBuildBackupDataAction_ContinuousType(t *testing.T) {
	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backup",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeContinuous,
				Backup: &dpv1alpha1.BackupActionSpec{
					BackupData: &dpv1alpha1.BackupDataActionSpec{
						JobActionSpec: dpv1alpha1.JobActionSpec{
							BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
								Image:   "backup-img:latest",
								Command: []string{"backup"},
							},
						},
					},
				},
			},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		Target:       &dpv1alpha1.BackupTarget{Name: "target-0", PodSelector: &dpv1alpha1.PodSelector{}},
		BackupRepo:   &dpv1alpha1.BackupRepo{},
		TargetPods: []*corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "c", Image: "img"}},
				},
			},
		},
	}
	pod := r.TargetPods[0]
	act, err := r.buildBackupDataAction(pod, "data-action")
	require.NoError(t, err)
	require.NotNil(t, act)
}

// --- buildPreBackupActions / buildPostBackupActions with non-empty actions ---

func TestBuildPreBackupActions_WithExecActions(t *testing.T) {
	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backup",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeFull,
				Backup: &dpv1alpha1.BackupActionSpec{
					PreBackup: []dpv1alpha1.ActionSpec{
						{
							Exec: &dpv1alpha1.ExecActionSpec{
								Command: []string{"pre-check"},
							},
						},
					},
				},
			},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		Target:       &dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{}},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c", Image: "img"}},
		},
	}
	var podActions []action.Action
	err := r.buildPreBackupActions(&podActions, pod, 0)
	require.NoError(t, err)
	assert.Len(t, podActions, 1)
}

func TestBuildPostBackupActions_WithExecActions(t *testing.T) {
	r := &Request{
		Backup: &dpv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-backup",
				Namespace: "default",
				UID:       "12345678-abcd",
				Labels:    map[string]string{},
			},
		},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{
				BackupType: dpv1alpha1.BackupTypeFull,
				Backup: &dpv1alpha1.BackupActionSpec{
					PostBackup: []dpv1alpha1.ActionSpec{
						{
							Exec: &dpv1alpha1.ExecActionSpec{
								Command: []string{"post-check"},
							},
						},
					},
				},
			},
		},
		BackupMethod: &dpv1alpha1.BackupMethod{},
		BackupPolicy: &dpv1alpha1.BackupPolicy{},
		Target:       &dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{}},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c", Image: "img"}},
		},
	}
	var podActions []action.Action
	err := r.buildPostBackupActions(&podActions, pod, 0)
	require.NoError(t, err)
	assert.Len(t, podActions, 1)
}
