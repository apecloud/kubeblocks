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

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

type MockRestoreJobFactory struct {
	restoreJob *dataprotectionv1alpha1.RestoreJob
}

func NewRestoreJobFactory(namespace, name string) *MockRestoreJobFactory {
	return &MockRestoreJobFactory{
		restoreJob: &dataprotectionv1alpha1.RestoreJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Spec: dataprotectionv1alpha1.RestoreJobSpec{
				Target: dataprotectionv1alpha1.TargetCluster{
					LabelsSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
				},
			},
		},
	}
}

func (factory *MockRestoreJobFactory) WithRandomName() *MockRestoreJobFactory {
	key := GetRandomizedKey("", factory.restoreJob.Name)
	factory.restoreJob.Name = key.Name
	return factory
}

func (factory *MockRestoreJobFactory) AddLabels(keysAndValues ...string) *MockRestoreJobFactory {
	factory.AddLabelsInMap(withMap(keysAndValues...))
	return factory
}

func (factory *MockRestoreJobFactory) AddLabelsInMap(labels map[string]string) *MockRestoreJobFactory {
	for k, v := range labels {
		factory.restoreJob.Labels[k] = v
	}
	return factory
}

func (factory *MockRestoreJobFactory) SetBackupJobName(backupJobName string) *MockRestoreJobFactory {
	factory.restoreJob.Spec.BackupJobName = backupJobName
	return factory
}

func (factory *MockRestoreJobFactory) AddTargetMatchLabels(keyAndValues ...string) *MockRestoreJobFactory {
	for k, v := range withMap(keyAndValues...) {
		factory.restoreJob.Spec.Target.LabelsSelector.MatchLabels[k] = v
	}
	return factory
}

func (factory *MockRestoreJobFactory) SetTargetSecretName(name string) *MockRestoreJobFactory {
	factory.restoreJob.Spec.Target.Secret.Name = name
	return factory
}

func (factory *MockRestoreJobFactory) AddTargetVolume(volume corev1.Volume) *MockRestoreJobFactory {
	factory.restoreJob.Spec.TargetVolumes = append(factory.restoreJob.Spec.TargetVolumes, volume)
	return factory
}

func (factory *MockRestoreJobFactory) AddTargetVolumePVC(volumeName, pvcName string) *MockRestoreJobFactory {
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	factory.AddTargetVolume(volume)
	return factory
}

func (factory *MockRestoreJobFactory) AddTargetVolumeMount(volumeMount corev1.VolumeMount) *MockRestoreJobFactory {
	factory.restoreJob.Spec.TargetVolumeMounts = append(factory.restoreJob.Spec.TargetVolumeMounts, volumeMount)
	return factory
}

func (factory *MockRestoreJobFactory) Create(testCtx *testutil.TestContext) *MockRestoreJobFactory {
	gomega.Expect(testCtx.CreateObj(testCtx.Ctx, factory.restoreJob)).Should(gomega.Succeed())
	return factory
}

func (factory *MockRestoreJobFactory) CreateCli(ctx context.Context, cli client.Client) *MockRestoreJobFactory {
	gomega.Expect(cli.Create(ctx, factory.restoreJob)).Should(gomega.Succeed())
	return factory
}

func (factory *MockRestoreJobFactory) GetRestoreJob() *dataprotectionv1alpha1.RestoreJob {
	return factory.restoreJob
}
