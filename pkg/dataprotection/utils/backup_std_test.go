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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
)

// --- GetBackupMethodsFromBackupPolicy ---

func TestGetBackupMethodsFromBackupPolicy_ByName(t *testing.T) {
	list := &dpv1alpha1.BackupPolicyList{
		Items: []dpv1alpha1.BackupPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "bp-1"},
				Status:     dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{
						{Name: "snapshot", SnapshotVolumes: boolptr.True()},
						{Name: "xtrabackup"},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "bp-2"},
				Status:     dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{Name: "other"}},
				},
			},
		},
	}
	defaultMethod, methods := GetBackupMethodsFromBackupPolicy(list, "bp-1")
	assert.Equal(t, "snapshot", defaultMethod)
	assert.Contains(t, methods, "snapshot")
	assert.Contains(t, methods, "xtrabackup")
	_, hasOther := methods["other"]
	assert.False(t, hasOther)
}

func TestGetBackupMethodsFromBackupPolicy_DefaultPolicy(t *testing.T) {
	list := &dpv1alpha1.BackupPolicyList{
		Items: []dpv1alpha1.BackupPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "bp-default",
					Annotations: map[string]string{types.DefaultBackupPolicyAnnotationKey: "true"},
				},
				Status: dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{
						{Name: "m1"},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "bp-other"},
				Status:     dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{Name: "m2"}},
				},
			},
		},
	}
	_, methods := GetBackupMethodsFromBackupPolicy(list, "")
	assert.Contains(t, methods, "m1")
	_, hasM2 := methods["m2"]
	assert.False(t, hasM2)
}

func TestGetBackupMethodsFromBackupPolicy_SkipsUnavailable(t *testing.T) {
	list := &dpv1alpha1.BackupPolicyList{
		Items: []dpv1alpha1.BackupPolicy{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "bp-1"},
				Status:     dpv1alpha1.BackupPolicyStatus{Phase: "NotAvailable"},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{Name: "m1"}},
				},
			},
		},
	}
	_, methods := GetBackupMethodsFromBackupPolicy(list, "bp-1")
	assert.Empty(t, methods)
}

// --- ValidateScheduleNames ---

func TestValidateScheduleNames_Empty(t *testing.T) {
	err := ValidateScheduleNames(nil)
	require.NoError(t, err)
}

func TestValidateScheduleNames_NoDuplicates(t *testing.T) {
	schedules := []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "m1"},
		{BackupMethod: "m2"},
	}
	err := ValidateScheduleNames(schedules)
	require.NoError(t, err)
}

func TestValidateScheduleNames_DuplicateMethodNames(t *testing.T) {
	schedules := []dpv1alpha1.SchedulePolicy{
		{BackupMethod: "m1"},
		{BackupMethod: "m1"},
	}
	err := ValidateScheduleNames(schedules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicated")
}

func TestValidateScheduleNames_NamedSchedules(t *testing.T) {
	schedules := []dpv1alpha1.SchedulePolicy{
		{Name: "daily", BackupMethod: "m1"},
		{Name: "weekly", BackupMethod: "m1"},
	}
	err := ValidateScheduleNames(schedules)
	require.NoError(t, err)
}

func TestValidateScheduleNames_DuplicateNames(t *testing.T) {
	schedules := []dpv1alpha1.SchedulePolicy{
		{Name: "daily", BackupMethod: "m1"},
		{Name: "daily", BackupMethod: "m2"},
	}
	err := ValidateScheduleNames(schedules)
	require.Error(t, err)
}
