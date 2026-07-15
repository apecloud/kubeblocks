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

package backup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestBuildCronJobSchedule(t *testing.T) {
	const (
		cronExpression       = "0 0 * * *"
		cronExpressionWithTZ = "CRON_TZ=UTC 0 0 * * *"
		timeZone             = "UTC"
	)

	tests := []struct {
		name           string
		versionInfo    interface{}
		cronExpression string
		timeZone       *string
	}{
		{
			name:           "version v1.20",
			versionInfo:    version.Info{GitVersion: "v1.20.1-eks-12345"},
			cronExpression: cronExpression,
			timeZone:       nil,
		},
		{
			name:           "version v1.21",
			versionInfo:    version.Info{GitVersion: "v1.21.1-eks-12345"},
			cronExpression: cronExpression,
			timeZone:       nil,
		},
		{
			name:           "version v1.22",
			versionInfo:    version.Info{GitVersion: "v1.22.1-eks-12345"},
			cronExpression: cronExpressionWithTZ,
			timeZone:       nil,
		},
		{
			name:           "version v1.25",
			versionInfo:    version.Info{GitVersion: "v1.25.0-eks-12345"},
			cronExpression: cronExpression,
			timeZone:       pointer.String(timeZone),
		},
		{
			name:           "invalid version",
			versionInfo:    version.Info{GitVersion: "invalid-version"},
			cronExpression: cronExpression,
			timeZone:       nil,
		},
	}

	oldVer := viper.Get(constant.CfgKeyServerInfo)
	defer viper.Set(constant.CfgKeyServerInfo, oldVer)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(constant.CfgKeyServerInfo, tt.versionInfo)
			tz, cronExp := BuildCronJobSchedule(cronExpression)
			assert.Equal(t, tt.cronExpression, cronExp)
			assert.Equal(t, tt.timeZone, tz)
		})
	}
}

func TestSetExpirationTime(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name          string
		backup        *dpv1alpha1.Backup
		expectedError bool
		verify        func(*testing.T, *dpv1alpha1.Backup)
	}{
		{
			name: "already has expiration",
			backup: &dpv1alpha1.Backup{
				Status: dpv1alpha1.BackupStatus{
					Expiration: &metav1.Time{Time: now.Add(24 * time.Hour)},
				},
			},
			expectedError: false,
			verify: func(t *testing.T, b *dpv1alpha1.Backup) {
				assert.NotNil(t, b.Status.Expiration)
			},
		},
		{
			name: "continuous backup type with running phase",
			backup: &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						dptypes.BackupTypeLabelKey: string(dpv1alpha1.BackupTypeContinuous),
					},
				},
				Spec: dpv1alpha1.BackupSpec{
					RetentionPeriod: "24h",
				},
				Status: dpv1alpha1.BackupStatus{
					Phase: dpv1alpha1.BackupPhaseRunning,
				},
			},
			expectedError: false,
			verify: func(t *testing.T, b *dpv1alpha1.Backup) {
				assert.Nil(t, b.Status.Expiration)
			},
		},
		{
			name: "continuous backup type with failed phase",
			backup: &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						dptypes.BackupTypeLabelKey: string(dpv1alpha1.BackupTypeContinuous),
					},
				},
				Spec: dpv1alpha1.BackupSpec{
					RetentionPeriod: "24h",
				},
				Status: dpv1alpha1.BackupStatus{
					Phase: dpv1alpha1.BackupPhaseFailed,
					TimeRange: &dpv1alpha1.BackupTimeRange{
						Start: &now,
						End:   func() *metav1.Time { t := metav1.NewTime(now.Add(1 * time.Hour)); return &t }(),
					},
				},
			},
			expectedError: false,
			verify: func(t *testing.T, b *dpv1alpha1.Backup) {
				assert.Equal(t, now.Add(24*time.Hour).UTC(), b.Status.Expiration.Time.UTC())
			},
		},
		{
			name: "continuous backup type with completed phase",
			backup: &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						dptypes.BackupTypeLabelKey: string(dpv1alpha1.BackupTypeContinuous),
					},
				},
				Spec: dpv1alpha1.BackupSpec{
					RetentionPeriod: "24h",
				},
				Status: dpv1alpha1.BackupStatus{
					Phase:               dpv1alpha1.BackupPhaseCompleted,
					CompletionTimestamp: &now,
				},
			},
			expectedError: false,
			verify: func(t *testing.T, b *dpv1alpha1.Backup) {
				assert.Equal(t, now.Add(24*time.Hour).UTC(), b.Status.Expiration.Time.UTC())
			},
		},
		{
			name: "with retention period and start timestamp",
			backup: &dpv1alpha1.Backup{
				Spec: dpv1alpha1.BackupSpec{
					RetentionPeriod: "24h",
				},
				Status: dpv1alpha1.BackupStatus{
					StartTimestamp: &now,
				},
			},
			expectedError: false,
			verify: func(t *testing.T, b *dpv1alpha1.Backup) {
				assert.Equal(t, now.Add(24*time.Hour).UTC(), b.Status.Expiration.Time.UTC())
			},
		},
		{
			name: "without retention period",
			backup: &dpv1alpha1.Backup{
				Status: dpv1alpha1.BackupStatus{
					StartTimestamp: &now,
				},
			},
			expectedError: false,
			verify: func(t *testing.T, b *dpv1alpha1.Backup) {
				assert.Nil(t, b.Status.Expiration)
			},
		},
		{
			name: "invalid retention period",
			backup: &dpv1alpha1.Backup{
				Spec: dpv1alpha1.BackupSpec{
					RetentionPeriod: "invalid",
				},
			},
			expectedError: true,
			verify: func(t *testing.T, b *dpv1alpha1.Backup) {
				assert.Nil(t, b.Status.Expiration)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetExpirationTime(tt.backup)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			tt.verify(t, tt.backup)
		})
	}
}

