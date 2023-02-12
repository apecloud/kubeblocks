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

package dbaas

import (
	"context"
	"time"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

type MockBackupPolicyTemplateFactory struct {
	backupPolicyTpl *dataprotectionv1alpha1.BackupPolicyTemplate
}

func NewBackupPolicyTemplateFactory(name string) *MockBackupPolicyTemplateFactory {
	return &MockBackupPolicyTemplateFactory{
		backupPolicyTpl: &dataprotectionv1alpha1.BackupPolicyTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{},
			},
			Spec: dataprotectionv1alpha1.BackupPolicyTemplateSpec{
				Hooks: &dataprotectionv1alpha1.BackupPolicyHook{},
			},
		},
	}
}

func (factory *MockBackupPolicyTemplateFactory) WithRandomName() *MockBackupPolicyTemplateFactory {
	key := GetRandomizedKey("", factory.backupPolicyTpl.Name)
	factory.backupPolicyTpl.Name = key.Name
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddLabels(keysAndValues ...string) *MockBackupPolicyTemplateFactory {
	factory.AddLabelsInMap(withMap(keysAndValues...))
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddLabelsInMap(labels map[string]string) *MockBackupPolicyTemplateFactory {
	for k, v := range labels {
		factory.backupPolicyTpl.Labels[k] = v
	}
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetBackupToolName(backupToolName string) *MockBackupPolicyTemplateFactory {
	factory.backupPolicyTpl.Spec.BackupToolName = backupToolName
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetSchedule(schedule string) *MockBackupPolicyTemplateFactory {
	factory.backupPolicyTpl.Spec.Schedule = schedule
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetTTL(duration string) *MockBackupPolicyTemplateFactory {
	du, err := time.ParseDuration(duration)
	gomega.Expect(err).Should(gomega.Succeed())

	var d metav1.Duration
	d.Duration = du
	factory.backupPolicyTpl.Spec.TTL = &d
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetHookContainerName(containerName string) *MockBackupPolicyTemplateFactory {
	factory.backupPolicyTpl.Spec.Hooks.ContainerName = containerName
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddHookPreCommand(preCommand string) *MockBackupPolicyTemplateFactory {
	preCommands := &factory.backupPolicyTpl.Spec.Hooks.PreCommands
	*preCommands = append(*preCommands, preCommand)
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) AddHookPostCommand(postCommand string) *MockBackupPolicyTemplateFactory {
	postCommands := &factory.backupPolicyTpl.Spec.Hooks.PostCommands
	*postCommands = append(*postCommands, postCommand)
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) SetCredentialKeyword(userKeyword, passwdKeyword string) *MockBackupPolicyTemplateFactory {
	factory.backupPolicyTpl.Spec.CredentialKeyword = &dataprotectionv1alpha1.BackupPolicyCredentialKeyword{
		UserKeyword:     userKeyword,
		PasswordKeyword: passwdKeyword,
	}
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) Create(testCtx *testutil.TestContext) *MockBackupPolicyTemplateFactory {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.backupPolicyTpl)).Should(gomega.Succeed())
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) CreateCli(ctx context.Context, cli client.Client) *MockBackupPolicyTemplateFactory {
	gomega.Expect(cli.Create(ctx, factory.backupPolicyTpl)).Should(gomega.Succeed())
	return factory
}

func (factory *MockBackupPolicyTemplateFactory) GetBackupPolicyTpl() *dataprotectionv1alpha1.BackupPolicyTemplate {
	return factory.backupPolicyTpl
}
