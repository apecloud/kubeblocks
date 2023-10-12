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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

type MockRestoreFactory struct {
	testapps.BaseFactory[dpv1alpha1.Restore, *dpv1alpha1.Restore, MockRestoreFactory]
}

func NewRestoreactory(namespace, name string) *MockRestoreFactory {
	f := &MockRestoreFactory{}
	f.Init(namespace, name,
		&dpv1alpha1.Restore{
			Spec: dpv1alpha1.RestoreSpec{},
		}, f)
	return f
}

func (f *MockRestoreFactory) SetBackup(name, namespace string) *MockRestoreFactory {
	f.Get().Spec.Backup = dpv1alpha1.BackupRef{
		Name:      name,
		Namespace: namespace,
	}
	return f
}

func (f *MockRestoreFactory) SetRestoreTime(restoreTime string) *MockRestoreFactory {
	f.Get().Spec.RestoreTime = restoreTime
	return f
}

func (f *MockRestoreFactory) SetLabels(labels map[string]string) *MockRestoreFactory {
	f.Get().SetLabels(labels)
	return f
}

func (f *MockRestoreFactory) AddEnv(env corev1.EnvVar) *MockRestoreFactory {
	f.Get().Spec.Env = append(f.Get().Spec.Env, env)
	return f
}

func (f *MockRestoreFactory) SetDataSourceRef(volumeSource, mountPath string) *MockRestoreFactory {
	prepareDataConfig := f.Get().Spec.PrepareDataConfig
	if prepareDataConfig == nil {
		prepareDataConfig = &dpv1alpha1.PrepareDataConfig{
			VolumeClaimManagementPolicy: dpv1alpha1.ParallelManagementPolicy,
		}
	}
	prepareDataConfig.DataSourceRef = &dpv1alpha1.VolumeConfig{
		VolumeSource: volumeSource,
		MountPath:    mountPath,
	}
	f.Get().Spec.PrepareDataConfig = prepareDataConfig
	return f
}

func (f *MockRestoreFactory) SetVolumeClaimsTemplate(templateName, volumeSource, mountPath, storageClass string, replicas, startingIndex int32) *MockRestoreFactory {
	prepareDataConfig := f.Get().Spec.PrepareDataConfig
	if prepareDataConfig == nil {
		prepareDataConfig = &dpv1alpha1.PrepareDataConfig{}
	}
	prepareDataConfig.RestoreVolumeClaimsTemplate = &dpv1alpha1.RestoreVolumeClaimsTemplate{
		Replicas:      replicas,
		StartingIndex: startingIndex,
		Templates: []dpv1alpha1.RestoreVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: templateName,
				},
				VolumeConfig: dpv1alpha1.VolumeConfig{
					VolumeSource: volumeSource,
					MountPath:    mountPath,
				},
				VolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &storageClass,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("20Gi"),
						},
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			},
		},
	}
	return f
}
