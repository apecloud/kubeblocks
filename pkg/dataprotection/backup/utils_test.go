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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/pointer"

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

func TestSetExpirationByCreationTime(t *testing.T) {
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
				assert.Equal(t, time.Date(9999, time.Month(1), 1, 0, 0, 0, 0, time.UTC).UTC(), b.Status.Expiration.Time.UTC())
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
					Phase: dpv1alpha1.BackupPhaseCompleted,
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
				assert.NotEqual(t, now.Add(24*time.Hour).UTC(), b.Status.StartTimestamp.Time.UTC())
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
			err := SetExpirationByCreationTime(tt.backup)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			tt.verify(t, tt.backup)
		})
	}
}
