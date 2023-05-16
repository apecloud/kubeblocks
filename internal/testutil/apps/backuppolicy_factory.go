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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type MockBackupPolicyFactory struct {
	BaseFactory[dataprotectionv1alpha1.BackupPolicy, *dataprotectionv1alpha1.BackupPolicy, MockBackupPolicyFactory]
	backupType dataprotectionv1alpha1.BackupType
}

func NewBackupPolicyFactory(namespace, name string) *MockBackupPolicyFactory {
	f := &MockBackupPolicyFactory{}
	f.init(namespace, name,
		&dataprotectionv1alpha1.BackupPolicy{}, f)
	return f
}

func (factory *MockBackupPolicyFactory) setBasePolicyField(setField func(basePolicy *dataprotectionv1alpha1.BasePolicy)) {
	var basePolicy *dataprotectionv1alpha1.BasePolicy
	switch factory.backupType {
	case dataprotectionv1alpha1.BackupTypeDataFile:
		basePolicy = &factory.get().Spec.Datafile.BasePolicy
	case dataprotectionv1alpha1.BackupTypeLogFile:
		basePolicy = &factory.get().Spec.Logfile.BasePolicy
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		basePolicy = &factory.get().Spec.Snapshot.BasePolicy
	}
	if basePolicy == nil {
		// ignore
		return
	}
	setField(basePolicy)
}

func (factory *MockBackupPolicyFactory) setCommonPolicyField(setField func(commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy)) {
	var commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy
	switch factory.backupType {
	case dataprotectionv1alpha1.BackupTypeDataFile:
		commonPolicy = factory.get().Spec.Datafile
	case dataprotectionv1alpha1.BackupTypeLogFile:
		commonPolicy = factory.get().Spec.Logfile
	}
	if commonPolicy == nil {
		// ignore
		return
	}
	setField(commonPolicy)
}

func (factory *MockBackupPolicyFactory) setScheduleField(setField func(schedulePolicy *dataprotectionv1alpha1.SchedulePolicy)) {
	var schedulePolicy *dataprotectionv1alpha1.SchedulePolicy
	switch factory.backupType {
	case dataprotectionv1alpha1.BackupTypeDataFile:
		factory.get().Spec.Schedule.Datafile = &dataprotectionv1alpha1.SchedulePolicy{}
		schedulePolicy = factory.get().Spec.Schedule.Datafile
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		factory.get().Spec.Schedule.Snapshot = &dataprotectionv1alpha1.SchedulePolicy{}
		schedulePolicy = factory.get().Spec.Schedule.Snapshot
	// todo: set logfile schedule
	case dataprotectionv1alpha1.BackupTypeLogFile:
		factory.get().Spec.Schedule.Logfile = &dataprotectionv1alpha1.SchedulePolicy{}
		schedulePolicy = factory.get().Spec.Schedule.Snapshot
	}
	if schedulePolicy == nil {
		// ignore
		return
	}
	setField(schedulePolicy)
}

func (factory *MockBackupPolicyFactory) AddSnapshotPolicy() *MockBackupPolicyFactory {
	factory.get().Spec.Snapshot = &dataprotectionv1alpha1.SnapshotPolicy{
		Hooks: &dataprotectionv1alpha1.BackupPolicyHook{},
	}
	factory.backupType = dataprotectionv1alpha1.BackupTypeSnapshot
	return factory
}

func (factory *MockBackupPolicyFactory) AddFullPolicy() *MockBackupPolicyFactory {
	factory.get().Spec.Datafile = &dataprotectionv1alpha1.CommonBackupPolicy{
		PersistentVolumeClaim: dataprotectionv1alpha1.PersistentVolumeClaim{
			CreatePolicy: dataprotectionv1alpha1.CreatePVCPolicyIfNotPresent,
		},
	}
	factory.backupType = dataprotectionv1alpha1.BackupTypeDataFile
	return factory
}

