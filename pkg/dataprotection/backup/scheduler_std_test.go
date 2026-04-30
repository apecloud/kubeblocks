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

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func schedulerTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = dpv1alpha1.AddToScheme(s)
	_ = opsv1alpha1.AddToScheme(s)
	return s
}

func newTestScheduler(cli client.Client, scheme *runtime.Scheme) *Scheduler {
	return &Scheduler{
		RequestCtx: intctrlutil.RequestCtx{
			Ctx: context.Background(),
			Log: logr.Discard(),
		},
		Client: cli,
		Scheme: scheme,
		BackupSchedule: &dpv1alpha1.BackupSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-schedule",
				Namespace:   "default",
				UID:         types.UID("abcdefgh-1234-5678-9abc-def012345678"),
				Annotations: map[string]string{},
			},
			Spec: dpv1alpha1.BackupScheduleSpec{
				BackupPolicyName: "test-policy",
			},
		},
		BackupPolicy: &dpv1alpha1.BackupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "default",
			},
			Spec: dpv1alpha1.BackupPolicySpec{
				BackupMethods: []dpv1alpha1.BackupMethod{
					{Name: "full"},
				},
			},
		},
	}
}

// --- getLastAppliedConfigsMap ---

func TestGetLastAppliedConfigsMap_Empty(t *testing.T) {
	s := newTestScheduler(nil, nil)
	result, err := s.getLastAppliedConfigsMap()
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetLastAppliedConfigsMap_Valid(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = `{"full":"[{\"name\":\"key\",\"value\":\"val\"}]"}`
	result, err := s.getLastAppliedConfigsMap()
	require.NoError(t, err)
	assert.Contains(t, result, "full")
}

func TestGetLastAppliedConfigsMap_InvalidJSON(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = `not-json`
	_, err := s.getLastAppliedConfigsMap()
	require.Error(t, err)
}

// --- getReconfigureGenerationKey ---

func TestGetReconfigureGenerationKey_Empty(t *testing.T) {
	s := newTestScheduler(nil, nil)
	gen, err := s.getReconfigureGenerationKey()
	require.NoError(t, err)
	assert.Equal(t, 0, gen)
}

func TestGetReconfigureGenerationKey_Valid(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.BackupSchedule.Annotations[dptypes.ReConfigureGenerationKey] = "5"
	gen, err := s.getReconfigureGenerationKey()
	require.NoError(t, err)
	assert.Equal(t, 5, gen)
}

func TestGetReconfigureGenerationKey_Invalid(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.BackupSchedule.Annotations[dptypes.ReConfigureGenerationKey] = "not-a-number"
	gen, err := s.getReconfigureGenerationKey()
	require.Error(t, err)
	assert.Equal(t, -1, gen)
}

// --- convertLastAppliedConfigs ---

func TestConvertLastAppliedConfigs_AlreadyMigrated(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = `{"full":"old"}`
	s.convertLastAppliedConfigs("full")
	// should not change anything
	assert.Equal(t, `{"full":"old"}`, s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey])
}

func TestConvertLastAppliedConfigs_NoLegacy(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.convertLastAppliedConfigs("full")
	// no annotations to convert
	_, exists := s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey]
	assert.False(t, exists)
}

func TestConvertLastAppliedConfigs_LegacyToNew(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.BackupSchedule.Annotations[constant.LastAppliedConfigAnnotationKey] = `[{"name":"k","value":"v"}]`
	s.convertLastAppliedConfigs("full")
	newVal := s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey]
	assert.Contains(t, newVal, "full")
	// the legacy value is JSON-marshaled as a string value in the new map
	assert.NotEmpty(t, newVal)
}

// --- generateBackupName ---

func TestGenerateBackupName_WithClusterLabel(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.BackupPolicy.Spec.Target = &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constant.AppInstanceLabelKey: "my-cluster",
				},
			},
		},
	}

	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod: "full",
	}
	name := s.generateBackupName(sp)
	assert.Contains(t, name, "my-cluster")
	assert.Contains(t, name, "$(date")
}

