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

package restore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// --- helpers ---

func newTestRestore() *dpv1alpha1.Restore {
	return &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-restore",
			Namespace: "default",
			UID:       types.UID("12345678-abcd-efgh-ijkl-123456789012"),
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{
				Name:      "test-backup",
				Namespace: "default",
			},
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{},
		},
	}
}

func newTestBackup() *dpv1alpha1.Backup {
	return &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-backup",
			Namespace: "default",
		},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted,
			BackupMethod: &dpv1alpha1.BackupMethod{
				Name: "test-method",
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
					VolumeMounts: []corev1.VolumeMount{
						{Name: "data", MountPath: "/data"},
					},
				},
			},
		},
	}
}

func newTestActionSet() *dpv1alpha1.ActionSet {
	return &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-actionset",
		},
		Spec: dpv1alpha1.ActionSetSpec{
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
						Image:   "restore-image:latest",
						Command: []string{"restore.sh"},
					},
				},
			},
		},
	}
}

func newTestBackupActionSet() BackupActionSet {
	return BackupActionSet{
		Backup:    newTestBackup(),
		ActionSet: newTestActionSet(),
	}
}

func newTestBuilder() *restoreJobBuilder {
	return newRestoreJobBuilder(newTestRestore(), newTestBackupActionSet(), nil, dpv1alpha1.PrepareData)
}

// --- newRestoreJobBuilder ---

func TestNewRestoreJobBuilder(t *testing.T) {
	restore := newTestRestore()
	backupSet := newTestBackupActionSet()
	b := newRestoreJobBuilder(restore, backupSet, nil, dpv1alpha1.PrepareData)

	assert.Equal(t, restore, b.restore)
	assert.Equal(t, dpv1alpha1.PrepareData, b.stage)
	assert.NotNil(t, b.commonVolumes)
	assert.NotNil(t, b.commonVolumeMounts)
	assert.NotNil(t, b.labels)
	assert.Equal(t, "test-restore", b.labels[DataProtectionRestoreLabelKey])
}

// --- buildPVCVolumeAndMount ---

func TestBuildPVCVolumeAndMount_WithMountPath(t *testing.T) {
	b := newTestBuilder()
	claim := dpv1alpha1.VolumeConfig{
		MountPath: "/custom/mount",
	}
	vol, mount, err := b.buildPVCVolumeAndMount(claim, "my-pvc", "dp-claim")
	require.NoError(t, err)
	require.NotNil(t, vol)
	require.NotNil(t, mount)
	assert.Equal(t, "/custom/mount", mount.MountPath)
	assert.Contains(t, vol.Name, "dp-claim")
}

func TestBuildPVCVolumeAndMount_WithVolumeSourceMatch(t *testing.T) {
	b := newTestBuilder()
	claim := dpv1alpha1.VolumeConfig{
		VolumeSource: "data",
	}
	vol, mount, err := b.buildPVCVolumeAndMount(claim, "my-pvc", "dp-claim")
	require.NoError(t, err)
	require.NotNil(t, vol)
	require.NotNil(t, mount)
	assert.Equal(t, "/data", mount.MountPath)
}

func TestBuildPVCVolumeAndMount_VolumeSnapshotNoActionSet(t *testing.T) {
	b := newTestBuilder()
	b.backupSet.UseVolumeSnapshot = true
	b.backupSet.ActionSet.Spec.Restore.PrepareData = nil
	claim := dpv1alpha1.VolumeConfig{
		VolumeSource: "nonexistent",
	}
	vol, mount, err := b.buildPVCVolumeAndMount(claim, "my-pvc", "dp-claim")
	require.NoError(t, err)
	assert.Nil(t, vol)
	assert.Nil(t, mount)
}

func TestBuildPVCVolumeAndMount_ErrorOnMissingMount(t *testing.T) {
	b := newTestBuilder()
	claim := dpv1alpha1.VolumeConfig{
		VolumeSource: "nonexistent",
	}
	_, _, err := b.buildPVCVolumeAndMount(claim, "my-pvc", "dp-claim")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find the mountPath")
}

// --- addToCommonVolumesAndMounts ---

func TestAddToCommonVolumesAndMounts(t *testing.T) {
	b := newTestBuilder()
	vol := &corev1.Volume{Name: "v1"}
	mount := &corev1.VolumeMount{Name: "v1", MountPath: "/mnt"}
	b.addToCommonVolumesAndMounts(vol, mount)

	assert.Len(t, b.commonVolumes, 1)
	assert.Len(t, b.commonVolumeMounts, 1)
}

