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
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func utilsTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	return s
}

func newTestReqCtx() intctrlutil.RequestCtx {
	return intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Req: reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test"}},
	}
}

// --- SetRestoreCondition ---

func TestSetRestoreCondition(t *testing.T) {
	restore := &dpv1alpha1.Restore{}
	SetRestoreCondition(restore, metav1.ConditionTrue, "TestType", "TestReason", "test message")
	require.Len(t, restore.Status.Conditions, 1)
	assert.Equal(t, "TestType", restore.Status.Conditions[0].Type)
	assert.Equal(t, "TestReason", restore.Status.Conditions[0].Reason)
	assert.Equal(t, metav1.ConditionTrue, restore.Status.Conditions[0].Status)
}

// --- SetRestoreCheckBackupRepoCondition ---

func TestSetRestoreCheckBackupRepoCondition_Success(t *testing.T) {
	restore := &dpv1alpha1.Restore{}
	SetRestoreCheckBackupRepoCondition(restore, ReasonCheckBackupRepoSuccessfully, "ok")
	require.Len(t, restore.Status.Conditions, 1)
	assert.Equal(t, metav1.ConditionTrue, restore.Status.Conditions[0].Status)
	assert.Equal(t, ConditionTypeRestoreCheckBackupRepo, restore.Status.Conditions[0].Type)
}

func TestSetRestoreCheckBackupRepoCondition_Failed(t *testing.T) {
	restore := &dpv1alpha1.Restore{}
	SetRestoreCheckBackupRepoCondition(restore, ReasonCheckBackupRepoFailed, "failed")
	require.Len(t, restore.Status.Conditions, 1)
	assert.Equal(t, metav1.ConditionFalse, restore.Status.Conditions[0].Status)
}

// --- SetRestoreValidationCondition ---

func TestSetRestoreValidationCondition_Success(t *testing.T) {
	restore := &dpv1alpha1.Restore{}
	SetRestoreValidationCondition(restore, ReasonValidateSuccessfully, "ok")
	require.Len(t, restore.Status.Conditions, 1)
	assert.Equal(t, metav1.ConditionTrue, restore.Status.Conditions[0].Status)
	assert.Equal(t, ConditionTypeRestoreValidationPassed, restore.Status.Conditions[0].Type)
}

func TestSetRestoreValidationCondition_Failed(t *testing.T) {
	restore := &dpv1alpha1.Restore{}
	SetRestoreValidationCondition(restore, ReasonValidateFailed, "bad")
	assert.Equal(t, metav1.ConditionFalse, restore.Status.Conditions[0].Status)
}

// --- SetRestoreStageCondition ---

func TestSetRestoreStageCondition_PrepareData(t *testing.T) {
	restore := &dpv1alpha1.Restore{}
	SetRestoreStageCondition(restore, dpv1alpha1.PrepareData, ReasonSucceed, "done")
	require.Len(t, restore.Status.Conditions, 1)
	assert.Equal(t, ConditionTypeRestorePreparedData, restore.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, restore.Status.Conditions[0].Status)
}

func TestSetRestoreStageCondition_PostReady(t *testing.T) {
	restore := &dpv1alpha1.Restore{}
	SetRestoreStageCondition(restore, dpv1alpha1.PostReady, ReasonFailed, "fail")
	require.Len(t, restore.Status.Conditions, 1)
	assert.Equal(t, ConditionTypeRestorePostReady, restore.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, restore.Status.Conditions[0].Status)
}

// --- FindRestoreStatusAction ---

func TestFindRestoreStatusAction_Found(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{
		{ObjectKey: "Job/job-1", Status: dpv1alpha1.RestoreActionCompleted},
		{ObjectKey: "Job/job-2", Status: dpv1alpha1.RestoreActionProcessing},
	}
	found := FindRestoreStatusAction(actions, "Job/job-2")
	require.NotNil(t, found)
	assert.Equal(t, dpv1alpha1.RestoreActionProcessing, found.Status)
}

func TestFindRestoreStatusAction_NotFound(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{
		{ObjectKey: "Job/job-1"},
	}
	assert.Nil(t, FindRestoreStatusAction(actions, "Job/nonexistent"))
}

