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
	"time"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type MockBackupPolicyTemplateFactory struct {
	BaseFactory[dataprotectionv1alpha1.BackupPolicyTemplate, *dataprotectionv1alpha1.BackupPolicyTemplate, MockBackupPolicyTemplateFactory]
}

func NewBackupPolicyTemplateFactory(name string) *MockBackupPolicyTemplateFactory {
	f := &MockBackupPolicyTemplateFactory{}
	f.init("", name,
		&dataprotectionv1alpha1.BackupPolicyTemplate{
			Spec: dataprotectionv1alpha1.BackupPolicyTemplateSpec{
				Hooks: &dataprotectionv1alpha1.BackupPolicyHook{},
			},
		}, f)
	return f
}

func (factory *MockBackupPolicyTemplateFactory) SetBackupToolName(backupToolName string) *MockBackupPolicyTemplateFactory {
	factory.get().Spec.BackupToolName = backupToolName
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetSchedule(schedule string) *MockBackupPolicyTemplateFactory {
	factory.get().Spec.Schedule = schedule
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetTTL(duration string) *MockBackupPolicyTemplateFactory {
	du, err := time.ParseDuration(duration)
	gomega.Expect(err).Should(gomega.Succeed())

	var d metav1.Duration
	d.Duration = du
	factory.get().Spec.TTL = &d
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetHookContainerName(containerName string) *MockBackupPolicyTemplateFactory {
	factory.get().Spec.Hooks.ContainerName = containerName
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddHookPreCommand(preCommand string) *MockBackupPolicyTemplateFactory {
	preCommands := &factory.get().Spec.Hooks.PreCommands
	*preCommands = append(*preCommands, preCommand)
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddHookPostCommand(postCommand string) *MockBackupPolicyTemplateFactory {
	postCommands := &factory.get().Spec.Hooks.PostCommands
	*postCommands = append(*postCommands, postCommand)
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetCredentialKeyword(userKeyword, passwdKeyword string) *MockBackupPolicyTemplateFactory {
	factory.get().Spec.CredentialKeyword = &dataprotectionv1alpha1.BackupPolicyCredentialKeyword{
		UserKeyword:     userKeyword,
		PasswordKeyword: passwdKeyword,
	}
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetLabels(labels map[string]string) *MockBackupPolicyTemplateFactory {
	factory.get().SetLabels(labels)
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetPointInTimeRecovery(scripts *dataprotectionv1alpha1.ScriptSpec, configs map[string]string) *MockBackupPolicyTemplateFactory {
	pitr := dataprotectionv1alpha1.BackupPointInTimeRecovery{
		Scripts: scripts,
		Config:  configs,
	}
	factory.get().Spec.PointInTimeRecovery = &pitr
	return factory
}