func TestAddToCommonVolumesAndMounts_NilSafe(t *testing.T) {
	b := newTestBuilder()
	b.addToCommonVolumesAndMounts(nil, nil)
	assert.Empty(t, b.commonVolumes)
	assert.Empty(t, b.commonVolumeMounts)
}

// --- resetSpecificVolumesAndMounts ---

func TestResetSpecificVolumesAndMounts(t *testing.T) {
	b := newTestBuilder()
	b.specificVolumes = []corev1.Volume{{Name: "v"}}
	b.specificVolumeMounts = []corev1.VolumeMount{{Name: "vm"}}
	b.resetSpecificVolumesAndMounts()
	assert.Empty(t, b.specificVolumes)
	assert.Empty(t, b.specificVolumeMounts)
}

// --- resetSpecificLabels ---

func TestResetSpecificLabels(t *testing.T) {
	b := newTestBuilder()
	b.labels["custom"] = "value"
	b.resetSpecificLabels()
	assert.Equal(t, "test-restore", b.labels[DataProtectionRestoreLabelKey])
	_, hasCustom := b.labels["custom"]
	assert.False(t, hasCustom)
}

// --- addToSpecificVolumesAndMounts ---

func TestAddToSpecificVolumesAndMounts(t *testing.T) {
	b := newTestBuilder()
	vol := &corev1.Volume{Name: "sv"}
	mount := &corev1.VolumeMount{Name: "sv", MountPath: "/s"}
	b.addToSpecificVolumesAndMounts(vol, mount)
	assert.Len(t, b.specificVolumes, 1)
	assert.Len(t, b.specificVolumeMounts, 1)
}

func TestAddToSpecificVolumesAndMounts_NilSafe(t *testing.T) {
	b := newTestBuilder()
	b.addToSpecificVolumesAndMounts(nil, nil)
	assert.Empty(t, b.specificVolumes)
	assert.Empty(t, b.specificVolumeMounts)
}

// --- fluent setters ---

func TestSetImage(t *testing.T) {
	b := newTestBuilder().setImage("my-image:v1")
	assert.Equal(t, "my-image:v1", b.image)
}

func TestSetCommand(t *testing.T) {
	b := newTestBuilder().setCommand([]string{"sh", "-c", "echo"})
	assert.Equal(t, []string{"sh", "-c", "echo"}, b.command)
}

func TestSetArgs(t *testing.T) {
	b := newTestBuilder().setArgs([]string{"--flag"})
	assert.Equal(t, []string{"--flag"}, b.args)
}

func TestSetToleration(t *testing.T) {
	tols := []corev1.Toleration{{Key: "k", Effect: corev1.TaintEffectNoSchedule}}
	b := newTestBuilder().setToleration(tols)
	assert.Equal(t, tols, b.tolerations)
}

func TestSetNodeNameToNodeSelector(t *testing.T) {
	b := newTestBuilder().setNodeNameToNodeSelector("node-1")
	assert.Equal(t, "node-1", b.nodeSelector[corev1.LabelHostname])
}

func TestSetJobName(t *testing.T) {
	b := newTestBuilder().setJobName("my-job")
	assert.Equal(t, "my-job", b.jobName)
}

func TestSetServiceAccount(t *testing.T) {
	b := newTestBuilder().setServiceAccount("sa-test")
	assert.Equal(t, "sa-test", b.serviceAccount)
}

func TestAttachBackupRepo(t *testing.T) {
	b := newTestBuilder().attachBackupRepo()
	assert.True(t, b.buildWithRepo)
}

// --- addLabel ---

func TestAddLabel(t *testing.T) {
	b := newTestBuilder()
	b.addLabel("new-key", "new-val")
	assert.Equal(t, "new-val", b.labels["new-key"])
}

func TestAddLabel_DoesNotOverwrite(t *testing.T) {
	b := newTestBuilder()
	b.addLabel(DataProtectionRestoreLabelKey, "overwritten")
	assert.Equal(t, "test-restore", b.labels[DataProtectionRestoreLabelKey])
}

func TestAddLabel_NilLabels(t *testing.T) {
	b := newTestBuilder()
	b.labels = nil
	b.addLabel("k", "v")
	assert.Equal(t, "v", b.labels["k"])
}

// --- addCommonEnv ---

