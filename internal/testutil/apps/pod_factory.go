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
)

type MockPodFactory struct {
	BaseFactory[corev1.Pod, *corev1.Pod, MockPodFactory]
}

func NewPodFactory(namespace, name string) *MockPodFactory {
	f := &MockPodFactory{}
	f.init(namespace, name,
		&corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{},
			},
		}, f)
	return f
}

func (factory *MockPodFactory) AddContainer(container corev1.Container) *MockPodFactory {
	containers := &factory.get().Spec.Containers
	*containers = append(*containers, container)
	return factory
}

func (factory *MockPodFactory) AddVolume(volume corev1.Volume) *MockPodFactory {
	volumes := &factory.get().Spec.Volumes
	if volumes == nil {
		volumes = &[]corev1.Volume{}
	}
	*volumes = append(*volumes, volume)
	return factory
}
