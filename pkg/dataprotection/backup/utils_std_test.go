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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func utilsTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	return s
}

func newTestPod(name, ns string, volumes []corev1.Volume, containers []corev1.Container) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			Volumes:    volumes,
			Containers: containers,
		},
	}
}

// --- getVolumesByNames ---

func TestGetVolumesByNames_Empty(t *testing.T) {
	pod := newTestPod("p", "default", nil, nil)
	result := getVolumesByNames(pod, []string{"data"})
	assert.Empty(t, result)
}

func TestGetVolumesByNames_Match(t *testing.T) {
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data"},
		{Name: "log"},
		{Name: "config"},
	}, nil)
	result := getVolumesByNames(pod, []string{"data", "config"})
	require.Len(t, result, 2)
	assert.Equal(t, "data", result[0].Name)
	assert.Equal(t, "config", result[1].Name)
}

func TestGetVolumesByNames_NoMatch(t *testing.T) {
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data"},
	}, nil)
	result := getVolumesByNames(pod, []string{"missing"})
	assert.Empty(t, result)
}

// --- getVolumesByMounts ---

func TestGetVolumesByMounts_Empty(t *testing.T) {
	pod := newTestPod("p", "default", nil, nil)
	result := getVolumesByMounts(pod, []corev1.VolumeMount{{Name: "data"}})
	assert.Empty(t, result)
}

func TestGetVolumesByMounts_Match(t *testing.T) {
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data"},
		{Name: "log"},
	}, nil)
	mounts := []corev1.VolumeMount{
		{Name: "data", MountPath: "/data"},
		{Name: "missing", MountPath: "/missing"},
	}
	result := getVolumesByMounts(pod, mounts)
	require.Len(t, result, 1)
	assert.Equal(t, "data", result[0].Name)
}

// --- getVolumesByVolumeInfo ---

func TestGetVolumesByVolumeInfo_Nil(t *testing.T) {
	pod := newTestPod("p", "default", nil, nil)
	result := getVolumesByVolumeInfo(pod, nil)
	assert.Nil(t, result)
}

func TestGetVolumesByVolumeInfo_ByVolumes(t *testing.T) {
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data"},
		{Name: "log"},
	}, nil)
	info := &dpv1alpha1.TargetVolumeInfo{
		Volumes: []string{"data"},
	}
	result := getVolumesByVolumeInfo(pod, info)
	require.Len(t, result, 1)
	assert.Equal(t, "data", result[0].Name)
}

func TestGetVolumesByVolumeInfo_ByVolumeMounts(t *testing.T) {
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data"},
		{Name: "log"},
	}, nil)
	info := &dpv1alpha1.TargetVolumeInfo{
		VolumeMounts: []corev1.VolumeMount{
			{Name: "log", MountPath: "/log"},
		},
	}
	result := getVolumesByVolumeInfo(pod, info)
	require.Len(t, result, 1)
	assert.Equal(t, "log", result[0].Name)
}

func TestGetVolumesByVolumeInfo_VolumesPreferred(t *testing.T) {
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data"},
		{Name: "log"},
	}, nil)
	info := &dpv1alpha1.TargetVolumeInfo{
		Volumes:      []string{"data"},
		VolumeMounts: []corev1.VolumeMount{{Name: "log", MountPath: "/log"}},
	}
	// When both are set, Volumes takes precedence
	result := getVolumesByVolumeInfo(pod, info)
	require.Len(t, result, 1)
	assert.Equal(t, "data", result[0].Name)
}

func TestGetVolumesByVolumeInfo_EmptyInfo(t *testing.T) {
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data"},
	}, nil)
	info := &dpv1alpha1.TargetVolumeInfo{}
	result := getVolumesByVolumeInfo(pod, info)
	assert.Empty(t, result)
}

// --- getVolumeMountsByVolumeInfo ---

