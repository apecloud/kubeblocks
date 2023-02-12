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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

type MockBackupPolicyFactory struct {
	backupPolicy *dataprotectionv1alpha1.BackupPolicy
}

func NewBackupPolicyFactory(namespace, name string) *MockBackupPolicyFactory {
	return &MockBackupPolicyFactory{
		backupPolicy: &dataprotectionv1alpha1.BackupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Spec: dataprotectionv1alpha1.BackupPolicySpec{
				Target: dataprotectionv1alpha1.TargetCluster{
					LabelsSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
				},
				Hooks: &dataprotectionv1alpha1.BackupPolicyHook{},
			},
		},
	}
}

func (factory *MockBackupPolicyFactory) WithRandomName() *MockBackupPolicyFactory {
	key := GetRandomizedKey("", factory.backupPolicy.Name)
	factory.backupPolicy.Name = key.Name
	return factory
}

func (factory *MockBackupPolicyFactory) AddLabels(keysAndValues ...string) *MockBackupPolicyFactory {
	factory.AddLabelsInMap(withMap(keysAndValues...))
	return factory
}

func (factory *MockBackupPolicyFactory) AddLabelsInMap(labels map[string]string) *MockBackupPolicyFactory {
	for k, v := range labels {
		factory.backupPolicy.Labels[k] = v
	}
	return factory
}

func (factory *MockBackupPolicyFactory) SetBackupPolicyTplName(backupPolicyTplName string) *MockBackupPolicyFactory {
	factory.backupPolicy.Spec.BackupPolicyTemplateName = backupPolicyTplName
	return factory
}

func (factory *MockBackupPolicyFactory) SetBackupToolName(backupToolName string) *MockBackupPolicyFactory {
	factory.backupPolicy.Spec.BackupToolName = backupToolName
	return factory
}

func (factory *MockBackupPolicyFactory) SetSchedule(schedule string) *MockBackupPolicyFactory {
	factory.backupPolicy.Spec.Schedule = schedule
	return factory
}

func (factory *MockBackupPolicyFactory) SetTTL(duration string) *MockBackupPolicyFactory {
	du, err := time.ParseDuration(duration)
	gomega.Expect(err).Should(gomega.Succeed())

	var d metav1.Duration
	d.Duration = du
	factory.backupPolicy.Spec.TTL = &d
	return factory
}

func (factory *MockBackupPolicyFactory) SetBackupsHistoryLimit(backupsHistoryLimit int32) *MockBackupPolicyFactory {
	factory.backupPolicy.Spec.BackupsHistoryLimit = backupsHistoryLimit
	return factory
}

func (factory *MockBackupPolicyFactory) AddMatchLabels(keyAndValues ...string) *MockBackupPolicyFactory {
	for k, v := range withMap(keyAndValues...) {
		factory.backupPolicy.Spec.Target.LabelsSelector.MatchLabels[k] = v
	}
	return factory
}

func (factory *MockBackupPolicyFactory) SetTargetSecretName(name string) *MockBackupPolicyFactory {
	factory.backupPolicy.Spec.Target.Secret.Name = name
	return factory
}

func (factory *MockBackupPolicyFactory) SetHookContainerName(containerName string) *MockBackupPolicyFactory {
	factory.backupPolicy.Spec.Hooks.ContainerName = containerName
	return factory
}

func (factory *MockBackupPolicyFactory) AddHookPreCommand(preCommand string) *MockBackupPolicyFactory {
	preCommands := &factory.backupPolicy.Spec.Hooks.PreCommands
	*preCommands = append(*preCommands, preCommand)
	return factory
}

func (factory *MockBackupPolicyFactory) AddHookPostCommand(postCommand string) *MockBackupPolicyFactory {
	postCommands := &factory.backupPolicy.Spec.Hooks.PostCommands
	*postCommands = append(*postCommands, postCommand)
	return factory
}

func (factory *MockBackupPolicyFactory) SetRemoteVolume(volume corev1.Volume) *MockBackupPolicyFactory {
	factory.backupPolicy.Spec.RemoteVolume = volume
	return factory
}

func (factory *MockBackupPolicyFactory) SetRemoteVolumePVC(volumeName, pvcName string) *MockBackupPolicyFactory {
	factory.backupPolicy.Spec.RemoteVolume = corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	return factory
}

func (factory *MockBackupPolicyFactory) Create(testCtx *testutil.TestContext) *MockBackupPolicyFactory {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.backupPolicy)).Should(gomega.Succeed())
	return factory
}

func (factory *MockBackupPolicyFactory) CreateCli(ctx context.Context, cli client.Client) *MockBackupPolicyFactory {
	gomega.Expect(cli.Create(ctx, factory.backupPolicy)).Should(gomega.Succeed())
	return factory
}

func (factory *MockBackupPolicyFactory) GetBackupPolicy() *dataprotectionv1alpha1.BackupPolicy {
	return factory.backupPolicy
}