// --- SetRestoreStatusAction ---

func TestSetRestoreStatusAction_NilSlice(t *testing.T) {
	// Should not panic on nil
	SetRestoreStatusAction(nil, dpv1alpha1.RestoreStatusAction{})
}

func TestSetRestoreStatusAction_NewAction(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{}
	statusAction := dpv1alpha1.RestoreStatusAction{
		ObjectKey: "Job/job-1",
		Status:    dpv1alpha1.RestoreActionProcessing,
	}
	SetRestoreStatusAction(&actions, statusAction)
	require.Len(t, actions, 1)
	assert.Contains(t, actions[0].Message, "is processing")
}

func TestSetRestoreStatusAction_CompletedAutoMessage(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{}
	SetRestoreStatusAction(&actions, dpv1alpha1.RestoreStatusAction{
		ObjectKey: "Job/job-1",
		Status:    dpv1alpha1.RestoreActionCompleted,
	})
	require.Len(t, actions, 1)
	assert.Contains(t, actions[0].Message, "successfully processed")
}

func TestSetRestoreStatusAction_FailedAutoMessage(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{}
	SetRestoreStatusAction(&actions, dpv1alpha1.RestoreStatusAction{
		ObjectKey: "Job/job-1",
		Status:    dpv1alpha1.RestoreActionFailed,
	})
	require.Len(t, actions, 1)
	assert.Contains(t, actions[0].Message, "is failed")
}

func TestSetRestoreStatusAction_UpdateExisting(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{
		{ObjectKey: "Job/job-1", Status: dpv1alpha1.RestoreActionProcessing, Message: "processing"},
	}
	SetRestoreStatusAction(&actions, dpv1alpha1.RestoreStatusAction{
		ObjectKey: "Job/job-1",
		Status:    dpv1alpha1.RestoreActionCompleted,
	})
	require.Len(t, actions, 1)
	assert.Equal(t, dpv1alpha1.RestoreActionCompleted, actions[0].Status)
}

func TestSetRestoreStatusAction_PreservesLogCollectorMessage(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{
		{ObjectKey: "Job/job-1", Status: dpv1alpha1.RestoreActionProcessing, Message: dptypes.LogCollectorOutput + " some logs"},
	}
	SetRestoreStatusAction(&actions, dpv1alpha1.RestoreStatusAction{
		ObjectKey: "Job/job-1",
		Status:    dpv1alpha1.RestoreActionCompleted,
	})
	assert.Contains(t, actions[0].Message, dptypes.LogCollectorOutput)
}

// --- GetRestoreActionsCountForPrepareData ---

func TestGetRestoreActionsCountForPrepareData_Nil(t *testing.T) {
	assert.Equal(t, 0, GetRestoreActionsCountForPrepareData(nil))
}

func TestGetRestoreActionsCountForPrepareData_NoTemplate(t *testing.T) {
	config := &dpv1alpha1.PrepareDataConfig{}
	assert.Equal(t, 1, GetRestoreActionsCountForPrepareData(config))
}

func TestGetRestoreActionsCountForPrepareData_WithTemplate(t *testing.T) {
	config := &dpv1alpha1.PrepareDataConfig{
		RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{
			Replicas: 3,
		},
	}
	assert.Equal(t, 3, GetRestoreActionsCountForPrepareData(config))
}

// --- GetRestoreDuration ---

func TestGetRestoreDuration_NilTimestamps(t *testing.T) {
	status := dpv1alpha1.RestoreStatus{}
	assert.Nil(t, GetRestoreDuration(status))
}

func TestGetRestoreDuration_Valid(t *testing.T) {
	now := metav1.Now()
	later := metav1.NewTime(now.Add(5 * time.Minute))
	status := dpv1alpha1.RestoreStatus{
		StartTimestamp:      &now,
		CompletionTimestamp: &later,
	}
	d := GetRestoreDuration(status)
	require.NotNil(t, d)
	assert.Equal(t, 5*time.Minute, d.Duration)
}

// --- getTimeFormat ---