func TestBackupNameAndPathHelpers(t *testing.T) {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-name",
			Namespace: "ns",
			UID:       types.UID("1234567890abcdef"),
			Labels: map[string]string{
				constant.KBAppComponentLabelKey: "excluded",
				constant.AppManagedByLabelKey:   constant.AppName,
				"keep":                          "value",
			},
		},
	}
	target := &dpv1alpha1.BackupTarget{Name: "target", PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAll}}

	labels := BuildBackupWorkloadLabels(backup)
	assert.Equal(t, "value", labels["keep"])
	assert.NotContains(t, labels, constant.KBAppComponentLabelKey)
	assert.Equal(t, backup.Name, labels[dptypes.BackupNameLabelKey])
	assert.Equal(t, dptypes.AppName, labels[constant.AppManagedByLabelKey])
	assert.Equal(t, constant.AppName, backup.Labels[constant.AppManagedByLabelKey])
	unlabeledBackup := backup.DeepCopy()
	unlabeledBackup.Labels = nil
	assert.Equal(t, dptypes.AppName, BuildBackupWorkloadLabels(unlabeledBackup)[constant.AppManagedByLabelKey])
	assert.Nil(t, unlabeledBackup.Labels)

	assert.Equal(t, "dp-backup-backup-name-12345678", GenerateBackupJobName(backup, "dp-backup"))
	longName := GenerateBackupJobName(&dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "very-long-backup-name-that-should-be-trimmed-for-job-label-limits", UID: types.UID("1234567890abcdef")}}, "prefix")
	assert.LessOrEqual(t, len(longName), 63)
	assert.False(t, longName[len(longName)-1:] == "-")

	assert.Equal(t, "dp-target-backup-name", GenerateBackupStatefulSetName(backup, "target", "dp"))
	assert.Equal(t, "backup-name", GenerateBackupStatefulSetName(backup, "", "dp"))
	assert.Equal(t, "/repo/ns/path/backup-name", BuildBaseBackupPath(backup, "/repo/", "/path/"))
	assert.Equal(t, "/repo/ns/path/backup-name/target/pod-0", BuildBackupPathByTarget(backup, target, "/repo/", "/path/", "pod-0"))
	assert.Equal(t, "target/pod-0", BuildTargetRelativePath(target, "pod-0"))
	assert.Equal(t, "/repo/ns/path/kopia", BuildKopiaRepoPath(backup, "/repo/", "/path/"))
}

