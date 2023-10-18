/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package apps

import (
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type MockBackupPolicyTemplateFactory struct {
	BaseFactory[appsv1alpha1.BackupPolicyTemplate, *appsv1alpha1.BackupPolicyTemplate, MockBackupPolicyTemplateFactory]
}

func NewBackupPolicyTemplateFactory(name string) *MockBackupPolicyTemplateFactory {
	f := &MockBackupPolicyTemplateFactory{}
	f.Init("", name,
		&appsv1alpha1.BackupPolicyTemplate{},
		f)
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetClusterDefRef(clusterDefRef string) *MockBackupPolicyTemplateFactory {
	f.Get().Spec.ClusterDefRef = clusterDefRef
	return f
}

func (f *MockBackupPolicyTemplateFactory) getLastBackupPolicy() *appsv1alpha1.BackupPolicy {
	l := len(f.Get().Spec.BackupPolicies)
	if l == 0 {
		return nil
	}
	backupPolicies := f.Get().Spec.BackupPolicies
	return &backupPolicies[l-1]
}

func (f *MockBackupPolicyTemplateFactory) getLastBackupMethod() *dpv1alpha1.BackupMethod {
	backupPolicy := f.getLastBackupPolicy()
	l := len(backupPolicy.BackupMethods)
	if l == 0 {
		return nil
	}
	backupMethods := backupPolicy.BackupMethods
	return &backupMethods[l-1].BackupMethod
}

func (f *MockBackupPolicyTemplateFactory) AddBackupPolicy(componentDef string) *MockBackupPolicyTemplateFactory {
	f.Get().Spec.BackupPolicies = append(f.Get().Spec.BackupPolicies, appsv1alpha1.BackupPolicy{
		ComponentDefRef: componentDef,
	})
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetRetentionPeriod(duration string) *MockBackupPolicyTemplateFactory {
	f.getLastBackupPolicy().RetentionPeriod = dpv1alpha1.RetentionPeriod(duration)
	return f
}

func (f *MockBackupPolicyTemplateFactory) setBackupPolicyField(setField func(backupPolicy *appsv1alpha1.BackupPolicy)) {
	backupPolicy := f.getLastBackupPolicy()
	if backupPolicy == nil {
		// ignore
		return
	}
	setField(backupPolicy)
}

func (f *MockBackupPolicyTemplateFactory) AddSchedule(method, schedule string, enable bool) *MockBackupPolicyTemplateFactory {
	schedulePolicy := appsv1alpha1.SchedulePolicy{
		Enabled:        &enable,
		CronExpression: schedule,
		BackupMethod:   method,
	}
	backupPolicy := f.getLastBackupPolicy()
	backupPolicy.Schedules = append(backupPolicy.Schedules, schedulePolicy)
	return f
}

func (f *MockBackupPolicyTemplateFactory) AddBackupMethod(name string,
	snapshotVolumes bool, actionSetName string, mappingEnvWithClusterVersion ...string) *MockBackupPolicyTemplateFactory {
	backupPolicy := f.getLastBackupPolicy()
	backupMethod := appsv1alpha1.BackupMethod{
		BackupMethod: dpv1alpha1.BackupMethod{
			Name:            name,
			SnapshotVolumes: &snapshotVolumes,
			ActionSetName:   actionSetName,
			TargetVolumes:   &dpv1alpha1.TargetVolumeInfo{},
		}}
	if len(mappingEnvWithClusterVersion) > 0 {
		backupMethod.EnvMapping = []appsv1alpha1.EnvMappingVar{
			{
				Key: EnvKeyImageTag,
				ValueFrom: appsv1alpha1.ValueFrom{
					ClusterVersionRef: []appsv1alpha1.ClusterVersionMapping{
						{
							Names:        mappingEnvWithClusterVersion,
							MappingValue: DefaultImageTag,
						},
					},
				},
			},
		}
	}
	backupPolicy.BackupMethods = append(backupPolicy.BackupMethods, backupMethod)
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetBackupMethodVolumes(names []string) *MockBackupPolicyTemplateFactory {
	backupMethod := f.getLastBackupMethod()
	backupMethod.TargetVolumes.Volumes = names
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetBackupMethodVolumeMounts(keyAndValues ...string) *MockBackupPolicyTemplateFactory {
	var volumeMounts []corev1.VolumeMount
	for k, v := range WithMap(keyAndValues...) {
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
	f.setBackupPolicyField(func(backupPolicy *appsv1alpha1.BackupPolicy) {
		backupPolicy.Target.Role = role
	})
	return f
}

func (f *MockBackupPolicyTemplateFactory) SetLabels(labels map[string]string) *MockBackupPolicyTemplateFactory {
	f.Get().SetLabels(labels)
	return f
}
