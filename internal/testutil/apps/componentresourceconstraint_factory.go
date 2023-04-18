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
	"k8s.io/apimachinery/pkg/util/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ResourceConstraintTplType string

const (
	ResourceConstraintSpecial ResourceConstraintTplType = "special"
	ResourceConstraintNormal  ResourceConstraintTplType = "normal"

	specialConstraintTemplate = `
- cpu:
    min: 0.5
    max: 2
    step: 0.5
  memory:
    sizePerCPU: 1Gi
- cpu:
    min: 2
    max: 2
  memory:
    sizePerCPU: 2Gi
`
	normalConstraintTemplate = `
- cpu:
    slots: [2, 4, 8, 16, 24, 32, 48, 64, 96, 128]
  memory:
    sizePerCPU: 4Gi
`
)

type MockComponentResourceConstraintFactory struct {
	BaseFactory[appsv1alpha1.ComponentResourceConstraint, *appsv1alpha1.ComponentResourceConstraint, MockComponentResourceConstraintFactory]
}

func NewComponentResourceConstraintFactory(name string) *MockComponentResourceConstraintFactory {
	f := &MockComponentResourceConstraintFactory{}
	f.init("", name, &appsv1alpha1.ComponentResourceConstraint{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"resourceconstraint.kubeblocks.io/provider": "kubeblocks",
			},
		},
	}, f)
	return f
}

func (factory *MockComponentResourceConstraintFactory) AddConstraints(constraintTplType ResourceConstraintTplType) *MockComponentResourceConstraintFactory {
	var (
		tpl            string
		newConstraints []appsv1alpha1.ResourceConstraint
		constraints    = factory.get().Spec.Constraints
	)
	switch constraintTplType {
	case ResourceConstraintSpecial:
		tpl = specialConstraintTemplate
	case ResourceConstraintNormal:
		tpl = normalConstraintTemplate
	}
	if err := yaml.Unmarshal([]byte(tpl), &newConstraints); err != nil {
		panic(err)
	}
	constraints = append(constraints, newConstraints...)
	factory.get().Spec.Constraints = constraints
	return factory
}