func TestGenerateBackupName_NoClusterLabel(t *testing.T) {
	s := newTestScheduler(nil, nil)
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod: "full",
	}
	name := s.generateBackupName(sp)
	assert.Contains(t, name, "test-schedule")
	assert.Contains(t, name, "$(date")
}

func TestGenerateBackupName_WithScheduleName(t *testing.T) {
	s := newTestScheduler(nil, nil)
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod: "full",
		Name:         "daily-full",
	}
	name := s.generateBackupName(sp)
	assert.Contains(t, name, "daily-full")
}

// --- validate ---

func TestValidate_MethodNotInPolicy(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "nonexistent"},
	}
	err := s.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in backup policy")
}

func TestValidate_MethodInPolicy(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "full"},
	}
	err := s.validate()
	require.NoError(t, err)
}

// --- reconcileCronJob ---

func TestReconcileCronJob_DisabledNoCronJob(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	falseVal := false
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod: "full",
		Enabled:      &falseVal,
	}
	err := s.reconcileCronJob(sp)
	require.NoError(t, err)
}

func TestReconcileCronJob_EnabledCreates(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	trueVal := true
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod:   "full",
		Enabled:        &trueVal,
		CronExpression: "0 0 * * *",
	}
	err := s.reconcileCronJob(sp)
	require.NoError(t, err)

	// verify cronjob was created
	cronJobList := &batchv1.CronJobList{}
	err = cli.List(context.Background(), cronJobList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Len(t, cronJobList.Items, 1)
	assert.Contains(t, cronJobList.Items[0].Labels[dptypes.BackupScheduleLabelKey], "test-schedule")
}

func TestReconcileCronJob_DisabledDeletesExisting(t *testing.T) {
	scheme := schedulerTestScheme()
	s := newTestScheduler(nil, scheme)

	// pre-create a CronJob matching the schedule
	cronJobName := GenerateCRNameByScheduleNameAndMethod(s.BackupSchedule, "full", "")
	existingCronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: "default",
			Labels: map[string]string{
				dptypes.BackupScheduleLabelKey: "test-schedule",
				dptypes.BackupMethodLabelKey:   "full",
			},
			Finalizers: []string{dptypes.DataProtectionFinalizerName},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: "c", Image: "img"}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingCronJob).Build()
	s.Client = cli

	falseVal := false
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod: "full",
		Enabled:      &falseVal,
	}
	err := s.reconcileCronJob(sp)
	require.NoError(t, err)

	// verify cronjob was deleted
	list := &batchv1.CronJobList{}
	err = cli.List(context.Background(), list, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestReconcileCronJob_PatchesExisting(t *testing.T) {
	scheme := schedulerTestScheme()
	s := newTestScheduler(nil, scheme)

	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	cronJobName := GenerateCRNameByScheduleNameAndMethod(s.BackupSchedule, "full", "")
	existingCronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: "default",
			Labels: map[string]string{
				dptypes.BackupScheduleLabelKey: "test-schedule",
				dptypes.BackupMethodLabelKey:   "full",
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: "c", Image: "old-img"}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingCronJob).Build()
	s.Client = cli

	trueVal := true
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod:   "full",
		Enabled:        &trueVal,
		CronExpression: "*/30 * * * *",
	}
	err := s.reconcileCronJob(sp)
	require.NoError(t, err)

	// verify cronjob was patched
	updated := &batchv1.CronJob{}
	err = cli.Get(context.Background(), client.ObjectKey{Name: cronJobName, Namespace: "default"}, updated)
	require.NoError(t, err)
}

