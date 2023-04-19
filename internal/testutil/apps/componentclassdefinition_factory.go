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

func (factory *MockComponentClassDefinitionFactory) AddClassGroup(constraintRef string) *MockComponentClassDefinitionFactory {
	groups := factory.get().Spec.Groups
	group := classGroupTemplate
	group.ResourceConstraintRef = constraintRef
	groups = append(groups, group)
	factory.get().Spec.Groups = groups
	return factory
}

func (factory *MockComponentClassDefinitionFactory) AddClasses(classes []appsv1alpha1.ComponentClass) *MockComponentClassDefinitionFactory {
	groups := factory.get().Spec.Groups
	if len(groups) > 0 {
		groups[len(groups)-1].Series[0].Classes = classes
	}
	factory.get().Spec.Groups = groups
	return factory
}