func TestAddCommonEnv_Basic(t *testing.T) {
	b := newTestBuilder()
	b.backupSet.Backup.Status.Path = "/backup/path"
	b.addCommonEnv("pod-0")

	found := map[string]string{}
	for _, e := range b.env {
		found[e.Name] = e.Value
	}
	assert.Equal(t, "test-backup", found[dptypes.DPBackupName])
}

func TestAddCommonEnv_WithBaseBackup(t *testing.T) {
	b := newTestBuilder()
	b.backupSet.BaseBackup = &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "base-backup"},
		Status: dpv1alpha1.BackupStatus{
			StartTimestamp:      &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
			CompletionTimestamp: &metav1.Time{Time: time.Now()},
		},
	}
	b.addCommonEnv("")

	found := map[string]string{}
	for _, e := range b.env {
		found[e.Name] = e.Value
	}
	assert.Equal(t, "base-backup", found[dptypes.DPBaseBackupName])
}

func TestAddCommonEnv_WithAncestorIncrementalBackups(t *testing.T) {
	b := newTestBuilder()
	b.backupSet.AncestorIncrementalBackups = []*dpv1alpha1.Backup{
		{ObjectMeta: metav1.ObjectMeta{Name: "inc-1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "inc-2"}},
	}
	b.addCommonEnv("")

	found := map[string]string{}
	for _, e := range b.env {
		found[e.Name] = e.Value
	}
	assert.Equal(t, "inc-1,inc-2", found[dptypes.DPAncestorIncrementalBackupNames])
}

func TestAddCommonEnv_WithRestoreTime(t *testing.T) {
	b := newTestBuilder()
	b.restore.Spec.RestoreTime = "2024-01-15T10:30:00Z"
	b.addCommonEnv("")

	found := map[string]string{}
	for _, e := range b.env {
		found[e.Name] = e.Value
	}
	assert.NotEmpty(t, found[DPRestoreTime])
	assert.NotEmpty(t, found[DPRestoreTimestamp])
}

func TestAddCommonEnv_MergesBackupMethodEnv(t *testing.T) {
	b := newTestBuilder()
	b.backupSet.Backup.Status.BackupMethod = &dpv1alpha1.BackupMethod{
		Name: "method",
		Env: []corev1.EnvVar{
			{Name: "METHOD_VAR", Value: "method_val"},
		},
	}
	b.addCommonEnv("")

	found := map[string]string{}
	for _, e := range b.env {
		found[e.Name] = e.Value
	}
	assert.Equal(t, "method_val", found["METHOD_VAR"])
}

func TestAddCommonEnv_MergesRestoreSpecEnv(t *testing.T) {
	b := newTestBuilder()
	b.restore.Spec.Env = []corev1.EnvVar{
		{Name: "RESTORE_VAR", Value: "restore_val"},
	}
	b.addCommonEnv("")

	found := map[string]string{}
	for _, e := range b.env {
		found[e.Name] = e.Value
	}
	assert.Equal(t, "restore_val", found["RESTORE_VAR"])
}

// --- addTargetPodAndCredentialEnv ---

func TestAddTargetPodAndCredentialEnv_NilPod(t *testing.T) {
	b := newTestBuilder()
	result := b.addTargetPodAndCredentialEnv(nil, nil, nil)
	assert.Equal(t, b, result)
}

func TestAddTargetPodAndCredentialEnv_WithPodNoCredential(t *testing.T) {
	b := newTestBuilder()
	b.env = []corev1.EnvVar{} // init
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Subdomain: "svc",
			Containers: []corev1.Container{
				{
					Name: "main",
					Env:  []corev1.EnvVar{{Name: "POD_ENV", Value: "pod_val"}},
					Ports: []corev1.ContainerPort{
						{ContainerPort: 3306},
					},
				},
			},
		},
	}
	b.addTargetPodAndCredentialEnv(pod, nil, &dpv1alpha1.BackupTarget{})

	found := map[string]string{}
	for _, e := range b.env {
		found[e.Name] = e.Value
	}
	assert.NotEmpty(t, found[dptypes.DPDBHost])
	assert.NotEmpty(t, found[dptypes.DPDBPort])
}