func TestReconcileCronJob_WithStartingDeadline(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	deadlineMinutes := int64(10)
	s.BackupSchedule.Spec.StartingDeadlineMinutes = &deadlineMinutes

	trueVal := true
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod:   "full",
		Enabled:        &trueVal,
		CronExpression: "0 0 * * *",
	}
	err := s.reconcileCronJob(sp)
	require.NoError(t, err)

	list := &batchv1.CronJobList{}
	err = cli.List(context.Background(), list, client.InNamespace("default"))
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.NotNil(t, list.Items[0].Spec.StartingDeadlineSeconds)
	assert.Equal(t, int64(600), *list.Items[0].Spec.StartingDeadlineSeconds)
}

// --- buildCronJob ---

func TestBuildCronJob_Basic(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod:   "full",
		CronExpression: "0 0 * * *",
	}
	cronJob, err := s.buildCronJob(sp, "")
	require.NoError(t, err)
	require.NotNil(t, cronJob)
	assert.Equal(t, "default", cronJob.Namespace)
	assert.Contains(t, cronJob.Labels[dptypes.BackupMethodLabelKey], "full")
	assert.Contains(t, cronJob.Labels[constant.AppManagedByLabelKey], dptypes.AppName)
	assert.Equal(t, batchv1.ForbidConcurrent, cronJob.Spec.ConcurrencyPolicy)
	assert.Equal(t, int32(0), *cronJob.Spec.SuccessfulJobsHistoryLimit)
}

func TestBuildCronJob_WithCustomName(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod:   "full",
		CronExpression: "*/5 * * * *",
	}
	cronJob, err := s.buildCronJob(sp, "custom-cron-name")
	require.NoError(t, err)
	assert.Equal(t, "custom-cron-name", cronJob.Name)
}

// --- getGenerateContinuousBackup ---

func TestGetGenerateContinuousBackup_NotFound(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "continuous"}
	backup, err := s.getGenerateContinuousBackup(sp)
	require.NoError(t, err)
	// returns empty backup object
	assert.Empty(t, backup.Name)
}

func TestGetGenerateContinuousBackup_Found(t *testing.T) {
	scheme := schedulerTestScheme()
	backupName := GenerateCRNameByBackupSchedule(&dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schedule",
			Namespace: "default",
			UID:       types.UID("abcdefgh-1234-5678-9abc-def012345678"),
		},
	}, "continuous")

	existing := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: "default",
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	s := newTestScheduler(cli, scheme)

	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "continuous"}
	backup, err := s.getGenerateContinuousBackup(sp)
	require.NoError(t, err)
	assert.Equal(t, backupName, backup.Name)
}

// --- reconcileForContinuous ---

func TestReconcileForContinuous_CreatesNewBackup(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	trueVal := true
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod:    "continuous",
		Enabled:         &trueVal,
		RetentionPeriod: dpv1alpha1.RetentionPeriod("7d"),
	}
	targetLabels := map[string]string{
		constant.AppInstanceLabelKey: "my-cluster",
	}
	err := s.reconcileForContinuous(sp, targetLabels)
	require.NoError(t, err)

	// verify backup was created
	backupList := &dpv1alpha1.BackupList{}
	err = cli.List(context.Background(), backupList, client.InNamespace("default"))
	require.NoError(t, err)
	require.Len(t, backupList.Items, 1)
	assert.Equal(t, "continuous", backupList.Items[0].Spec.BackupMethod)
	assert.Equal(t, "my-cluster", backupList.Items[0].Labels[constant.AppInstanceLabelKey])
}

func TestReconcileForContinuous_DisabledNoExisting(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	falseVal := false
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod: "continuous",
		Enabled:      &falseVal,
	}
	err := s.reconcileForContinuous(sp, nil)
	require.NoError(t, err)

	// no backup should be created
	backupList := &dpv1alpha1.BackupList{}
	err = cli.List(context.Background(), backupList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, backupList.Items)
}

