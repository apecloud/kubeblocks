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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type MockBackupPolicyFactory struct {
	BaseFactory[dataprotectionv1alpha1.BackupPolicy, *dataprotectionv1alpha1.BackupPolicy, MockBackupPolicyFactory]
}

func NewBackupPolicyFactory(namespace, name string) *MockBackupPolicyFactory {
	f := &MockBackupPolicyFactory{}
	f.init(namespace, name,
		&dataprotectionv1alpha1.BackupPolicy{
			Spec: dataprotectionv1alpha1.BackupPolicySpec{
				Target: dataprotectionv1alpha1.TargetCluster{
					LabelsSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
				},
				Hooks: &dataprotectionv1alpha1.BackupPolicyHook{},
			},
		}, f)
	return f
}

func (factory *MockBackupPolicyFactory) SetBackupPolicyTplName(backupPolicyTplName string) *MockBackupPolicyFactory {
	factory.get().Spec.BackupPolicyTemplateName = backupPolicyTplName
	return factory
}

func (factory *MockBackupPolicyFactory) SetBackupToolName(backupToolName string) *MockBackupPolicyFactory {
	factory.get().Spec.BackupToolName = backupToolName
	return factory
}

func (factory *MockBackupPolicyFactory) SetSchedule(schedule string) *MockBackupPolicyFactory {
	factory.get().Spec.Schedule = schedule
	return factory
}

func (factory *MockBackupPolicyFactory) SetTTL(duration string) *MockBackupPolicyFactory {
	du, err := time.ParseDuration(duration)
	gomega.Expect(err).Should(gomega.Succeed())

	var d metav1.Duration
	d.Duration = du
	factory.get().Spec.TTL = &d
	return factory
}

func (factory *MockBackupPolicyFactory) SetBackupsHistoryLimit(backupsHistoryLimit int32) *MockBackupPolicyFactory {
	factory.get().Spec.BackupsHistoryLimit = backupsHistoryLimit
	return factory
}

func (factory *MockBackupPolicyFactory) AddMatchLabels(keyAndValues ...string) *MockBackupPolicyFactory {
	for k, v := range WithMap(keyAndValues...) {
		factory.get().Spec.Target.LabelsSelector.MatchLabels[k] = v
	}
	return factory
}

func (factory *MockBackupPolicyFactory) SetTargetSecretName(name string) *MockBackupPolicyFactory {
	factory.get().Spec.Target.Secret = &dataprotectionv1alpha1.BackupPolicySecret{}
	factory.get().Spec.Target.Secret.Name = name
	return factory
}

func (factory *MockBackupPolicyFactory) SetHookContainerName(containerName string) *MockBackupPolicyFactory {
	factory.get().Spec.Hooks.ContainerName = containerName
	return factory
}

func (factory *MockBackupPolicyFactory) AddHookPreCommand(preCommand string) *MockBackupPolicyFactory {
	preCommands := &factory.get().Spec.Hooks.PreCommands
	*preCommands = append(*preCommands, preCommand)
	return factory
}

func (factory *MockBackupPolicyFactory) AddHookPostCommand(postCommand string) *MockBackupPolicyFactory {
	postCommands := &factory.get().Spec.Hooks.PostCommands
	*postCommands = append(*postCommands, postCommand)
	return factory
}

func (factory *MockBackupPolicyFactory) SetRemoteVolume(volume corev1.Volume) *MockBackupPolicyFactory {
	factory.get().Spec.RemoteVolume = volume
	return factory
}

func (factory *MockBackupPolicyFactory) SetRemoteVolumePVC(volumeName, pvcName string) *MockBackupPolicyFactory {
	factory.get().Spec.RemoteVolume = corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	return factory
}
