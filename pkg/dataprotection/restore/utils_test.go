/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestBuildRestoreLabelsKeepsRestoreNameForControllerLookup(t *testing.T) {
	restoreName := "restore-name-that-controller-uses-for-label-filtering"

	labels := BuildRestoreLabels(restoreName)
	if labels[DataProtectionRestoreLabelKey] != restoreName {
		t.Fatalf("restore label = %q, want %q", labels[DataProtectionRestoreLabelKey], restoreName)
	}
	if labels[constant.AppManagedByLabelKey] != dptypes.AppName {
		t.Fatalf("managed-by label = %q, want %q", labels[constant.AppManagedByLabelKey], dptypes.AppName)
	}
}

func TestBuildPVCVolumeAndMountShortensDerivedVolumeName(t *testing.T) {
	builder := &restoreJobBuilder{
		restore: &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "restore"}},
		backupSet: BackupActionSet{
			Backup: &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup"}},
		},
	}
	claim := dpv1alpha1.VolumeConfig{
		MountPath: "/data",
	}
	claimName := "very-long-claim-name-with-extra-suffix-for-restore-prepare-data"
	identifier := "restore-prepare-data-job-with-an-overlength-derived-volume-identifier"

	volume, volumeMount, err := builder.buildPVCVolumeAndMount(claim, claimName, identifier)
	if err != nil {
		t.Fatalf("buildPVCVolumeAndMount returned error: %v", err)
	}
	if volume == nil || volumeMount == nil {
		t.Fatalf("expected volume and volumeMount to be created")
		return
	}
	if len(volume.Name) > constant.KubeNameMaxLength {
		t.Fatalf("volume name length = %d, want <= %d", len(volume.Name), constant.KubeNameMaxLength)
	}
	if volume.Name != volumeMount.Name {
		t.Fatalf("volume name %q and volumeMount name %q should match", volume.Name, volumeMount.Name)
	}
	if volumeMount.MountPath != claim.MountPath {
		t.Fatalf("volumeMount path = %q, want %q", volumeMount.MountPath, claim.MountPath)
	}
	wantSource := corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName}}
	if volume.VolumeSource.PersistentVolumeClaim == nil || volume.VolumeSource.PersistentVolumeClaim.ClaimName != wantSource.PersistentVolumeClaim.ClaimName {
		t.Fatalf("volume claim name = %q, want %q", volume.VolumeSource.PersistentVolumeClaim.ClaimName, claimName)
	}
}

func TestRestoreBuilderCommonVolumesAndSourceMountPath(t *testing.T) {
	builder := &restoreJobBuilder{
		backupSet: BackupActionSet{Backup: &dpv1alpha1.Backup{Status: dpv1alpha1.BackupStatus{BackupMethod: &dpv1alpha1.BackupMethod{TargetVolumes: &dpv1alpha1.TargetVolumeInfo{VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}}}}},
	}
	assert.Equal(t, "/data", getMountPathWithSourceVolume(builder.backupSet.Backup, "data"))
	assert.Empty(t, getMountPathWithSourceVolume(builder.backupSet.Backup, "missing"))

	volume := &corev1.Volume{Name: "data"}
	mount := &corev1.VolumeMount{Name: "data", MountPath: "/data"}
	assert.Same(t, builder, builder.addToCommonVolumesAndMounts(volume, mount))
	assert.Equal(t, []corev1.Volume{*volume}, builder.commonVolumes)
	assert.Equal(t, []corev1.VolumeMount{*mount}, builder.commonVolumeMounts)
	builder.addToCommonVolumesAndMounts(nil, nil)
	assert.Len(t, builder.commonVolumes, 1)
}