func TestGetVolumeMountsByVolumeInfo_Nil(t *testing.T) {
	pod := newTestPod("p", "default", nil, nil)
	result := getVolumeMountsByVolumeInfo(pod, nil)
	assert.Nil(t, result)
}

func TestGetVolumeMountsByVolumeInfo_EmptyMounts(t *testing.T) {
	pod := newTestPod("p", "default", nil, nil)
	info := &dpv1alpha1.TargetVolumeInfo{}
	result := getVolumeMountsByVolumeInfo(pod, info)
	assert.Nil(t, result)
}

func TestGetVolumeMountsByVolumeInfo_Match(t *testing.T) {
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data"},
		{Name: "log"},
	}, nil)
	info := &dpv1alpha1.TargetVolumeInfo{
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
			{Name: "missing", MountPath: "/missing"},
		},
	}
	result := getVolumeMountsByVolumeInfo(pod, info)
	require.Len(t, result, 1)
	assert.Equal(t, "data", result[0].Name)
	assert.Equal(t, "/data", result[0].MountPath)
}

// --- getPVCsByVolumeNames ---

func TestGetPVCsByVolumeNames_EmptyNames(t *testing.T) {
	cli := fake.NewClientBuilder().WithScheme(utilsTestScheme()).Build()
	pod := newTestPod("p", "default", nil, nil)
	result, err := getPVCsByVolumeNames(cli, pod, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestGetPVCsByVolumeNames_NoPVCVolume(t *testing.T) {
	cli := fake.NewClientBuilder().WithScheme(utilsTestScheme()).Build()
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}, nil)
	result, err := getPVCsByVolumeNames(cli, pod, []string{"data"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetPVCsByVolumeNames_Found(t *testing.T) {
	scheme := utilsTestScheme()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "default"},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"},
		}},
	}, nil)
	result, err := getPVCsByVolumeNames(cli, pod, []string{"data"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "data", result[0].VolumeName)
	assert.Equal(t, "data-pvc", result[0].PersistentVolumeClaim.Name)
}

func TestGetPVCsByVolumeNames_NotFound(t *testing.T) {
	cli := fake.NewClientBuilder().WithScheme(utilsTestScheme()).Build()
	pod := newTestPod("p", "default", []corev1.Volume{
		{Name: "data", VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "missing-pvc"},
		}},
	}, nil)
	_, err := getPVCsByVolumeNames(cli, pod, []string{"data"})
	require.Error(t, err)
}

// --- excludeLabelsForWorkload ---

func TestExcludeLabelsForWorkload(t *testing.T) {
	labels := excludeLabelsForWorkload()
	require.Len(t, labels, 1)
	assert.Equal(t, constant.KBAppComponentLabelKey, labels[0])
}

// --- BuildBackupWorkloadLabels ---

func TestBuildBackupWorkloadLabels_Basic(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-backup",
			Labels: map[string]string{
				"app":  "mysql",
				"tier": "db",
			},
		},
	}
	labels := BuildBackupWorkloadLabels(backup)
	assert.Equal(t, "mysql", labels["app"])
	assert.Equal(t, "db", labels["tier"])
	assert.Equal(t, "my-backup", labels[dptypes.BackupNameLabelKey])
}

func TestBuildBackupWorkloadLabels_ExcludesComponentLabel(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-backup",
			Labels: map[string]string{
				constant.KBAppComponentLabelKey: "mysql-comp",
				"keep-me":                      "yes",
			},
		},
	}
	labels := BuildBackupWorkloadLabels(backup)
	_, exists := labels[constant.KBAppComponentLabelKey]
	assert.False(t, exists)
	assert.Equal(t, "yes", labels["keep-me"])
	assert.Equal(t, "my-backup", labels[dptypes.BackupNameLabelKey])
}

func TestBuildBackupWorkloadLabels_NilLabels(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup"},
	}
	labels := BuildBackupWorkloadLabels(backup)
	assert.Equal(t, "my-backup", labels[dptypes.BackupNameLabelKey])
}

// --- buildBackupJobObjMeta ---