func TestGetTimeFormat_Default(t *testing.T) {
	assert.Equal(t, time.RFC3339, getTimeFormat(nil))
}

func TestGetTimeFormat_Custom(t *testing.T) {
	envs := []corev1.EnvVar{
		{Name: "OTHER", Value: "val"},
		{Name: dptypes.DPTimeFormat, Value: "2006-01-02"},
	}
	assert.Equal(t, "2006-01-02", getTimeFormat(envs))
}

// --- transformTimeWithZone ---

func TestTransformTimeWithZone_EmptyZone(t *testing.T) {
	now := metav1.Now()
	result, err := transformTimeWithZone(&now, "")
	require.NoError(t, err)
	assert.Equal(t, &now, result)
}

func TestTransformTimeWithZone_ValidPositive(t *testing.T) {
	now := metav1.Now()
	result, err := transformTimeWithZone(&now, "+08:00")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTransformTimeWithZone_ValidNegative(t *testing.T) {
	now := metav1.Now()
	result, err := transformTimeWithZone(&now, "-05:30")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTransformTimeWithZone_InvalidFormat(t *testing.T) {
	now := metav1.Now()
	_, err := transformTimeWithZone(&now, "abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incorrect format")
}

func TestTransformTimeWithZone_NoColon(t *testing.T) {
	now := metav1.Now()
	_, err := transformTimeWithZone(&now, "+08000")
	require.Error(t, err)
}

func TestTransformTimeWithZone_BadHour(t *testing.T) {
	now := metav1.Now()
	_, err := transformTimeWithZone(&now, "+xx:00")
	require.Error(t, err)
}

func TestTransformTimeWithZone_BadMinute(t *testing.T) {
	now := metav1.Now()
	_, err := transformTimeWithZone(&now, "+08:yy")
	require.Error(t, err)
}

// --- BuildJobKeyForActionStatus ---

func TestBuildJobKeyForActionStatus(t *testing.T) {
	assert.Equal(t, "Job/my-job", BuildJobKeyForActionStatus("my-job"))
}

// --- getMountPathWithSourceVolume ---

func TestGetMountPathWithSourceVolume_Found(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
					VolumeMounts: []corev1.VolumeMount{
						{Name: "data", MountPath: "/data"},
						{Name: "log", MountPath: "/log"},
					},
				},
			},
		},
	}
	assert.Equal(t, "/data", getMountPathWithSourceVolume(backup, "data"))
}

func TestGetMountPathWithSourceVolume_NotFound(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
					VolumeMounts: []corev1.VolumeMount{
						{Name: "data", MountPath: "/data"},
					},
				},
			},
		},
	}
	assert.Equal(t, "", getMountPathWithSourceVolume(backup, "missing"))
}

func TestGetMountPathWithSourceVolume_NilMethod(t *testing.T) {
	backup := &dpv1alpha1.Backup{}
	assert.Equal(t, "", getMountPathWithSourceVolume(backup, "data"))
}

func TestGetMountPathWithSourceVolume_NilTargetVolumes(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{},
		},
	}
	assert.Equal(t, "", getMountPathWithSourceVolume(backup, "data"))
}

// --- restoreJobHasCompleted ---

func TestRestoreJobHasCompleted_True(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{
		{ObjectKey: "Job/job-1", Status: dpv1alpha1.RestoreActionCompleted},
	}
	assert.True(t, restoreJobHasCompleted(actions, "job-1"))
}

func TestRestoreJobHasCompleted_NotCompleted(t *testing.T) {
	actions := []dpv1alpha1.RestoreStatusAction{
		{ObjectKey: "Job/job-1", Status: dpv1alpha1.RestoreActionProcessing},
	}
	assert.False(t, restoreJobHasCompleted(actions, "job-1"))
}

func TestRestoreJobHasCompleted_NotFound(t *testing.T) {
	assert.False(t, restoreJobHasCompleted(nil, "job-1"))
}

// --- deleteRestoreJob ---