func TestBackupScheduleNameHelpers(t *testing.T) {
	schedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "schedule",
			Namespace: "namespace",
			UID:       types.UID("schedule-uid"),
			OwnerReferences: []metav1.OwnerReference{{
				UID: types.UID("owneruid-123456"),
			}},
		},
		Spec: dpv1alpha1.BackupScheduleSpec{Schedules: []dpv1alpha1.SchedulePolicy{{Name: "daily", BackupMethod: "full"}, {BackupMethod: "log"}}},
	}
	assert.Equal(t, "owneruid-schedule-namespace-full", GenerateCRNameByBackupSchedule(schedule, "full"))
	assert.Equal(t, "owneruid-schedule-namespace-cron", GenerateCRNameByScheduleNameAndMethod(schedule, "full", "cron"))
	assert.Equal(t, "owneruid-schedule-namespace-full", GenerateCRNameByScheduleNameAndMethod(schedule, "full", ""))
	assert.Equal(t, "schedule-schedule-namespace-full", GenerateLegacyCRNameByBackupSchedule(schedule, "full"))
	assert.Equal(t, "daily", GetSchedulePolicyByMethod(schedule, "full").Name)
	assert.Nil(t, GetSchedulePolicyByMethod(schedule, "missing"))
}

func TestBackupVolumeHelpers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"}}},
				{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{}}},
			},
		},
	}
	info := &dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data", "missing"}}
	assert.Equal(t, []corev1.Volume{pod.Spec.Volumes[0]}, getVolumesByVolumeInfo(pod, info))
	assert.Nil(t, getVolumesByVolumeInfo(pod, nil))

	mountInfo := &dpv1alpha1.TargetVolumeInfo{VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/cfg"}, {Name: "missing", MountPath: "/missing"}}}
	assert.Equal(t, []corev1.Volume{pod.Spec.Volumes[1]}, getVolumesByVolumeInfo(pod, mountInfo))
	assert.Equal(t, []corev1.VolumeMount{{Name: "config", MountPath: "/cfg"}}, getVolumeMountsByVolumeInfo(pod, mountInfo))
	assert.Nil(t, getVolumeMountsByVolumeInfo(pod, nil))

	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "ns"}}).Build()
	pvcs, err := getPVCsByVolumeNames(cli, pod, []string{"data"})
	assert.NoError(t, err)
	assert.Len(t, pvcs, 1)
	assert.Equal(t, "data", pvcs[0].VolumeName)
	_, err = getPVCsByVolumeNames(cli, pod, []string{"missing-pvc"})
	assert.NoError(t, err)
}

func TestStopStatefulSetsWhenFailed(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, appsv1.AddToScheme(scheme))
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))

	backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns"}, Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseFailed}}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: GenerateBackupStatefulSetName(backup, "target", BackupDataJobNamePrefix), Namespace: "ns"},
		Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(1)},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()
	assert.NoError(t, StopStatefulSetsWhenFailed(context.Background(), cli, backup, "target"))

	got := &appsv1.StatefulSet{}
	assert.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, got))
	assert.Equal(t, int32(0), *got.Spec.Replicas)

	backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
	got.Spec.Replicas = pointer.Int32(2)
	assert.NoError(t, cli.Update(context.Background(), got))
	assert.NoError(t, StopStatefulSetsWhenFailed(context.Background(), cli, backup, "target"))
	assert.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, got))
	assert.Equal(t, int32(2), *got.Spec.Replicas)
}

func TestBuildParametersManifest(t *testing.T) {
	manifest, err := BuildParametersManifest(nil)
	assert.NoError(t, err)
	assert.Empty(t, manifest)

	manifest, err = BuildParametersManifest([]dpv1alpha1.ParameterPair{{Name: "p", Value: "v"}})
	assert.NoError(t, err)
	assert.JSONEq(t, `[{"name":"p","value":"v"}]`, manifest[len("\n  parameters: "):])
}
