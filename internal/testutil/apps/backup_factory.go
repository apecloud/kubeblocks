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