func TestReconcileForContinuous_ExistingBackupPatch(t *testing.T) {
	scheme := schedulerTestScheme()
	s := newTestScheduler(nil, scheme)

	backupName := GenerateCRNameByBackupSchedule(s.BackupSchedule, "continuous")
	existing := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: "default",
		},
		Spec: dpv1alpha1.BackupSpec{
			BackupMethod: "continuous",
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	s.Client = cli

	trueVal := true
	sp := &dpv1alpha1.SchedulePolicy{
		BackupMethod:    "continuous",
		Enabled:         &trueVal,
		RetentionPeriod: dpv1alpha1.RetentionPeriod("14d"),
	}
	err := s.reconcileForContinuous(sp, nil)
	require.NoError(t, err)

	// verify backup was patched
	updated := &dpv1alpha1.Backup{}
	err = cli.Get(context.Background(), client.ObjectKey{Name: backupName, Namespace: "default"}, updated)
	require.NoError(t, err)
	assert.Equal(t, dpv1alpha1.RetentionPeriod("14d"), updated.Spec.RetentionPeriod)
}

// --- reconfigure ---

func TestReconfigure_EmptyRef(t *testing.T) {
	s := newTestScheduler(nil, nil)
	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full"}
	err := s.reconfigure(sp)
	require.NoError(t, err)
}

func TestReconfigure_InvalidJSON(t *testing.T) {
	s := newTestScheduler(nil, nil)
	s.BackupSchedule.Annotations[dptypes.ReconfigureRefAnnotationKey] = "not-json"
	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full"}
	err := s.reconfigure(sp)
	require.Error(t, err)
}

// --- backupReconfigureRef struct ---

func TestBackupReconfigureRef_Fields(t *testing.T) {
	ref := backupReconfigureRef{
		Name: "test-config",
		Key:  "config-key",
	}
	assert.Equal(t, "test-config", ref.Name)
	assert.Equal(t, "config-key", ref.Key)
	assert.Nil(t, ref.Enable)
	assert.Nil(t, ref.Disable)
}

// --- Schedule ---

func TestSchedule_HappyPath(t *testing.T) {
	scheme := schedulerTestScheme()
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "full-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	s := newTestScheduler(cli, scheme)
	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "full", ActionSetName: "full-as"},
	}

	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	trueVal := true
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "full", Enabled: &trueVal, CronExpression: "0 0 * * *"},
	}
	err := s.Schedule()
	require.NoError(t, err)

	// verify cronjob was created
	cronJobList := &batchv1.CronJobList{}
	err = cli.List(context.Background(), cronJobList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Len(t, cronJobList.Items, 1)
}

func TestSchedule_ValidationFails(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "nonexistent"},
	}
	err := s.Schedule()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in backup policy")
}

// --- handleSchedulePolicy ---

func TestHandleSchedulePolicy_CronJobPath(t *testing.T) {
	scheme := schedulerTestScheme()
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "full-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	s := newTestScheduler(cli, scheme)
	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "full", ActionSetName: "full-as"},
	}

	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	trueVal := true
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "full", Enabled: &trueVal, CronExpression: "0 0 * * *"},
	}
	err := s.handleSchedulePolicy(0)
	require.NoError(t, err)
}

func TestHandleSchedulePolicy_ContinuousPath(t *testing.T) {
	scheme := schedulerTestScheme()

	// Create an ActionSet with BackupType=Continuous
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "continuous-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeContinuous,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	s := newTestScheduler(cli, scheme)

	// Add a method with the continuous actionset
	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "continuous", ActionSetName: "continuous-as"},
	}
	s.BackupPolicy.Spec.Target = &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constant.AppInstanceLabelKey: "my-cluster",
				},
			},
		},
	}

	trueVal := true
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "continuous", Enabled: &trueVal, RetentionPeriod: dpv1alpha1.RetentionPeriod("7d")},
	}
	err := s.handleSchedulePolicy(0)
	require.NoError(t, err)

	// verify backup was created for continuous
	backupList := &dpv1alpha1.BackupList{}
	err = cli.List(context.Background(), backupList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Len(t, backupList.Items, 1)
}