func TestAddTargetPodAndCredentialEnv_WithCredential(t *testing.T) {
	b := newTestBuilder()
	b.env = []corev1.EnvVar{}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main"}},
		},
	}
	cred := &dpv1alpha1.ConnectionCredential{
		SecretName:  "my-secret",
		UsernameKey: "user",
		PasswordKey: "pass",
		PortKey:     "port",
		HostKey:     "host",
	}
	b.addTargetPodAndCredentialEnv(pod, cred, &dpv1alpha1.BackupTarget{})

	var hasDBUser bool
	for _, e := range b.env {
		if e.Name == dptypes.DPDBUser && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
			assert.Equal(t, "my-secret", e.ValueFrom.SecretKeyRef.Name)
			assert.Equal(t, "user", e.ValueFrom.SecretKeyRef.Key)
			hasDBUser = true
		}
	}
	assert.True(t, hasDBUser)
}

func TestAddTargetPodAndCredentialEnv_CredentialWithoutPortKey(t *testing.T) {
	b := newTestBuilder()
	b.env = []corev1.EnvVar{}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "main",
				Ports: []corev1.ContainerPort{{ContainerPort: 5432}},
			}},
		},
	}
	cred := &dpv1alpha1.ConnectionCredential{
		SecretName:  "my-secret",
		UsernameKey: "user",
		PasswordKey: "pass",
	}
	b.addTargetPodAndCredentialEnv(pod, cred, &dpv1alpha1.BackupTarget{})
	// should fall back to addDBPortEnv
	assert.NotEmpty(t, b.env)
}

func TestAddTargetPodAndCredentialEnv_CredentialWithoutHostKey(t *testing.T) {
	b := newTestBuilder()
	b.env = []corev1.EnvVar{}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main"}},
		},
	}
	cred := &dpv1alpha1.ConnectionCredential{
		SecretName:  "my-secret",
		UsernameKey: "user",
		PasswordKey: "pass",
		PortKey:     "port",
	}
	b.addTargetPodAndCredentialEnv(pod, cred, &dpv1alpha1.BackupTarget{})

	var hasDBHost bool
	for _, e := range b.env {
		if e.Name == dptypes.DPDBHost {
			hasDBHost = true
		}
	}
	assert.True(t, hasDBHost)
}

// --- builderRestoreJobName ---

func TestBuilderRestoreJobName(t *testing.T) {
	b := newTestBuilder()
	name := b.builderRestoreJobName(0)
	assert.Contains(t, name, "restore-preparedata")
	assert.Contains(t, name, "12345678")
	assert.Contains(t, name, "test-backup")
}

func TestBuilderRestoreJobName_PostReady(t *testing.T) {
	b := newTestBuilder()
	b.stage = dpv1alpha1.PostReady
	name := b.builderRestoreJobName(3)
	assert.Contains(t, name, "restore-postready")
}

// --- build ---

func TestBuild_BasicJob(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newTestBuilder()
	b.setImage("restore-image:v1").
		setCommand([]string{"restore.sh"}).
		setServiceAccount("my-sa").
		addCommonEnv("")

	job := b.build()
	require.NotNil(t, job)
	assert.Equal(t, "default", job.Namespace)
	assert.NotEmpty(t, job.Name)
	assert.Equal(t, "test-restore", job.Labels[DataProtectionRestoreLabelKey])

	// check backoffLimit defaults
	require.NotNil(t, job.Spec.BackoffLimit)
	assert.Equal(t, int32(1), *job.Spec.BackoffLimit)

	// check restore container
	require.Len(t, job.Spec.Template.Spec.Containers, 2)
	restoreContainer := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, Restore, restoreContainer.Name)
	assert.Equal(t, []string{"restore.sh"}, restoreContainer.Command)

	// check manager container injected
	managerContainer := job.Spec.Template.Spec.Containers[1]
	assert.Equal(t, restoreManagerContainerName, managerContainer.Name)

	// check service account
	assert.Equal(t, "my-sa", job.Spec.Template.Spec.ServiceAccountName)

	// check security context
	require.NotNil(t, job.Spec.Template.Spec.SecurityContext)
	require.NotNil(t, job.Spec.Template.Spec.SecurityContext.RunAsUser)
	assert.Equal(t, int64(0), *job.Spec.Template.Spec.SecurityContext.RunAsUser)

	// check restart policy
	assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)

	// check finalizer
	assert.Contains(t, job.Finalizers, dptypes.DataProtectionFinalizerName)
}

func TestBuild_CustomBackoffLimit(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newTestBuilder()
	limit := int32(5)
	b.restore.Spec.BackoffLimit = &limit
	b.addCommonEnv("")

	job := b.build()
	require.NotNil(t, job.Spec.BackoffLimit)
	assert.Equal(t, int32(5), *job.Spec.BackoffLimit)
}

