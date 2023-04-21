/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	factory.get().Spec.Target.Secret = &dataprotectionv1alpha1.BackupPolicySecret{Name: name}
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
