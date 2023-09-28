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

package dataprotection

import (
	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

type MockRestoreFactory struct {
	testapps.BaseFactory[dpv1alpha1.Restore, *dpv1alpha1.Restore, MockRestoreFactory]
}

func NewRestoreJobFactory(namespace, name string) *MockRestoreFactory {
	f := &MockRestoreFactory{}
	// f.init(namespace, name,
	//	&dpv1alpha1.RestoreJob{
	//		Spec: dpv1alpha1.RestoreJobSpec{
	//			Target: dpv1alpha1.TargetCluster{
	//				LabelsSelector: &metav1.LabelSelector{
	//					MatchLabels: map[string]string{},
	//				},
	//			},
	//		},
	//	}, f)
	return f
}

func (factory *MockRestoreFactory) SetBackupName(backupName string) *MockRestoreFactory {
	// factory.get().Spec.Backup.Name = backupName
	return factory
}

func (factory *MockRestoreFactory) AddTargetMatchLabels(keyAndValues ...string) *MockRestoreFactory {
	// for k, v := range WithMap(keyAndValues...) {
	//	factory.get().Spec.Target.LabelsSelector.MatchLabels[k] = v
	// }
	return factory
}

func (factory *MockRestoreFactory) SetTargetSecretName(name string) *MockRestoreFactory {
	// factory.get().Spec.Target.Secret = &dataprotectionv1alpha1.BackupPolicySecret{Name: name}
	return factory
}

func (factory *MockRestoreFactory) AddTargetVolume(volume corev1.Volume) *MockRestoreFactory {
	// factory.get().Spec.TargetVolumes = append(factory.get().Spec.TargetVolumes, volume)
	return factory
}

func (factory *MockRestoreFactory) AddTargetVolumePVC(volumeName, pvcName string) *MockRestoreFactory {
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

func (factory *MockRestoreFactory) AddTargetVolumeMount(volumeMount corev1.VolumeMount) *MockRestoreFactory {
	// factory.get().Spec.TargetVolumeMounts = append(factory.get().Spec.TargetVolumeMounts, volumeMount)
	return factory
}