func TestBuild_PrepareDataSchedulingSpec(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newTestBuilder()
	b.restore.Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
		SchedulingSpec: dpv1alpha1.SchedulingSpec{
			NodeSelector: map[string]string{"zone": "us-east-1"},
			Tolerations: []corev1.Toleration{
				{Key: "dedicated", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}
	b.addCommonEnv("")
	job := b.build()
	assert.Equal(t, map[string]string{"zone": "us-east-1"}, job.Spec.Template.Spec.NodeSelector)
	assert.Len(t, job.Spec.Template.Spec.Tolerations, 1)
}

func TestBuild_PostReadyUsesBuilderTolerations(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newRestoreJobBuilder(newTestRestore(), newTestBackupActionSet(), nil, dpv1alpha1.PostReady)
	tols := []corev1.Toleration{{Key: "k1", Effect: corev1.TaintEffectNoExecute}}
	b.setToleration(tols).
		setNodeNameToNodeSelector("node-5").
		addCommonEnv("")

	job := b.build()
	assert.Equal(t, tols, job.Spec.Template.Spec.Tolerations)
	assert.Equal(t, "node-5", job.Spec.Template.Spec.NodeSelector[corev1.LabelHostname])
}

func TestBuild_WithBackupExtras(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newTestBuilder()
	b.backupSet.Backup.Status.Extras = []map[string]string{
		{"extra-key": "extra-val"},
	}
	b.addCommonEnv("")
	job := b.build()

	assert.NotNil(t, job.Spec.Template.ObjectMeta.Annotations)
	assert.Contains(t, job.Spec.Template.ObjectMeta.Annotations, DataProtectionBackupExtrasLabelKey)

	// check downward volume added
	var foundDownward bool
	for _, v := range job.Spec.Template.Spec.Volumes {
		if v.Name == "downward-volume" {
			foundDownward = true
		}
	}
	assert.True(t, foundDownward)
}

func TestBuild_WithExplicitJobName(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newTestBuilder()
	b.setJobName("explicit-name").addCommonEnv("")
	job := b.build()
	assert.Equal(t, "explicit-name", job.Name)
}

func TestBuild_VolumesIncludeCommonAndSpecific(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newTestBuilder()
	b.addToCommonVolumesAndMounts(
		&corev1.Volume{Name: "common-vol"},
		&corev1.VolumeMount{Name: "common-vol", MountPath: "/c"},
	)
	b.addToSpecificVolumesAndMounts(
		&corev1.Volume{Name: "specific-vol"},
		&corev1.VolumeMount{Name: "specific-vol", MountPath: "/s"},
	)
	b.addCommonEnv("")

	job := b.build()
	volNames := map[string]bool{}
	for _, v := range job.Spec.Template.Spec.Volumes {
		volNames[v.Name] = true
	}
	assert.True(t, volNames["common-vol"])
	assert.True(t, volNames["specific-vol"])

	mountNames := map[string]bool{}
	for _, m := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
		mountNames[m.Name] = true
	}
	assert.True(t, mountNames["common-vol"])
	assert.True(t, mountNames["specific-vol"])
}

// --- InjectManagerContainer ---

func TestInjectManagerContainer(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newTestBuilder()
	podSpec := &corev1.PodSpec{}
	b.InjectManagerContainer(podSpec)

	require.Len(t, podSpec.Containers, 1)
	c := podSpec.Containers[0]
	assert.Equal(t, restoreManagerContainerName, c.Name)
	assert.Equal(t, "apecloud/kubeblocks-tools:latest", c.Image)
	assert.Equal(t, []string{"sh", "-c"}, c.Command)
	assert.Len(t, c.Args, 1)
	assert.Contains(t, c.Args[0], "stop_restore_manager")

	// check downward volume was added
	require.Len(t, podSpec.Volumes, 1)
	assert.Equal(t, "downward-volume-sidecard", podSpec.Volumes[0].Name)
	require.NotNil(t, podSpec.Volumes[0].VolumeSource.DownwardAPI)
}

// --- build with repo (datasafed injection) ---

func TestBuild_WithPVCFallback(t *testing.T) {
	viper.Set(constant.KBToolsImage, "apecloud/kubeblocks-tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	b := newTestBuilder()
	b.backupSet.Backup.Status.PersistentVolumeClaimName = "backup-pvc"
	b.buildWithRepo = true
	b.addCommonEnv("")

	job := b.build()
	require.NotNil(t, job)
	// datasafed injection adds an init container or modifies volumes
	// Just verify the job was built without error
	assert.NotEmpty(t, job.Name)
}