func TestRestoreConditionAndStatusHelpers(t *testing.T) {
	restore := &dpv1alpha1.Restore{}
	SetRestoreCheckBackupRepoCondition(restore, ReasonCheckBackupRepoSuccessfully, "ok")
	assert.Equal(t, metav1.ConditionTrue, restore.Status.Conditions[0].Status)
	assert.Equal(t, ConditionTypeRestoreCheckBackupRepo, restore.Status.Conditions[0].Type)

	SetRestoreValidationCondition(restore, "Invalid", "bad")
	assert.Equal(t, metav1.ConditionFalse, restore.Status.Conditions[1].Status)
	assert.Equal(t, ConditionTypeRestoreValidationPassed, restore.Status.Conditions[1].Type)

	SetRestoreStageCondition(restore, dpv1alpha1.PostReady, ReasonSucceed, "done")
	assert.Equal(t, metav1.ConditionTrue, restore.Status.Conditions[2].Status)
	assert.Equal(t, ConditionTypeRestorePostReady, restore.Status.Conditions[2].Type)

	actions := []dpv1alpha1.RestoreStatusAction{}
	SetRestoreStatusAction(&actions, dpv1alpha1.RestoreStatusAction{ObjectKey: "Job/job", Status: dpv1alpha1.RestoreActionProcessing})
	assert.Len(t, actions, 1)
	assert.NotNil(t, actions[0].StartTime)
	assert.Contains(t, actions[0].Message, "processing")

	SetRestoreStatusAction(&actions, dpv1alpha1.RestoreStatusAction{ObjectKey: "Job/job", Status: dpv1alpha1.RestoreActionCompleted})
	assert.Equal(t, dpv1alpha1.RestoreActionCompleted, actions[0].Status)
	assert.NotNil(t, actions[0].EndTime)
	assert.Contains(t, actions[0].Message, "successfully")
	assert.Nil(t, FindRestoreStatusAction(actions, "missing"))

	var nilActions *[]dpv1alpha1.RestoreStatusAction
	SetRestoreStatusAction(nilActions, dpv1alpha1.RestoreStatusAction{ObjectKey: "noop"})
}

func TestRestoreTimeAndPathHelpers(t *testing.T) {
	start := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	end := metav1.NewTime(start.Add(2 * time.Hour))
	assert.Equal(t, "2h0m0s", GetRestoreDuration(dpv1alpha1.RestoreStatus{StartTimestamp: &start, CompletionTimestamp: &end}).Duration.String())
	assert.Nil(t, GetRestoreDuration(dpv1alpha1.RestoreStatus{StartTimestamp: &start}))

	assert.Equal(t, time.RFC3339, getTimeFormat(nil))
	assert.Equal(t, "custom", getTimeFormat([]corev1.EnvVar{{Name: dptypes.DPTimeFormat, Value: "custom"}}))

	shifted, err := transformTimeWithZone(&start, "+08:30")
	assert.NoError(t, err)
	_, offset := shifted.Time.Zone()
	assert.Equal(t, 8*3600+30*60, offset)
	_, err = transformTimeWithZone(&start, "bad")
	assert.Error(t, err)

	assert.Equal(t, "Job/restore-job", BuildJobKeyForActionStatus("restore-job"))
	assert.Equal(t, "target/pod-0", GetTargetRelativePath("target", "pod-0"))
	assert.Equal(t, "pod-0", GetTargetRelativePath("", "pod-0"))
	assert.Empty(t, BackupFilePathEnv("", "target", "pod-0"))
	envs := BackupFilePathEnv("/repo/ns/path/backup", "target", "pod-0")
	assert.Equal(t, dptypes.DPTargetRelativePath, envs[0].Name)
	assert.Equal(t, "target/pod-0", envs[0].Value)

	short := cutJobName("short")
	assert.Equal(t, "short", short)
	long := cutJobName("restore-job-name-that-is-far-too-long-to-fit-in-the-kubernetes-job-name-limit")
	assert.LessOrEqual(t, len(long), 63)
}

func TestFormatRestoreTimeAndValidate(t *testing.T) {
	start := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	end := metav1.NewTime(start.Add(2 * time.Hour))
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "continuous", Namespace: "ns", Labels: map[string]string{constant.AppInstanceLabelKey: "cluster"}},
		Status:     dpv1alpha1.BackupStatus{TimeRange: &dpv1alpha1.BackupTimeRange{Start: &start, End: &end}},
	}

	got, err := FormatRestoreTimeAndValidate("2026-01-01T01:00:00Z", backup)
	assert.NoError(t, err)
	assert.Equal(t, "2026-01-01T01:00:00Z", got)

	got, err = FormatRestoreTimeAndValidate("Jan 01,2026 01:00:00 UTC+0000", backup)
	assert.NoError(t, err)
	assert.Equal(t, "2026-01-01T01:00:00Z", got)

	_, err = FormatRestoreTimeAndValidate("2026-01-02T00:00:00Z", backup)
	assert.Error(t, err)
	_, err = FormatRestoreTimeAndValidate("not a time", backup)
	assert.Error(t, err)
	assert.Equal(t, "", func() string { got, _ := FormatRestoreTimeAndValidate("", backup); return got }())
}