func TestHandleSchedulePolicy_SnapshotVolumesSkipsContinuous(t *testing.T) {
	scheme := schedulerTestScheme()

	trueVal := true
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "snap-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeContinuous,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	s := newTestScheduler(cli, scheme)

	viper.Set(constant.KBToolsImage, "tools:latest")
	defer viper.Set(constant.KBToolsImage, "")

	// Method has SnapshotVolumes=true so continuous check is skipped
	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "snap-method", ActionSetName: "snap-as", SnapshotVolumes: &trueVal},
	}
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "snap-method", Enabled: &trueVal, CronExpression: "0 0 * * *"},
	}
	// Should fall through to reconcileCronJob since SnapshotVolumes is true
	err := s.handleSchedulePolicy(0)
	require.NoError(t, err)
}

// --- buildCheckCommand ---

func TestBuildCheckCommand_IncrementalType(t *testing.T) {
	scheme := schedulerTestScheme()
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "incr-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeIncremental,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	s := newTestScheduler(cli, scheme)

	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "full", ActionSetName: "incr-as", CompatibleMethod: "base-full"},
	}

	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full"}
	cmd, err := s.buildCheckCommand(sp)
	require.NoError(t, err)
	assert.Contains(t, cmd, "kubectl get backups")
	assert.Contains(t, cmd, "base-full")
	assert.Contains(t, cmd, "Completed")
}

func TestBuildCheckCommand_FullType_Empty(t *testing.T) {
	scheme := schedulerTestScheme()
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "full-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	s := newTestScheduler(cli, scheme)

	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "full", ActionSetName: "full-as"},
	}

	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full"}
	cmd, err := s.buildCheckCommand(sp)
	require.NoError(t, err)
	assert.Empty(t, cmd)
}

func TestBuildCheckCommand_NoActionSet(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "full", ActionSetName: "missing-as"},
	}

	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full"}
	_, err := s.buildCheckCommand(sp)
	require.Error(t, err)
}

// --- reconcileReconfigure ---

func TestReconcileReconfigure_NoOps(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	err := s.reconcileReconfigure(s.BackupSchedule)
	require.NoError(t, err)
}

func TestReconcileReconfigure_SucceededOps(t *testing.T) {
	scheme := schedulerTestScheme()
	ops := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ops",
			Namespace: "default",
			Labels: map[string]string{
				dptypes.BackupScheduleLabelKey: "test-schedule",
			},
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			Type:        opsv1alpha1.ReconfiguringType,
			ClusterName: "my-cluster",
		},
		Status: opsv1alpha1.OpsRequestStatus{
			Phase: opsv1alpha1.OpsSucceedPhase,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ops).Build()
	s := newTestScheduler(cli, scheme)

	err := s.reconcileReconfigure(s.BackupSchedule)
	require.NoError(t, err)
}

func TestReconcileReconfigure_FailedOps(t *testing.T) {
	scheme := schedulerTestScheme()
	ops := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ops",
			Namespace: "default",
			Labels: map[string]string{
				dptypes.BackupScheduleLabelKey: "test-schedule",
			},
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			Type:        opsv1alpha1.ReconfiguringType,
			ClusterName: "my-cluster",
		},
		Status: opsv1alpha1.OpsRequestStatus{
			Phase: opsv1alpha1.OpsFailedPhase,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ops).Build()
	s := newTestScheduler(cli, scheme)

	err := s.reconcileReconfigure(s.BackupSchedule)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ops failed")
}

func TestReconcileReconfigure_InProgressOps(t *testing.T) {
	scheme := schedulerTestScheme()
	ops := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ops",
			Namespace: "default",
			Labels: map[string]string{
				dptypes.BackupScheduleLabelKey: "test-schedule",
			},
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			Type:        opsv1alpha1.ReconfiguringType,
			ClusterName: "my-cluster",
		},
		Status: opsv1alpha1.OpsRequestStatus{
			Phase: opsv1alpha1.OpsRunningPhase,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ops).Build()
	s := newTestScheduler(cli, scheme)

	err := s.reconcileReconfigure(s.BackupSchedule)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "waiting for ops")
}

