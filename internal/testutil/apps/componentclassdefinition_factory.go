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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type MockComponentClassDefinitionFactory struct {
	BaseFactory[appsv1alpha1.ComponentClassDefinition, *appsv1alpha1.ComponentClassDefinition, MockComponentClassDefinitionFactory]
}

func NewComponentClassDefinitionFactory(name, clusterDefinitionRef, componentType string) *MockComponentClassDefinitionFactory {
	f := &MockComponentClassDefinitionFactory{}
	f.init("", name, &appsv1alpha1.ComponentClassDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constant.ClassProviderLabelKey:        "kubeblocks",
				constant.ClusterDefLabelKey:           clusterDefinitionRef,
				constant.KBAppComponentDefRefLabelKey: componentType,
			},
		},
	}, f)
	return f
}

func (factory *MockComponentClassDefinitionFactory) AddClasses(constraintRef string, classes []appsv1alpha1.ComponentClass) *MockComponentClassDefinitionFactory {
	groups := factory.get().Spec.Groups
	groups = append(groups, appsv1alpha1.ComponentClassGroup{
		ResourceConstraintRef: constraintRef,
		Series: []appsv1alpha1.ComponentClassSeries{
			{
				Classes: classes,
			},
		},
	})
	factory.get().Spec.Groups = groups
	return factory
}
