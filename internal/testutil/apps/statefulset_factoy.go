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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type MockStatefulSetFactory struct {
	BaseFactory[appsv1.StatefulSet, *appsv1.StatefulSet, MockStatefulSetFactory]
}

func NewStatefulSetFactory(namespace, name string, clusterName string, componentName string) *MockStatefulSetFactory {
	f := &MockStatefulSetFactory{}
	f.init(namespace, name,
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					intctrlutil.AppInstanceLabelKey:  clusterName,
					intctrlutil.AppComponentLabelKey: componentName,
					intctrlutil.AppManagedByLabelKey: intctrlutil.AppName,
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						intctrlutil.AppInstanceLabelKey:  clusterName,
						intctrlutil.AppComponentLabelKey: componentName,
						intctrlutil.AppManagedByLabelKey: intctrlutil.AppName,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							intctrlutil.AppInstanceLabelKey:  clusterName,
							intctrlutil.AppComponentLabelKey: componentName,
							intctrlutil.AppManagedByLabelKey: intctrlutil.AppName,
						},
					},
				},
			},
		}, f)
	return f
}

func (factory *MockStatefulSetFactory) SetReplicas(replicas int32) *MockStatefulSetFactory {
	factory.get().Spec.Replicas = &replicas
	return factory
}

func (factory *MockStatefulSetFactory) AddVolume(volume corev1.Volume) *MockStatefulSetFactory {
	volumes := &factory.get().Spec.Template.Spec.Volumes
	*volumes = append(*volumes, volume)
	return factory
}

func (factory *MockStatefulSetFactory) AddConfigmapVolume(volumeName string, configmapName string) *MockStatefulSetFactory {
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configmapName},
			},
		},
	}
	factory.AddVolume(volume)
	return factory
}

func (factory *MockStatefulSetFactory) AddVolumeClaimTemplate(pvc corev1.PersistentVolumeClaim) *MockStatefulSetFactory {
	volumeClaimTpls := &factory.get().Spec.VolumeClaimTemplates
	*volumeClaimTpls = append(*volumeClaimTpls, pvc)
	return factory
}

func (factory *MockStatefulSetFactory) AddContainer(container corev1.Container) *MockStatefulSetFactory {
	containers := &factory.get().Spec.Template.Spec.Containers
	*containers = append(*containers, container)
	return factory
}
