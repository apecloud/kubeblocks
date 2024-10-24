/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var (
	defaultSidecarContainer = corev1.Container{
		Name:    "sidecar",
		Command: []string{"/scripts/setup.sh"},
	}
)

type MockSidecarDefinitionFactory struct {
	BaseFactory[appsv1.SidecarDefinition, *appsv1.SidecarDefinition, MockSidecarDefinitionFactory]
}

func NewSidecarDefinitionFactory(name, owner string, selectors []string) *MockSidecarDefinitionFactory {
	f := &MockSidecarDefinitionFactory{}
	f.Init("", name,
		&appsv1.SidecarDefinition{
			Spec: appsv1.SidecarDefinitionSpec{
				Owner:     owner,
				Selectors: selectors,
			},
		}, f)
	return f
}

func (f *MockSidecarDefinitionFactory) AddContainer(c *corev1.Container) *MockSidecarDefinitionFactory {
	if c == nil {
		c = &defaultSidecarContainer
	}

	if f.Get().Spec.Containers == nil {
		f.Get().Spec.Containers = []corev1.Container{}
	}
	f.Get().Spec.Containers = append(f.Get().Spec.Containers, *c)
	return f
}