func TestRestoreSourcePodAndSnapshotHelpers(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget:       dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAny}},
		SelectedTargetPods: []string{"pod-0", "pod-1"},
	}
	podName, err := GetSourcePodNameFromTarget(target, nil, 0)
	assert.NoError(t, err)
	assert.Empty(t, podName)

	target.PodSelector.Strategy = dpv1alpha1.PodSelectionStrategyAll
	_, err = GetSourcePodNameFromTarget(target, nil, 0)
	assert.Error(t, err)
	podName, err = GetSourcePodNameFromTarget(target, &dpv1alpha1.RequiredPolicyForAllPodSelection{DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy}, 1)
	assert.NoError(t, err)
	assert.Equal(t, "pod-1", podName)
	podName, err = GetSourcePodNameFromTarget(target, &dpv1alpha1.RequiredPolicyForAllPodSelection{DataRestorePolicy: dpv1alpha1.OneToManyRestorePolicy, SourceOfOneToMany: &dpv1alpha1.SourceOfOneToMany{TargetPodName: "pod-0"}}, 5)
	assert.NoError(t, err)
	assert.Equal(t, "pod-0", podName)

	backup := &dpv1alpha1.Backup{Status: dpv1alpha1.BackupStatus{Actions: []dpv1alpha1.ActionStatus{
		{TargetPodName: "pod-x", VolumeSnapshots: []dpv1alpha1.VolumeSnapshotStatus{{VolumeName: "ignored", Name: "snap-x"}}},
		{TargetPodName: "pod-1", VolumeSnapshots: []dpv1alpha1.VolumeSnapshotStatus{{VolumeName: "data", Name: "snap-data"}}},
	}}}
	assert.Equal(t, map[string]string{"data": "snap-data"}, GetVolumeSnapshotsBySourcePod(backup, target, "pod-1"))
	assert.Nil(t, GetVolumeSnapshotsBySourcePod(backup, target, "missing"))
}

func TestValidateParentBackupSet(t *testing.T) {
	now := metav1.NewTime(time.Now())
	later := metav1.NewTime(now.Add(time.Hour))
	parent := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "parent"},
		Spec:       dpv1alpha1.BackupSpec{BackupPolicyName: "policy", BackupMethod: "full"},
		Status:     dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted, CompletionTimestamp: &now, BackupRepoName: "repo"},
	}
	child := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "child"},
		Spec:       dpv1alpha1.BackupSpec{BackupPolicyName: "policy", BackupMethod: "inc"},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted, CompletionTimestamp: &later, BackupRepoName: "repo",
			BackupMethod:   &dpv1alpha1.BackupMethod{CompatibleMethod: "full"},
			BaseBackupName: "parent",
		},
	}
	assert.NoError(t, ValidateParentBackupSet(
		&BackupActionSet{Backup: parent, ActionSet: &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull}}},
		&BackupActionSet{Backup: child},
	))

	child.Status.BackupRepoName = "other"
	assert.Error(t, ValidateParentBackupSet(
		&BackupActionSet{Backup: parent, ActionSet: &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull}}},
		&BackupActionSet{Backup: child},
	))
	assert.Error(t, ValidateParentBackupSet(&BackupActionSet{}, &BackupActionSet{Backup: child}))
}