func TestBuildBackupJobObjMeta(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "ns1",
			UID:       types.UID("12345678-abcd-efgh-ijkl-mnopqrstuvwx"),
			Labels:    map[string]string{"env": "test"},
		},
	}
	meta := buildBackupJobObjMeta(backup, "dp-backup")
	assert.Equal(t, "dp-backup-bk-1-12345678", meta.Name)
	assert.Equal(t, "ns1", meta.Namespace)
	assert.Equal(t, "bk-1", meta.Labels[dptypes.BackupNameLabelKey])
}

// --- GenerateBackupJobName ---

func TestGenerateBackupJobName_Short(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bk",
			UID:  types.UID("12345678-abcd"),
		},
	}
	name := GenerateBackupJobName(backup, "dp-backup")
	assert.Equal(t, "dp-backup-bk-12345678", name)
}

func TestGenerateBackupJobName_Truncated(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "very-long-backup-name-that-exceeds-the-limit-for-labels",
			UID:  types.UID("12345678-abcd"),
		},
	}
	name := GenerateBackupJobName(backup, "dp-backup")
	assert.LessOrEqual(t, len(name), 63)
	assert.False(t, name[len(name)-1] == '-')
}

// --- GenerateBackupStatefulSetName ---

func TestGenerateBackupStatefulSetName_NoTarget(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1"},
	}
	name := GenerateBackupStatefulSetName(backup, "", "dp-backup")
	assert.Equal(t, "bk-1", name)
}

func TestGenerateBackupStatefulSetName_WithTarget(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1"},
	}
	name := GenerateBackupStatefulSetName(backup, "target-1", "dp-backup")
	assert.Equal(t, "dp-backup-target-1-bk-1", name)
}

func TestGenerateBackupStatefulSetName_Truncated(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "very-long-backup-name-that-is-really-long"},
	}
	name := GenerateBackupStatefulSetName(backup, "target-name-also-long", "dp-backup")
	assert.LessOrEqual(t, len(name), 52)
	assert.False(t, name[len(name)-1] == '-')
}

// --- generateBaseCRNameByBackupSchedule ---

func TestGenerateBaseCRNameByBackupSchedule_Short(t *testing.T) {
	name := generateBaseCRNameByBackupSchedule("uid-sched", "ns1", "full")
	assert.Equal(t, "uid-sched-ns1-full", name)
}

func TestGenerateBaseCRNameByBackupSchedule_LongTruncated(t *testing.T) {
	name := generateBaseCRNameByBackupSchedule("very-long-unique-name-with-bs", "long-namespace-name", "method")
	// first part (unique+ns) truncated to 30 chars
	parts := name[:len(name)-len("-method")]
	assert.LessOrEqual(t, len(parts), 30)
}

// --- GenerateCRNameByBackupSchedule ---

func TestGenerateCRNameByBackupSchedule_NoOwner(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sched-1",
			Namespace: "ns1",
			UID:       types.UID("abcdefgh-1234-5678-9abc-def012345678"),
		},
	}
	name := GenerateCRNameByBackupSchedule(schedule, "full")
	assert.Contains(t, name, "abcdefgh")
	assert.Contains(t, name, "sched-1")
	assert.Contains(t, name, "full")
}

func TestGenerateCRNameByBackupSchedule_WithOwner(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sched-1",
			Namespace: "ns1",
			UID:       types.UID("abcdefgh-1234-5678-9abc-def012345678"),
			OwnerReferences: []metav1.OwnerReference{
				{UID: types.UID("owneruid-1234-5678-9abc-def012345678")},
			},
		},
	}
	name := GenerateCRNameByBackupSchedule(schedule, "full")
	assert.Contains(t, name, "owneruid")
	assert.NotContains(t, name, "abcdefgh")
}

// --- GenerateCRNameByScheduleNameAndMethod ---