func TestDeleteRestoreJob_NotFound(t *testing.T) {
	scheme := utilsTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	reqCtx := newTestReqCtx()

	err := deleteRestoreJob(reqCtx, cli, "Job/missing-job", "default")
	require.NoError(t, err)
}

func TestDeleteRestoreJob_WithFinalizer(t *testing.T) {
	scheme := utilsTestScheme()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-job",
			Namespace:  "default",
			Finalizers: []string{dptypes.DataProtectionFinalizerName},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "c", Image: "img"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build()
	reqCtx := newTestReqCtx()

	err := deleteRestoreJob(reqCtx, cli, "Job/test-job", "default")
	require.NoError(t, err)
}

// --- cutJobName ---

func TestCutJobName_Short(t *testing.T) {
	assert.Equal(t, "short-name", cutJobName("short-name"))
}

func TestCutJobName_Exactly63(t *testing.T) {
	name := "aaaaaaaaa-bbbbbbbbb-ccccccccc-ddddddddd-eeeeeeeee-fffffffffff" // 63 chars
	assert.Equal(t, name, cutJobName(name))
}

func TestCutJobName_Long(t *testing.T) {
	name := "very-long-job-name-that-exceeds-the-maximum-length-allowed-by-kubernetes-naming-conventions"
	result := cutJobName(name)
	assert.LessOrEqual(t, len(result), 63)
}

// --- FormatRestoreTimeAndValidate ---

func TestFormatRestoreTimeAndValidate_Empty(t *testing.T) {
	result, err := FormatRestoreTimeAndValidate("", nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestFormatRestoreTimeAndValidate_RFC3339(t *testing.T) {
	now := time.Now().UTC()
	timeStr := now.Format(time.RFC3339)
	startTime := metav1.NewTime(now.Add(-1 * time.Hour))
	endTime := metav1.NewTime(now.Add(1 * time.Hour))
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			TimeRange: &dpv1alpha1.BackupTimeRange{
				Start: &startTime,
				End:   &endTime,
			},
		},
	}
	result, err := FormatRestoreTimeAndValidate(timeStr, backup)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestFormatRestoreTimeAndValidate_CustomFormat(t *testing.T) {
	now := time.Now().UTC()
	layout := "Jan 02,2006 15:04:05 UTC-0700"
	timeStr := now.Format(layout)
	startTime := metav1.NewTime(now.Add(-1 * time.Hour))
	endTime := metav1.NewTime(now.Add(1 * time.Hour))
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			TimeRange: &dpv1alpha1.BackupTimeRange{
				Start: &startTime,
				End:   &endTime,
			},
		},
	}
	result, err := FormatRestoreTimeAndValidate(timeStr, backup)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestFormatRestoreTimeAndValidate_InvalidFormat(t *testing.T) {
	_, err := FormatRestoreTimeAndValidate("not-a-time", &dpv1alpha1.Backup{})
	require.Error(t, err)
}

func TestFormatRestoreTimeAndValidate_NilTimeRange(t *testing.T) {
	now := time.Now().UTC()
	timeStr := now.Format(time.RFC3339)
	backup := &dpv1alpha1.Backup{}
	_, err := FormatRestoreTimeAndValidate(timeStr, backup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeRange")
}

func TestFormatRestoreTimeAndValidate_OutOfRange(t *testing.T) {
	now := time.Now().UTC()
	timeStr := now.Format(time.RFC3339)
	startTime := metav1.NewTime(now.Add(1 * time.Hour))
	endTime := metav1.NewTime(now.Add(2 * time.Hour))
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			TimeRange: &dpv1alpha1.BackupTimeRange{
				Start: &startTime,
				End:   &endTime,
			},
		},
	}
	_, err := FormatRestoreTimeAndValidate(timeStr, backup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of time range")
}

// --- isTimeInRange ---

func TestIsTimeInRange(t *testing.T) {
	now := time.Now()
	assert.True(t, isTimeInRange(now, now.Add(-1*time.Hour), now.Add(1*time.Hour)))
	assert.True(t, isTimeInRange(now, now, now.Add(1*time.Hour)))
	assert.True(t, isTimeInRange(now, now.Add(-1*time.Hour), now))
	assert.False(t, isTimeInRange(now, now.Add(1*time.Hour), now.Add(2*time.Hour)))
	assert.False(t, isTimeInRange(now, now.Add(-2*time.Hour), now.Add(-1*time.Hour)))
}