func (factory *MockBackupPolicyFactory) AddIncrementalPolicy() *MockBackupPolicyFactory {
	factory.get().Spec.Logfile = &dataprotectionv1alpha1.CommonBackupPolicy{
		PersistentVolumeClaim: dataprotectionv1alpha1.PersistentVolumeClaim{
			CreatePolicy: dataprotectionv1alpha1.CreatePVCPolicyIfNotPresent,
		},
	}
	factory.backupType = dataprotectionv1alpha1.BackupTypeLogFile
	return factory
}

func (factory *MockBackupPolicyFactory) SetBackupToolName(backupToolName string) *MockBackupPolicyFactory {
	factory.setCommonPolicyField(func(commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) {
		commonPolicy.BackupToolName = backupToolName
	})
	return factory
}

func (factory *MockBackupPolicyFactory) SetSchedule(schedule string, enable bool) *MockBackupPolicyFactory {
	factory.setScheduleField(func(schedulePolicy *dataprotectionv1alpha1.SchedulePolicy) {
		schedulePolicy.Enable = enable
		schedulePolicy.CronExpression = schedule
	})
	return factory
}

func (factory *MockBackupPolicyFactory) SetTTL(duration string) *MockBackupPolicyFactory {
	factory.get().Spec.Retention = &dataprotectionv1alpha1.RetentionSpec{
		TTL: &duration,
	}
	return factory
}

func (factory *MockBackupPolicyFactory) SetBackupsHistoryLimit(backupsHistoryLimit int32) *MockBackupPolicyFactory {
	factory.setBasePolicyField(func(basePolicy *dataprotectionv1alpha1.BasePolicy) {
		basePolicy.BackupsHistoryLimit = backupsHistoryLimit
	})
	return factory
}

func (factory *MockBackupPolicyFactory) AddMatchLabels(keyAndValues ...string) *MockBackupPolicyFactory {
	matchLabels := make(map[string]string)
	for k, v := range WithMap(keyAndValues...) {
		matchLabels[k] = v
	}
	factory.setBasePolicyField(func(basePolicy *dataprotectionv1alpha1.BasePolicy) {
		basePolicy.Target.LabelsSelector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
	})
	return factory
}

func (factory *MockBackupPolicyFactory) SetTargetSecretName(name string) *MockBackupPolicyFactory {
	factory.setBasePolicyField(func(basePolicy *dataprotectionv1alpha1.BasePolicy) {
		basePolicy.Target.Secret = &dataprotectionv1alpha1.BackupPolicySecret{Name: name}
	})
	return factory
}

func (factory *MockBackupPolicyFactory) SetHookContainerName(containerName string) *MockBackupPolicyFactory {
	snapshotPolicy := factory.get().Spec.Snapshot
	if snapshotPolicy == nil {
		return factory
	}
	snapshotPolicy.Hooks.ContainerName = containerName
	return factory
}

func (factory *MockBackupPolicyFactory) AddHookPreCommand(preCommand string) *MockBackupPolicyFactory {
	snapshotPolicy := factory.get().Spec.Snapshot
	if snapshotPolicy == nil {
		return factory
	}
	preCommands := &snapshotPolicy.Hooks.PreCommands
	*preCommands = append(*preCommands, preCommand)
	return factory
}

func (factory *MockBackupPolicyFactory) AddHookPostCommand(postCommand string) *MockBackupPolicyFactory {
	snapshotPolicy := factory.get().Spec.Snapshot
	if snapshotPolicy == nil {
		return factory
	}
	postCommands := &snapshotPolicy.Hooks.PostCommands
	*postCommands = append(*postCommands, postCommand)
	return factory
}

func (factory *MockBackupPolicyFactory) SetPVC(pvcName string) *MockBackupPolicyFactory {
	factory.setCommonPolicyField(func(commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) {
		commonPolicy.PersistentVolumeClaim.Name = pvcName
		commonPolicy.PersistentVolumeClaim.InitCapacity = resource.MustParse(constant.DefaultBackupPvcInitCapacity)
	})
	return factory
}
