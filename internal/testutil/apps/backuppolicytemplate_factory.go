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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type MockBackupPolicyTemplateFactory struct {
	BaseFactory[appsv1alpha1.BackupPolicyTemplate, *appsv1alpha1.BackupPolicyTemplate, MockBackupPolicyTemplateFactory]
	backupType dataprotectionv1alpha1.BackupType
}

func NewBackupPolicyTemplateFactory(name string) *MockBackupPolicyTemplateFactory {
	f := &MockBackupPolicyTemplateFactory{}
	f.init("", name,
		&appsv1alpha1.BackupPolicyTemplate{},
		f)
	return f
}

func (factory *MockBackupPolicyTemplateFactory) SetClusterDefRef(clusterDefRef string) *MockBackupPolicyTemplateFactory {
	factory.get().Spec.ClusterDefRef = clusterDefRef
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) getLastBackupPolicy() *appsv1alpha1.BackupPolicy {
	l := len(factory.get().Spec.BackupPolicies)
	if l == 0 {
		return nil
	}
	backupPolicies := factory.get().Spec.BackupPolicies
	return &backupPolicies[l-1]
}

func (factory *MockBackupPolicyTemplateFactory) AddBackupPolicy(componentDef string) *MockBackupPolicyTemplateFactory {
	factory.get().Spec.BackupPolicies = append(factory.get().Spec.BackupPolicies, appsv1alpha1.BackupPolicy{
		ComponentDefRef: componentDef,
	})
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetTTL(duration string) *MockBackupPolicyTemplateFactory {
	factory.getLastBackupPolicy().Retention = &appsv1alpha1.RetentionSpec{
		TTL: &duration,
	}
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) setBasePolicyField(setField func(basePolicy *appsv1alpha1.BasePolicy)) {
	backupPolicy := factory.getLastBackupPolicy()
	var basePolicy *appsv1alpha1.BasePolicy
	switch factory.backupType {
	case dataprotectionv1alpha1.BackupTypeDataFile:
		basePolicy = &backupPolicy.Datafile.BasePolicy
	case dataprotectionv1alpha1.BackupTypeLogFile:
		basePolicy = &backupPolicy.Logfile.BasePolicy
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		basePolicy = &backupPolicy.Snapshot.BasePolicy
	}
	if basePolicy == nil {
		// ignore
		return
	}
	setField(basePolicy)
}

func (factory *MockBackupPolicyTemplateFactory) setCommonPolicyField(setField func(commonPolicy *appsv1alpha1.CommonBackupPolicy)) {
	backupPolicy := factory.getLastBackupPolicy()
	var commonPolicy *appsv1alpha1.CommonBackupPolicy
	switch factory.backupType {
	case dataprotectionv1alpha1.BackupTypeDataFile:
		commonPolicy = backupPolicy.Datafile
	case dataprotectionv1alpha1.BackupTypeLogFile:
		commonPolicy = backupPolicy.Logfile
	}
	if commonPolicy == nil {
		// ignore
		return
	}
	setField(commonPolicy)
}

func (factory *MockBackupPolicyTemplateFactory) setScheduleField(setField func(schedulePolicy *appsv1alpha1.SchedulePolicy)) {
	backupPolicy := factory.getLastBackupPolicy()
	var schedulePolicy *appsv1alpha1.SchedulePolicy
	switch factory.backupType {
	case dataprotectionv1alpha1.BackupTypeSnapshot:
		backupPolicy.Schedule.Snapshot = &appsv1alpha1.SchedulePolicy{}
		schedulePolicy = backupPolicy.Schedule.Snapshot
	case dataprotectionv1alpha1.BackupTypeDataFile:
		backupPolicy.Schedule.Datafile = &appsv1alpha1.SchedulePolicy{}
		schedulePolicy = backupPolicy.Schedule.Datafile
	case dataprotectionv1alpha1.BackupTypeLogFile:
		backupPolicy.Schedule.Logfile = &appsv1alpha1.SchedulePolicy{}
		schedulePolicy = backupPolicy.Schedule.Logfile
	}
	if schedulePolicy == nil {
		// ignore
		return
	}
	setField(schedulePolicy)
}

func (factory *MockBackupPolicyTemplateFactory) AddSnapshotPolicy() *MockBackupPolicyTemplateFactory {
	backupPolicy := factory.getLastBackupPolicy()
	backupPolicy.Snapshot = &appsv1alpha1.SnapshotPolicy{
		Hooks: &appsv1alpha1.BackupPolicyHook{},
	}
	factory.backupType = dataprotectionv1alpha1.BackupTypeSnapshot
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddDatafilePolicy() *MockBackupPolicyTemplateFactory {
	backupPolicy := factory.getLastBackupPolicy()
	backupPolicy.Datafile = &appsv1alpha1.CommonBackupPolicy{}
	factory.backupType = dataprotectionv1alpha1.BackupTypeDataFile
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddIncrementalPolicy() *MockBackupPolicyTemplateFactory {
	backupPolicy := factory.getLastBackupPolicy()
	backupPolicy.Logfile = &appsv1alpha1.CommonBackupPolicy{}
	factory.backupType = dataprotectionv1alpha1.BackupTypeLogFile
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetHookContainerName(containerName string) *MockBackupPolicyTemplateFactory {
	backupPolicy := factory.getLastBackupPolicy()
	if backupPolicy.Snapshot == nil {
		return factory
	}
	backupPolicy.Snapshot.Hooks.ContainerName = containerName
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddHookPreCommand(preCommand string) *MockBackupPolicyTemplateFactory {
	backupPolicy := factory.getLastBackupPolicy()
	if backupPolicy.Snapshot == nil {
		return factory
	}
	preCommands := &backupPolicy.Snapshot.Hooks.PreCommands
	*preCommands = append(*preCommands, preCommand)
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddHookPostCommand(postCommand string) *MockBackupPolicyTemplateFactory {
	backupPolicy := factory.getLastBackupPolicy()
	if backupPolicy.Snapshot == nil {
		return factory
	}
	postCommands := &backupPolicy.Snapshot.Hooks.PostCommands
	*postCommands = append(*postCommands, postCommand)
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetSchedule(schedule string, enable bool) *MockBackupPolicyTemplateFactory {
	factory.setScheduleField(func(schedulePolicy *appsv1alpha1.SchedulePolicy) {
		schedulePolicy.Enable = enable
		schedulePolicy.CronExpression = schedule
	})
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetBackupsHistoryLimit(backupsHistoryLimit int32) *MockBackupPolicyTemplateFactory {
	factory.setBasePolicyField(func(basePolicy *appsv1alpha1.BasePolicy) {
		basePolicy.BackupsHistoryLimit = backupsHistoryLimit
	})
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetBackupToolName(backupToolName string) *MockBackupPolicyTemplateFactory {
	factory.setCommonPolicyField(func(commonPolicy *appsv1alpha1.CommonBackupPolicy) {
		commonPolicy.BackupToolName = backupToolName
	})
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetTargetRole(role string) *MockBackupPolicyTemplateFactory {
	factory.setBasePolicyField(func(basePolicy *appsv1alpha1.BasePolicy) {
		basePolicy.Target.Role = role
	})
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetTargetAccount(account string) *MockBackupPolicyTemplateFactory {
	factory.setBasePolicyField(func(basePolicy *appsv1alpha1.BasePolicy) {
		basePolicy.Target.Account = account
	})
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetLabels(labels map[string]string) *MockBackupPolicyTemplateFactory {
	factory.get().SetLabels(labels)
	return factory
}
