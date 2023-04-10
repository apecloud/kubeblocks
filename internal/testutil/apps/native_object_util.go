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
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/testutil"
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

func CreateStorageClass(testCtx testutil.TestContext, storageClassName string, allowVolumeExpansion bool) *storagev1.StorageClass {
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClassName,
			Annotations: map[string]string{
				storage.IsDefaultStorageClassAnnotation: "false",
			},
		},
		Provisioner:          "kubernetes.io/no-provisioner",
		AllowVolumeExpansion: &allowVolumeExpansion,
	}
	return CreateK8sResource(testCtx, storageClass).(*storagev1.StorageClass)
}