func TestGenerateCRNameByScheduleNameAndMethod_WithName(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sched-1",
			Namespace: "ns1",
			UID:       types.UID("abcdefgh-1234-5678-9abc-def012345678"),
		},
	}
	name := GenerateCRNameByScheduleNameAndMethod(schedule, "full", "custom-name")
	assert.Contains(t, name, "custom-name")
}

func TestGenerateCRNameByScheduleNameAndMethod_EmptyName(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sched-1",
			Namespace: "ns1",
			UID:       types.UID("abcdefgh-1234-5678-9abc-def012345678"),
		},
	}
	name := GenerateCRNameByScheduleNameAndMethod(schedule, "full", "")
	assert.Contains(t, name, "full")
}

// --- GenerateLegacyCRNameByBackupSchedule ---

func TestGenerateLegacyCRNameByBackupSchedule(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sched-1",
			Namespace: "ns1",
			UID:       types.UID("abcdefgh-1234-5678-9abc-def012345678"),
		},
	}
	name := GenerateLegacyCRNameByBackupSchedule(schedule, "snap")
	assert.Contains(t, name, "abcdefgh")
	assert.Contains(t, name, "sched-1")
	assert.Contains(t, name, "snap")
}

// --- BuildBaseBackupPath ---

func TestBuildBaseBackupPath(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1", Namespace: "ns1"},
	}
	path := BuildBaseBackupPath(backup, "repo-prefix", "path-prefix")
	assert.Equal(t, "/repo-prefix/ns1/path-prefix/bk-1", path)
}

func TestBuildBaseBackupPath_SlashTrimming(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1", Namespace: "ns1"},
	}
	path := BuildBaseBackupPath(backup, "/repo/", "/prefix/")
	assert.Equal(t, "/repo/ns1/prefix/bk-1", path)
}

// --- BuildBackupRootPath ---

func TestBuildBackupRootPath(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1"},
	}
	path := BuildBackupRootPath(backup, "repo", "prefix")
	assert.Equal(t, "/repo/ns1/prefix", path)
}

func TestBuildBackupRootPath_EmptyPrefixes(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1"},
	}
	path := BuildBackupRootPath(backup, "", "")
	assert.Equal(t, "/ns1", path)
}

// --- BuildBackupPathByTarget ---

func TestBuildBackupPathByTarget_WithTargetAndPod(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1", Namespace: "ns1"},
	}
	target := &dpv1alpha1.BackupTarget{
		Name: "target-1",
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: dpv1alpha1.PodSelectionStrategyAll,
		},
	}
	path := BuildBackupPathByTarget(backup, target, "repo", "prefix", "pod-0")
	assert.Equal(t, "/repo/ns1/prefix/bk-1/target-1/pod-0", path)
}

func TestBuildBackupPathByTarget_NoTargetName(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk-1", Namespace: "ns1"},
	}
	target := &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: dpv1alpha1.PodSelectionStrategyAny,
		},
	}
	path := BuildBackupPathByTarget(backup, target, "repo", "prefix", "pod-0")
	assert.Equal(t, "/repo/ns1/prefix/bk-1", path)
}

// --- BuildTargetRelativePath ---

func TestBuildTargetRelativePath_StrategyAll(t *testing.T) {
	target := &dpv1alpha1.BackupTarget{
		Name: "tgt",
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: dpv1alpha1.PodSelectionStrategyAll,
		},
	}
	path := BuildTargetRelativePath(target, "pod-0")
	assert.Equal(t, "tgt/pod-0", path)
}

func TestBuildTargetRelativePath_StrategyAny(t *testing.T) {
	target := &dpv1alpha1.BackupTarget{
		Name: "tgt",
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: dpv1alpha1.PodSelectionStrategyAny,
		},
	}
	path := BuildTargetRelativePath(target, "pod-0")
	assert.Equal(t, "tgt", path)
}

func TestBuildTargetRelativePath_NoName(t *testing.T) {
	target := &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: dpv1alpha1.PodSelectionStrategyAll,
		},
	}
	path := BuildTargetRelativePath(target, "pod-0")
	assert.Equal(t, "pod-0", path)
}