// --- GetRestoreFromBackupAnnotation ---

func TestGetRestoreFromBackupAnnotation_NoComponentName(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{},
		},
	}
	_, err := GetRestoreFromBackupAnnotation(backup, "Parallel", "", nil, false, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to obtain the name of the component")
}

func TestGetRestoreFromBackupAnnotation_WithComponentLabel(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			Labels: map[string]string{
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
	}
	result, err := GetRestoreFromBackupAnnotation(backup, "Parallel", "", nil, false, nil)
	require.NoError(t, err)

	var parsed map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Contains(t, parsed, "mysql")
	assert.Equal(t, "bk-1", parsed["mysql"][constant.BackupNameKeyForRestore])
}

func TestGetRestoreFromBackupAnnotation_WithShardingLabel(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "ns1",
			Labels: map[string]string{
				constant.KBAppShardingNameLabelKey: "shard-0",
				constant.KBAppComponentLabelKey:    "mysql",
			},
		},
	}
	result, err := GetRestoreFromBackupAnnotation(backup, "Parallel", "", nil, false, nil)
	require.NoError(t, err)

	var parsed map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	// sharding label takes precedence
	assert.Contains(t, parsed, "shard-0")
}

func TestGetRestoreFromBackupAnnotation_WithRestoreTimeAndEnv(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			Labels: map[string]string{
				constant.KBAppComponentLabelKey: "pg",
			},
		},
	}
	envs := []corev1.EnvVar{{Name: "FOO", Value: "bar"}}
	result, err := GetRestoreFromBackupAnnotation(backup, "Parallel", "2024-01-01T00:00:00Z", envs, true, nil)
	require.NoError(t, err)

	var parsed map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Equal(t, "2024-01-01T00:00:00Z", parsed["pg"][constant.RestoreTimeKeyForRestore])
	assert.Equal(t, "true", parsed["pg"][constant.DoReadyRestoreAfterClusterRunning])
	assert.NotEmpty(t, parsed["pg"][constant.EnvForRestore])
}

func TestGetRestoreFromBackupAnnotation_WithConnectionPassword(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bk-1",
			Namespace: "default",
			Labels: map[string]string{
				constant.KBAppComponentLabelKey: "pg",
			},
			Annotations: map[string]string{
				dptypes.ConnectionPasswordAnnotationKey: "secret-pass",
			},
		},
	}
	result, err := GetRestoreFromBackupAnnotation(backup, "Parallel", "", nil, false, nil)
	require.NoError(t, err)

	var parsed map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Equal(t, "secret-pass", parsed["pg"][constant.ConnectionPassword])
}

func TestGetRestoreFromBackupAnnotation_WithParameters(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bk-1", Namespace: "default",
			Labels: map[string]string{constant.KBAppComponentLabelKey: "pg"},
		},
	}
	params := []dpv1alpha1.ParameterPair{{Name: "key", Value: "val"}}
	result, err := GetRestoreFromBackupAnnotation(backup, "Parallel", "", nil, false, params)
	require.NoError(t, err)

	var parsed map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.NotEmpty(t, parsed["pg"][constant.ParametersForRestore])
}

// --- GetSourcePodNameFromTarget ---

func TestGetSourcePodNameFromTarget_AnyStrategy(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAny,
			},
		},
	}
	name, err := GetSourcePodNameFromTarget(target, nil, 0)
	require.NoError(t, err)
	assert.Empty(t, name)
}

func TestGetSourcePodNameFromTarget_AllStrategyNilPolicy(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAll,
			},
		},
	}
	_, err := GetSourcePodNameFromTarget(target, nil, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requiredPolicyForAllPodSelection can not be empty")
}

