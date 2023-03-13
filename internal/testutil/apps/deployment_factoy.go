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

	"github.com/apecloud/kubeblocks/internal/constant"
)

type MockDeploymentFactory struct {
	BaseFactory[appsv1.Deployment, *appsv1.Deployment, MockDeploymentFactory]
}

func NewDeploymentFactory(namespace, name, clusterName, componentName string) *MockDeploymentFactory {
	f := &MockDeploymentFactory{}
	f.init(namespace, name,
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: componentName,
					constant.AppManagedByLabelKey:   constant.AppName,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey:    clusterName,
						constant.KBAppComponentLabelKey: componentName,
						constant.AppManagedByLabelKey:   constant.AppName,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							constant.AppInstanceLabelKey:    clusterName,
							constant.KBAppComponentLabelKey: componentName,
							constant.AppManagedByLabelKey:   constant.AppName,
						},
					},
				},
			},
		}, f)
	return f
}

func (factory *MockDeploymentFactory) SetMinReadySeconds(minReadySeconds int32) *MockDeploymentFactory {
	factory.get().Spec.MinReadySeconds = minReadySeconds
	return factory
}

func (factory *MockDeploymentFactory) SetReplicas(replicas int32) *MockDeploymentFactory {
	factory.get().Spec.Replicas = &replicas
	return factory
}

func (factory *MockDeploymentFactory) AddVolume(volume corev1.Volume) *MockDeploymentFactory {
	volumes := &factory.get().Spec.Template.Spec.Volumes
	*volumes = append(*volumes, volume)
	return factory
}

func (factory *MockDeploymentFactory) AddConfigmapVolume(volumeName, configmapName string) *MockDeploymentFactory {
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

func (factory *MockDeploymentFactory) AddContainer(container corev1.Container) *MockDeploymentFactory {
	containers := &factory.get().Spec.Template.Spec.Containers
	*containers = append(*containers, container)
	return factory
}
