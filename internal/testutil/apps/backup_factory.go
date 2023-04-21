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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type MockBackupFactory struct {
	BaseFactory[dataprotectionv1alpha1.Backup, *dataprotectionv1alpha1.Backup, MockBackupFactory]
}

func NewBackupFactory(namespace, name string) *MockBackupFactory {
	f := &MockBackupFactory{}
	f.init(namespace, name,
		&dataprotectionv1alpha1.Backup{
			Spec: dataprotectionv1alpha1.BackupSpec{},
		}, f)
	return f
}

func (factory *MockBackupFactory) SetBackupPolicyName(backupPolicyName string) *MockBackupFactory {
	factory.get().Spec.BackupPolicyName = backupPolicyName
	return factory
}

func (factory *MockBackupFactory) SetBackupType(backupType dataprotectionv1alpha1.BackupType) *MockBackupFactory {
	factory.get().Spec.BackupType = backupType
	return factory
}

func (factory *MockBackupFactory) SetLabels(labels map[string]string) *MockBackupFactory {
	factory.get().SetLabels(labels)
	return factory
}

func (factory *MockBackupFactory) SetBackLog(startTime, stopTime time.Time) *MockBackupFactory {
	manitests := factory.get().Status.Manifests
	if manitests == nil {
		manitests = &dataprotectionv1alpha1.ManifestsStatus{}
	}
	if manitests.BackupLog == nil {
		manitests.BackupLog = &dataprotectionv1alpha1.BackupLogStatus{}
	}
	manitests.BackupLog.StartTime = &metav1.Time{Time: startTime}
	manitests.BackupLog.StopTime = &metav1.Time{Time: stopTime}
	factory.get().Status.Manifests = manitests
	return factory
}
