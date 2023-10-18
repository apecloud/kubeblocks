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

package apps

import (
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

type MockPersistentVolumeClaimFactory struct {
	BaseFactory[corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaim, MockPersistentVolumeClaimFactory]
}

func NewPersistentVolumeClaimFactory(namespace, name, clusterName, componentName, vctName string) *MockPersistentVolumeClaimFactory {
	f := &MockPersistentVolumeClaimFactory{}
	volumeMode := corev1.PersistentVolumeFilesystem
	f.Init(namespace, name,
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constant.AppInstanceLabelKey:             clusterName,
					constant.KBAppComponentLabelKey:          componentName,
					constant.AppManagedByLabelKey:            constant.AppName,
					constant.VolumeClaimTemplateNameLabelKey: vctName,
					constant.VolumeTypeLabelKey:              vctName,
				},
				Annotations: map[string]string{
					kube.KubeAnnBindCompleted: "yes",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				VolumeMode:  &volumeMode,
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
		}, f)
	return f
}

func (factory *MockPersistentVolumeClaimFactory) SetStorageClass(storageClassName string) *MockPersistentVolumeClaimFactory {
	factory.Get().Spec.StorageClassName = &storageClassName
	return factory
}

func (factory *MockPersistentVolumeClaimFactory) SetStorage(storageSize string) *MockPersistentVolumeClaimFactory {
	factory.Get().Spec.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(storageSize),
		},
	}
	return factory
}

func (factory *MockPersistentVolumeClaimFactory) SetVolumeName(volName string) *MockPersistentVolumeClaimFactory {
	factory.Get().Spec.VolumeName = volName
	return factory
}

func (factory *MockPersistentVolumeClaimFactory) SetAnnotations(annotations map[string]string) *MockPersistentVolumeClaimFactory {
	factory.Get().Annotations = annotations
	return factory
}

func (factory *MockPersistentVolumeClaimFactory) SetDataSourceRef(apiGroup, kind, name string) *MockPersistentVolumeClaimFactory {
	factory.Get().Spec.DataSourceRef = &corev1.TypedObjectReference{
		Name:     name,
		APIGroup: &apiGroup,
		Kind:     kind,
	}
	return factory
}