func TestGetSourcePodNameFromTarget_OneToMany(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAll,
			},
		},
	}
	policy := &dpv1alpha1.RequiredPolicyForAllPodSelection{
		DataRestorePolicy: dpv1alpha1.OneToManyRestorePolicy,
		SourceOfOneToMany: &dpv1alpha1.SourceOfOneToMany{TargetPodName: "pod-0"},
	}
	name, err := GetSourcePodNameFromTarget(target, policy, 0)
	require.NoError(t, err)
	assert.Equal(t, "pod-0", name)
}

func TestGetSourcePodNameFromTarget_OneToManyNoSource(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAll,
			},
		},
	}
	policy := &dpv1alpha1.RequiredPolicyForAllPodSelection{
		DataRestorePolicy: dpv1alpha1.OneToManyRestorePolicy,
	}
	_, err := GetSourcePodNameFromTarget(target, policy, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source target pod can not be empty")
}

func TestGetSourcePodNameFromTarget_OneToOne(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAll,
			},
		},
		SelectedTargetPods: []string{"pod-0", "pod-1", "pod-2"},
	}
	policy := &dpv1alpha1.RequiredPolicyForAllPodSelection{
		DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy,
	}
	name, err := GetSourcePodNameFromTarget(target, policy, 1)
	require.NoError(t, err)
	assert.Equal(t, "pod-1", name)
}

func TestGetSourcePodNameFromTarget_OneToOneOutOfRange(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				Strategy: dpv1alpha1.PodSelectionStrategyAll,
			},
		},
		SelectedTargetPods: []string{"pod-0"},
	}
	policy := &dpv1alpha1.RequiredPolicyForAllPodSelection{
		DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy,
	}
	name, err := GetSourcePodNameFromTarget(target, policy, 5)
	require.NoError(t, err)
	assert.Empty(t, name)
}

// --- GetVolumeSnapshotsBySourcePod ---

func TestGetVolumeSnapshotsBySourcePod_Found(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			Actions: []dpv1alpha1.ActionStatus{
				{
					TargetPodName: "pod-0",
					VolumeSnapshots: []dpv1alpha1.VolumeSnapshotStatus{
						{VolumeName: "data", Name: "snap-data"},
						{VolumeName: "log", Name: "snap-log"},
					},
				},
			},
		},
	}
	target := &dpv1alpha1.BackupStatusTarget{}
	result := GetVolumeSnapshotsBySourcePod(backup, target, "pod-0")
	require.NotNil(t, result)
	assert.Equal(t, "snap-data", result["data"])
	assert.Equal(t, "snap-log", result["log"])
}

func TestGetVolumeSnapshotsBySourcePod_WrongPod(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			Actions: []dpv1alpha1.ActionStatus{
				{
					TargetPodName: "pod-0",
					VolumeSnapshots: []dpv1alpha1.VolumeSnapshotStatus{
						{VolumeName: "data", Name: "snap-data"},
					},
				},
			},
		},
	}
	target := &dpv1alpha1.BackupStatusTarget{}
	assert.Nil(t, GetVolumeSnapshotsBySourcePod(backup, target, "pod-1"))
}

func TestGetVolumeSnapshotsBySourcePod_FilteredBySelectedPods(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		Status: dpv1alpha1.BackupStatus{
			Actions: []dpv1alpha1.ActionStatus{
				{
					TargetPodName: "pod-0",
					VolumeSnapshots: []dpv1alpha1.VolumeSnapshotStatus{
						{VolumeName: "data", Name: "snap-data"},
					},
				},
			},
		},
	}
	target := &dpv1alpha1.BackupStatusTarget{
		SelectedTargetPods: []string{"pod-1"}, // pod-0 is not in selected
	}
	assert.Nil(t, GetVolumeSnapshotsBySourcePod(backup, target, ""))
}

// --- ValidateParentBackupSet ---

