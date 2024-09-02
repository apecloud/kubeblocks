/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package dataprotection

import (
	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockBackupPolicyTemplateFactory struct {
	testapps.BaseFactory[dpv1alpha1.BackupPolicyTemplate, *dpv1alpha1.BackupPolicyTemplate, MockBackupPolicyTemplateFactory]
}

func NewBackupPolicyTemplateFactory(name string) *MockBackupPolicyTemplateFactory {
	f := &MockBackupPolicyTemplateFactory{}
	f.Init("", name,
		&dpv1alpha1.BackupPolicyTemplate{},
		f)
	return f
}

/*func (f *MockBackupPolicyTemplateFactory) getLastBackupPolicy() *dpv1alpha1.BackupPolicy {
	l := len(f.Get().Spec.BackupPolicies)
	if l == 0 {
		return nil
	}
	backupPolicies := f.Get().Spec.BackupPolicies
	return &backupPolicies[l-1]
}*/

func (f *MockBackupPolicyTemplateFactory) getLastBackupMethod() *dpv1alpha1.BackupMethodTPL {
	l := len(f.Get().Spec.BackupMethods)
	if l == 0 {
		return nil
	}
	backupMethods := f.Get().Spec.BackupMethods
	return &backupMethods[l-1]
}

/*func (f *MockBackupPolicyTemplateFactory) setBackupPolicyField(setField func(backupPolicy *dpv1alpha1.BackupPolicy)) {
	backupPolicy := f.getLastBackupPolicy()
	if backupPolicy == nil {
		// ignore
		return
	}
	setField(backupPolicy)
}*/

func (f *MockBackupPolicyTemplateFactory) AddSchedule(method, schedule, retentionPeriod string, enable bool) *MockBackupPolicyTemplateFactory {
	schedulePolicy := dpv1alpha1.SchedulePolicy{
		Enabled:         &enable,
		CronExpression:  schedule,
		BackupMethod:    method,
		RetentionPeriod: dpv1alpha1.RetentionPeriod(retentionPeriod),
	}
	f.Get().Spec.Schedules = append(f.Get().Spec.Schedules, schedulePolicy)
	return f
}

func (f *MockBackupPolicyTemplateFactory) AddBackupMethod(name string, snapshotVolumes bool, actionSetName string) *MockBackupPolicyTemplateFactory {
	backupMethod := dpv1alpha1.BackupMethodTPL{
		Name:            name,
		SnapshotVolumes: &snapshotVolumes,
		ActionSetName:   actionSetName,
		TargetVolumes:   &dpv1alpha1.TargetVolumeInfo{},
	}
	f.Get().Spec.BackupMethods = append(f.Get().Spec.BackupMethods, backupMethod)
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetBackupMethodVolumes(names []string) *MockBackupPolicyTemplateFactory {
	backupMethod := f.getLastBackupMethod()
	backupMethod.TargetVolumes.Volumes = names
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetBackupMethodVolumeMounts(keyAndValues ...string) *MockBackupPolicyTemplateFactory {
	var volumeMounts []corev1.VolumeMount
	for k, v := range testapps.WithMap(keyAndValues...) {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      k,
			MountPath: v,
		})
	}
	backupMethod := f.getLastBackupMethod()
	backupMethod.TargetVolumes.VolumeMounts = volumeMounts
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetTargetRole(role string) *MockBackupPolicyTemplateFactory {
	f.Get().Spec.Target.Role = role
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetLabels(labels map[string]string) *MockBackupPolicyTemplateFactory {
	f.Get().SetLabels(labels)
	return f
}