func TestBuildTargetRelativePath_NoNameNoAll(t *testing.T) {
	target := &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: dpv1alpha1.PodSelectionStrategyAny,
		},
	}
	path := BuildTargetRelativePath(target, "pod-0")
	assert.Equal(t, "", path)
}

// --- BuildKopiaRepoPath ---

func TestBuildKopiaRepoPath(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1"},
	}
	path := BuildKopiaRepoPath(backup, "repo", "prefix")
	assert.Equal(t, "/repo/ns1/prefix/kopia", path)
}

func TestBuildKopiaRepoPath_SlashTrimming(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns1"},
	}
	path := BuildKopiaRepoPath(backup, "/repo/", "/prefix/")
	assert.Equal(t, "/repo/ns1/prefix/kopia", path)
}

// --- GetSchedulePolicyByMethod ---

func TestGetSchedulePolicyByMethod_Found(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{
		Spec: dpv1alpha1.BackupScheduleSpec{
			Schedules: []dpv1alpha1.SchedulePolicy{
				{BackupMethod: "full", CronExpression: "0 0 * * *"},
				{BackupMethod: "incr", CronExpression: "*/15 * * * *"},
			},
		},
	}
	policy := GetSchedulePolicyByMethod(schedule, "incr")
	require.NotNil(t, policy)
	assert.Equal(t, "*/15 * * * *", policy.CronExpression)
}

func TestGetSchedulePolicyByMethod_NotFound(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{
		Spec: dpv1alpha1.BackupScheduleSpec{
			Schedules: []dpv1alpha1.SchedulePolicy{
				{BackupMethod: "full"},
			},
		},
	}
	policy := GetSchedulePolicyByMethod(schedule, "missing")
	assert.Nil(t, policy)
}

func TestGetSchedulePolicyByMethod_EmptySchedules(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{}
	policy := GetSchedulePolicyByMethod(schedule, "full")
	assert.Nil(t, policy)
}

// --- StopStatefulSetsWhenFailed ---

func TestStopStatefulSetsWhenFailed_NotFailed(t *testing.T) {
	cli := fake.NewClientBuilder().WithScheme(utilsTestScheme()).Build()
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk", Namespace: "default"},
		Status:     dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseRunning},
	}
	err := StopStatefulSetsWhenFailed(context.Background(), cli, backup, "")
	assert.NoError(t, err)
}

func TestStopStatefulSetsWhenFailed_StsNotFound(t *testing.T) {
	cli := fake.NewClientBuilder().WithScheme(utilsTestScheme()).Build()
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk", Namespace: "default"},
		Status:     dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseFailed},
	}
	err := StopStatefulSetsWhenFailed(context.Background(), cli, backup, "")
	assert.NoError(t, err)
}

func TestStopStatefulSetsWhenFailed_ScalesDown(t *testing.T) {
	scheme := utilsTestScheme()
	replicas := int32(1)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk", // GenerateBackupStatefulSetName with empty target returns backup.Name
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "bk", Namespace: "default"},
		Status:     dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseFailed},
	}
	err := StopStatefulSetsWhenFailed(context.Background(), cli, backup, "")
	require.NoError(t, err)

	updated := &appsv1.StatefulSet{}
	err = cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "bk"}, updated)
	require.NoError(t, err)
	assert.Equal(t, int32(0), *updated.Spec.Replicas)
}

// --- BuildParametersManifest ---

func TestBuildParametersManifest_Empty(t *testing.T) {
	result, err := BuildParametersManifest(nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestBuildParametersManifest_WithParams(t *testing.T) {
	params := []dpv1alpha1.ParameterPair{
		{Name: "key1", Value: "val1"},
	}
	result, err := BuildParametersManifest(params)
	require.NoError(t, err)
	assert.Contains(t, result, "parameters:")
	assert.Contains(t, result, `"key1"`)
	assert.Contains(t, result, `"val1"`)
}
