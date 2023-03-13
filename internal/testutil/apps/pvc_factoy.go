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
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/constant"
)

type MockPersistentVolumeClaimFactory struct {
	BaseFactory[corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaim, MockPersistentVolumeClaimFactory]
}

func NewPersistentVolumeClaimFactory(namespace, name, clusterName, componentName, vctName string) *MockPersistentVolumeClaimFactory {
	f := &MockPersistentVolumeClaimFactory{}
	volumeMode := corev1.PersistentVolumeFilesystem
	f.init(namespace, name,
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					intctrlutil.AppInstanceLabelKey:             clusterName,
					intctrlutil.KBAppComponentLabelKey:          componentName,
					intctrlutil.AppManagedByLabelKey:            intctrlutil.AppName,
					intctrlutil.VolumeClaimTemplateNameLabelKey: vctName,
					intctrlutil.VolumeTypeLabelKey:              vctName,
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
	factory.get().Spec.StorageClassName = &storageClassName
	return factory
}

func (factory *MockPersistentVolumeClaimFactory) SetStorage(storageSize string) *MockPersistentVolumeClaimFactory {
	factory.get().Spec.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(storageSize),
		},
	}
	return factory
}

func (factory *MockPersistentVolumeClaimFactory) SetAnnotations(annotations map[string]string) *MockPersistentVolumeClaimFactory {
	factory.get().Annotations = annotations
	return factory
}
