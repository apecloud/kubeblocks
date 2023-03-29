/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
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
	case dataprotectionv1alpha1.BackupTypeFull:
		basePolicy = &factory.get().Spec.Full.BasePolicy
	case dataprotectionv1alpha1.BackupTypeIncremental:
		basePolicy = &factory.get().Spec.Incremental.BasePolicy
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
	case dataprotectionv1alpha1.BackupTypeFull:
		commonPolicy = factory.get().Spec.Full
	case dataprotectionv1alpha1.BackupTypeIncremental:
		commonPolicy = factory.get().Spec.Incremental
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
	case dataprotectionv1alpha1.BackupTypeFull, dataprotectionv1alpha1.BackupTypeSnapshot:
		factory.get().Spec.Schedule.BaseBackup = &dataprotectionv1alpha1.BaseBackupSchedulePolicy{
			SchedulePolicy: dataprotectionv1alpha1.SchedulePolicy{},
			Type:           dataprotectionv1alpha1.BaseBackupType(factory.backupType),
		}
		schedulePolicy = &factory.get().Spec.Schedule.BaseBackup.SchedulePolicy
	case dataprotectionv1alpha1.BackupTypeIncremental:
		schedulePolicy = &dataprotectionv1alpha1.SchedulePolicy{}
		factory.get().Spec.Schedule.Incremental = schedulePolicy
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
	factory.get().Spec.Full = &dataprotectionv1alpha1.CommonBackupPolicy{}
	factory.backupType = dataprotectionv1alpha1.BackupTypeFull
	return factory
}

func (factory *MockBackupPolicyFactory) AddIncrementalPolicy() *MockBackupPolicyFactory {
	factory.get().Spec.Incremental = &dataprotectionv1alpha1.CommonBackupPolicy{}
	factory.backupType = dataprotectionv1alpha1.BackupTypeIncremental
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
	factory.get().Spec.TTL = &duration
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

func (factory *MockBackupPolicyFactory) SetRemoteVolume(volume corev1.Volume) *MockBackupPolicyFactory {
	factory.setCommonPolicyField(func(commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) {
		commonPolicy.RemoteVolume = volume
	})
	return factory
}

func (factory *MockBackupPolicyFactory) SetRemoteVolumePVC(volumeName, pvcName string) *MockBackupPolicyFactory {
	factory.setCommonPolicyField(func(commonPolicy *dataprotectionv1alpha1.CommonBackupPolicy) {
		commonPolicy.RemoteVolume = corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		}
	})
	return factory
}