func TestRestoreManagerBuildIncrementalAndDifferentialBackupSets(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))

	now := metav1.NewTime(time.Now())
	later := metav1.NewTime(now.Add(time.Hour))
	restoreSpec := &dpv1alpha1.RestoreActionSpec{PrepareData: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "busybox", Command: []string{"restore"}}}}
	fullActionSet := &dpv1alpha1.ActionSet{ObjectMeta: metav1.ObjectMeta{Name: "full-action"}, Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeFull, Restore: restoreSpec}}
	incrementalActionSet := &dpv1alpha1.ActionSet{ObjectMeta: metav1.ObjectMeta{Name: "inc-action"}, Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeIncremental, Restore: restoreSpec}}
	full := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "full", Namespace: "ns"},
		Spec:       dpv1alpha1.BackupSpec{BackupPolicyName: "policy", BackupMethod: "full"},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted, CompletionTimestamp: &now, BackupRepoName: "repo",
			BackupMethod: &dpv1alpha1.BackupMethod{Name: "full", ActionSetName: "full-action"},
		},
	}
	child := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "child", Namespace: "ns"},
		Spec:       dpv1alpha1.BackupSpec{BackupPolicyName: "policy", BackupMethod: "inc"},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted, CompletionTimestamp: &later, BackupRepoName: "repo",
			ParentBackupName: "full", BaseBackupName: "full",
			BackupMethod: &dpv1alpha1.BackupMethod{Name: "inc", ActionSetName: "inc-action", CompatibleMethod: "full"},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(fullActionSet, incrementalActionSet, full, child).Build()
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background(), Req: ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "ns", Name: "restore"}}}
	mgr := &RestoreManager{}

	err := mgr.BuildIncrementalBackupActionSet(reqCtx, cli, BackupActionSet{Backup: child, ActionSet: incrementalActionSet})
	assert.NoError(t, err)
	assert.Len(t, mgr.PrepareDataBackupSets, 1)
	assert.Equal(t, "full", mgr.PrepareDataBackupSets[0].BaseBackup.Name)

	mgr = &RestoreManager{}
	child.Spec.ParentBackupName = "full"
	err = mgr.BuildDifferentialBackupActionSets(reqCtx, cli, BackupActionSet{Backup: child, ActionSet: incrementalActionSet})
	assert.NoError(t, err)
	assert.Len(t, mgr.PrepareDataBackupSets, 2)

	_, err = mgr.GetBackupActionSetByNamespaced(reqCtx, cli, "missing", "ns")
	assert.Error(t, err)
}

func TestValidateAndInitRestoreMGRFullBackup(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))

	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "full-action"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
			Restore:    &dpv1alpha1.RestoreActionSpec{PrepareData: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "busybox", Command: []string{"restore"}}}},
		},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns"},
		Status: dpv1alpha1.BackupStatus{
			Phase:        dpv1alpha1.BackupPhaseCompleted,
			BackupMethod: &dpv1alpha1.BackupMethod{Name: "full", ActionSetName: "full-action"},
		},
	}
	restoreObj := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "ns"}, Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: "ns"}}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet, backup).Build()
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background(), Req: ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "ns", Name: "restore"}}}
	mgr := &RestoreManager{Restore: restoreObj}

	assert.NoError(t, ValidateAndInitRestoreMGR(reqCtx, cli, mgr))
	assert.Len(t, mgr.PrepareDataBackupSets, 1)

	backup.Status.Phase = dpv1alpha1.BackupPhaseFailed
	assert.NoError(t, cli.Update(context.Background(), backup))
	mgr = &RestoreManager{Restore: restoreObj}
	assert.Error(t, ValidateAndInitRestoreMGR(reqCtx, cli, mgr))
}

func TestRestoreManagerStopsManagerContainer(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, batchv1.AddToScheme(scheme))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "restore-job-pod", Namespace: "ns", Labels: map[string]string{"job-name": "restore-job"}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name:  Restore,
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}},
		}}},
	}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "restore-job", Namespace: "ns"}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod, job).Build()
	mgr := &RestoreManager{Client: cli}

	normalTerminated, err := mgr.CheckIfRestoreContainerTerminated(job)
	assert.NoError(t, err)
	assert.False(t, normalTerminated)

	got := &corev1.Pod{}
	assert.NoError(t, cli.Get(context.Background(), client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, got))
	assert.Equal(t, "true", got.Annotations[DataProtectionStopRestoreManagerAnnotationKey])
	assert.NoError(t, mgr.StopManagerContainer(got))
	assert.NoError(t, mgr.StopManagerContainerByJob(job))
}
