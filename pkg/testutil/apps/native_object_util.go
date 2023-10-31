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
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/testutil"
)

// ConfigMap

func NewConfigMap(namespace, name string, options ...any) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{},
	}
	for _, option := range options {
		switch f := option.(type) {
		case func(*corev1.ConfigMap):
			f(cm)
		case func(object client.Object):
			f(cm)
		}
	}
	return cm
}

func SetConfigMapData(key string, value string) func(*corev1.ConfigMap) {
	return func(configMap *corev1.ConfigMap) {
		configMap.Data[key] = value
	}
}

func NewPVC(size string) corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(size),
			},
		},
	}
}

func CreateStorageClass(testCtx *testutil.TestContext, storageClassName string,
	allowVolumeExpansion bool) *storagev1.StorageClass {
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClassName,
			Annotations: map[string]string{
				storage.IsDefaultStorageClassAnnotation: "false",
			},
		},
		Provisioner:          testutil.DefaultStorageProvisoner,
		AllowVolumeExpansion: &allowVolumeExpansion,
	}
	return CreateK8sResource(testCtx, storageClass).(*storagev1.StorageClass)
}

func CreateVolumeSnapshotClass(testCtx *testutil.TestContext) {
	volumeSnapshotClass := &vsv1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-vs",
		},
		Driver:         testutil.DefaultCSIDriver,
		DeletionPolicy: vsv1.VolumeSnapshotContentDelete,
	}
	CreateK8sResource(testCtx, volumeSnapshotClass)
}
