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

type MockBackupFactory struct {
	backup *dataprotectionv1alpha1.Backup
}

func NewBackupFactory(namespace, name string) *MockBackupFactory {
	return &MockBackupFactory{
		backup: &dataprotectionv1alpha1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Spec: dataprotectionv1alpha1.BackupSpec{},
		},
	}
}

func (factory *MockBackupFactory) WithRandomName() *MockBackupFactory {
	key := GetRandomizedKey("", factory.backup.Name)
	factory.backup.Name = key.Name
	return factory
}

func (factory *MockBackupFactory) AddLabels(keysAndValues ...string) *MockBackupFactory {
	factory.AddLabelsInMap(withMap(keysAndValues...))
	return factory
}

func (factory *MockBackupFactory) AddLabelsInMap(labels map[string]string) *MockBackupFactory {
	for k, v := range labels {
		factory.backup.Labels[k] = v
	}
	return factory
}

func (factory *MockBackupFactory) SetBackupPolicyName(backupPolicyName string) *MockBackupFactory {
	factory.backup.Spec.BackupPolicyName = backupPolicyName
	return factory
}

func (factory *MockBackupFactory) SetBackupType(backupType dataprotectionv1alpha1.BackupType) *MockBackupFactory {
	factory.backup.Spec.BackupType = backupType
	return factory
}

func (factory *MockBackupFactory) SetTTL(duration string) *MockBackupFactory {
	du, err := time.ParseDuration(duration)
	gomega.Expect(err).Should(gomega.Succeed())

	var d metav1.Duration
	d.Duration = du
	factory.backup.Spec.TTL = &d
	return factory
}

func (factory *MockBackupFactory) Create(testCtx *testutil.TestContext) *MockBackupFactory {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.backup)).Should(gomega.Succeed())
	return factory
}

func (factory *MockBackupFactory) CreateCli(ctx context.Context, cli client.Client) *MockBackupFactory {
	gomega.Expect(cli.Create(ctx, factory.backup)).Should(gomega.Succeed())
	return factory
}

func (factory *MockBackupFactory) GetBackup() *dataprotectionv1alpha1.Backup {
	return factory.backup
}
