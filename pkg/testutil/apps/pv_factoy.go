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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MockPersistentVolumeFactory struct {
	BaseFactory[corev1.PersistentVolume, *corev1.PersistentVolume, MockPersistentVolumeFactory]
}

func NewPersistentVolumeFactory(namespace, name, pvcName string) *MockPersistentVolumeFactory {
	f := &MockPersistentVolumeFactory{}
	volumeMode := corev1.PersistentVolumeFilesystem
	f.Init(namespace, name,
		&corev1.PersistentVolume{
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/tmp/hostpath-provisioner/default/" + pvcName,
					},
				},
				VolumeMode:                    &volumeMode,
				AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			},
		}, f)
	return f
}

func (f *MockPersistentVolumeFactory) SetStorageClass(storageClassName string) *MockPersistentVolumeFactory {
	f.Get().Spec.StorageClassName = storageClassName
	return f
}

func (f *MockPersistentVolumeFactory) SetStorage(storageSize string) *MockPersistentVolumeFactory {
	f.Get().Spec.Capacity = corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse(storageSize),
	}
	return f
}

func (f *MockPersistentVolumeFactory) SetPersistentVolumeReclaimPolicy(reclaimPolicy corev1.PersistentVolumeReclaimPolicy) *MockPersistentVolumeFactory {
	f.Get().Spec.PersistentVolumeReclaimPolicy = reclaimPolicy
	return f
}

func (f *MockPersistentVolumeFactory) SetClaimRef(obj client.Object) *MockPersistentVolumeFactory {
	f.Get().Spec.ClaimRef = &corev1.ObjectReference{
		Kind:            obj.GetObjectKind().GroupVersionKind().Kind,
		Namespace:       obj.GetNamespace(),
		Name:            obj.GetName(),
		UID:             obj.GetUID(),
		APIVersion:      "v1",
		ResourceVersion: obj.GetResourceVersion(),
	}
	return f
}

func (f *MockPersistentVolumeFactory) SetCSIDriver(driverName string) *MockPersistentVolumeFactory {
	f.Get().Spec.CSI = &corev1.CSIPersistentVolumeSource{
		Driver:       driverName,
		VolumeHandle: f.Get().Name,
	}
	// clear default persistentVolumeSource
	f.Get().Spec.PersistentVolumeSource.HostPath = nil
	return f
}
