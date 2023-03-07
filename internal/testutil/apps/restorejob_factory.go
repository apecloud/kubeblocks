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

type MockRestoreJobFactory struct {
	BaseFactory[dataprotectionv1alpha1.RestoreJob, *dataprotectionv1alpha1.RestoreJob, MockRestoreJobFactory]
}

func NewRestoreJobFactory(namespace, name string) *MockRestoreJobFactory {
	f := &MockRestoreJobFactory{}
	f.init(namespace, name,
		&dataprotectionv1alpha1.RestoreJob{
			Spec: dataprotectionv1alpha1.RestoreJobSpec{
				Target: dataprotectionv1alpha1.TargetCluster{
					LabelsSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
				},
			},
		}, f)
	return f
}

func (factory *MockRestoreJobFactory) SetBackupJobName(backupJobName string) *MockRestoreJobFactory {
	factory.get().Spec.BackupJobName = backupJobName
	return factory
}

func (factory *MockRestoreJobFactory) AddTargetMatchLabels(keyAndValues ...string) *MockRestoreJobFactory {
	for k, v := range WithMap(keyAndValues...) {
		factory.get().Spec.Target.LabelsSelector.MatchLabels[k] = v
	}
	return factory
}

func (factory *MockRestoreJobFactory) SetTargetSecretName(name string) *MockRestoreJobFactory {
	factory.get().Spec.Target.Secret = &dataprotectionv1alpha1.BackupPolicySecret{}
	factory.get().Spec.Target.Secret.Name = name
	return factory
}

func (factory *MockRestoreJobFactory) AddTargetVolume(volume corev1.Volume) *MockRestoreJobFactory {
	factory.get().Spec.TargetVolumes = append(factory.get().Spec.TargetVolumes, volume)
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
	factory.get().Spec.TargetVolumeMounts = append(factory.get().Spec.TargetVolumeMounts, volumeMount)
	return factory
}
