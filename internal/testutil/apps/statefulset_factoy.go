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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/apecloud/kubeblocks/internal/constant"
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
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: componentName,
					constant.AppManagedByLabelKey:   constant.AppName,
				},
			},
			Spec: appsv1.StatefulSetSpec{
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
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
					Type: appsv1.OnDeleteStatefulSetStrategyType,
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