// --- reconfigure deeper paths ---

func TestReconfigure_DisabledFirstPolicy(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	// Set up reconfigure ref annotation with valid JSON
	s.BackupSchedule.Annotations[dptypes.ReconfigureRefAnnotationKey] = `{"name":"cfg","key":"k"}`

	falseVal := false
	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full", Enabled: &falseVal}
	// No lastAppliedConfigsMap entry for "full" and disabled → should return nil
	err := s.reconfigure(sp)
	require.NoError(t, err)
}

func TestReconfigure_NilConfigParameters(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	// reconfigure ref with enable/disable both nil
	s.BackupSchedule.Annotations[dptypes.ReconfigureRefAnnotationKey] = `{"name":"cfg","key":"k"}`
	// force lastAppliedConfigsMap to have an entry so we pass the "first policy" check
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = `{"full":"old"}`

	trueVal := true
	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full", Enabled: &trueVal}
	err := s.reconfigure(sp)
	require.NoError(t, err)
}

func TestReconfigure_EmptyParameters(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	// reconfigure ref with enable set but empty parameters for "full"
	s.BackupSchedule.Annotations[dptypes.ReconfigureRefAnnotationKey] = `{"name":"cfg","key":"k","enable":{"other-method":[{"name":"p","value":"v"}]}}`
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = `{"full":"old"}`

	trueVal := true
	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full", Enabled: &trueVal}
	err := s.reconfigure(sp)
	require.NoError(t, err)
}

func TestReconfigure_MatchingParametersCallsReconcileReconfigure(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)
	s.RequestCtx.Recorder = record.NewFakeRecorder(10)

	// Set parameters that match lastApplied so it calls reconcileReconfigure
	// opsv1alpha1.ParameterPair has Key and Value fields
	s.BackupSchedule.Annotations[dptypes.ReconfigureRefAnnotationKey] = `{"name":"cfg","key":"k","enable":{"full":[{"key":"p","value":"v"}]}}`
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = `{"full":"[{\"key\":\"p\",\"value\":\"v\"}]"}`

	trueVal := true
	sp := &dpv1alpha1.SchedulePolicy{BackupMethod: "full", Enabled: &trueVal}
	// Should call reconcileReconfigure which lists ops → finds none → returns nil
	err := s.reconfigure(sp)
	require.NoError(t, err)
}

// --- validate with parameters ---

func TestValidate_WithActionSetNoParams(t *testing.T) {
	scheme := schedulerTestScheme()
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	s := newTestScheduler(cli, scheme)

	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "full", ActionSetName: "test-as"},
	}
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "full"},
	}
	err := s.validate()
	require.NoError(t, err)
}

func TestValidate_DuplicateScheduleNames(t *testing.T) {
	scheme := schedulerTestScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	s := newTestScheduler(cli, scheme)

	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "full", Name: "dup"},
		{BackupMethod: "full", Name: "dup"},
	}
	err := s.validate()
	require.Error(t, err)
}

func TestValidate_UndeclaredParameters(t *testing.T) {
	scheme := schedulerTestScheme()
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{Name: "test-as"},
		Spec: dpv1alpha1.ActionSetSpec{
			BackupType: dpv1alpha1.BackupTypeFull,
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actionSet).Build()
	s := newTestScheduler(cli, scheme)

	s.BackupPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{
		{Name: "full", ActionSetName: "test-as"},
	}
	s.BackupSchedule.Spec.Schedules = []dpv1alpha1.SchedulePolicy{
		{
			BackupMethod: "full",
			Parameters:   []dpv1alpha1.ParameterPair{{Name: "p", Value: "v"}},
		},
	}
	err := s.validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undeclared")
}

// --- DeletionStatus ---

func TestSchedulerConstants(t *testing.T) {
	assert.Equal(t, "dp-prebackup", prebackupJobNamePrefix)
	assert.Equal(t, "dp-postbackup", postbackupJobNamePrefix)
}