func TestValidateParentBackupSet_NilBackups(t *testing.T) {
	err := ValidateParentBackupSet(&BackupActionSet{}, &BackupActionSet{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestValidateParentBackupSet_NotCompleted(t *testing.T) {
	parent := &BackupActionSet{Backup: &dpv1alpha1.Backup{Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseRunning}}}
	child := &BackupActionSet{Backup: &dpv1alpha1.Backup{Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted}}}
	err := ValidateParentBackupSet(parent, child)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not completed")
}

func TestValidateParentBackupSet_DifferentPolicies(t *testing.T) {
	parent := &BackupActionSet{Backup: &dpv1alpha1.Backup{
		Spec:   dpv1alpha1.BackupSpec{BackupPolicyName: "policy-a"},
		Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
	}}
	child := &BackupActionSet{Backup: &dpv1alpha1.Backup{
		Spec:   dpv1alpha1.BackupSpec{BackupPolicyName: "policy-b"},
		Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
	}}
	err := ValidateParentBackupSet(parent, child)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "defferent")
}

func TestValidateParentBackupSet_UnsupportedType(t *testing.T) {
	parent := &BackupActionSet{
		Backup: &dpv1alpha1.Backup{
			Spec:   dpv1alpha1.BackupSpec{BackupPolicyName: "p"},
			Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
		},
		ActionSet: &dpv1alpha1.ActionSet{
			Spec: dpv1alpha1.ActionSetSpec{BackupType: dpv1alpha1.BackupTypeContinuous},
		},
	}
	child := &BackupActionSet{Backup: &dpv1alpha1.Backup{
		Spec:   dpv1alpha1.BackupSpec{BackupPolicyName: "p"},
		Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
	}}
	err := ValidateParentBackupSet(parent, child)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not incremental or full")
}

// --- GetTargetRelativePath ---

func TestGetTargetRelativePath_BothEmpty(t *testing.T) {
	assert.Equal(t, "", GetTargetRelativePath("", ""))
}

func TestGetTargetRelativePath_TargetOnly(t *testing.T) {
	assert.Equal(t, "target-0", GetTargetRelativePath("target-0", ""))
}

func TestGetTargetRelativePath_Both(t *testing.T) {
	assert.Equal(t, "target-0/pod-0", GetTargetRelativePath("target-0", "pod-0"))
}

func TestGetTargetRelativePath_PodOnly(t *testing.T) {
	assert.Equal(t, "pod-0", GetTargetRelativePath("", "pod-0"))
}

// --- BackupFilePathEnv ---

func TestBackupFilePathEnv_Empty(t *testing.T) {
	envs := BackupFilePathEnv("", "t", "p")
	assert.Empty(t, envs)
}

func TestBackupFilePathEnv_Valid(t *testing.T) {
	envs := BackupFilePathEnv("/repo/ns/prefix", "target-0", "pod-0")
	require.Len(t, envs, 3)
	envMap := map[string]string{}
	for _, e := range envs {
		envMap[e.Name] = e.Value
	}
	assert.Equal(t, "target-0/pod-0", envMap[dptypes.DPTargetRelativePath])
	assert.NotEmpty(t, envMap[dptypes.DPBackupRootPath])
	assert.NotEmpty(t, envMap[dptypes.DPBackupBasePath])
}

// --- getComponentNameFromObj ---

func TestGetComponentNameFromObj_ShardingTakesPrecedence(t *testing.T) {
	obj := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constant.KBAppShardingNameLabelKey: "shard-0",
				constant.KBAppComponentLabelKey:    "mysql",
			},
		},
	}
	assert.Equal(t, "shard-0", getComponentNameFromObj(obj))
}

func TestGetComponentNameFromObj_ComponentFallback(t *testing.T) {
	obj := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
	}
	assert.Equal(t, "mysql", getComponentNameFromObj(obj))
}

func TestGetComponentNameFromObj_NoLabels(t *testing.T) {
	obj := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}},
	}
	assert.Equal(t, "", getComponentNameFromObj(obj))
}

// --- deleteRestoreJob with existing job without finalizer ---

func TestDeleteRestoreJob_WithoutFinalizer(t *testing.T) {
	scheme := utilsTestScheme()
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{{Name: "c", Image: "img"}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build()
	reqCtx := newTestReqCtx()

	err := deleteRestoreJob(reqCtx, cli, "Job/test-job", "default")
	require.NoError(t, err)

	// verify job was deleted
	list := &batchv1.JobList{}
	err = cli.List(context.Background(), list, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}
