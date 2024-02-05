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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ResourceConstraintTplType string

const (
	GeneralResourceConstraint         ResourceConstraintTplType = "general"
	MemoryOptimizedResourceConstraint ResourceConstraintTplType = "memory-optimized"
	ProductionResourceConstraint      ResourceConstraintTplType = "production"

	generalResourceConstraintTemplate = `
- name: c1
  cpu:
    min: 0.5
    max: 2
    step: 0.5
  memory:
    sizePerCPU: 1Gi
- name: c2
  cpu:
    min: 2
    max: 2
  memory:
    sizePerCPU: 2Gi
- name: c3
  cpu:
    slots: [1, 2, 4, 8, 16, 24, 32, 48, 64, 96, 128]
  memory:
    sizePerCPU: 4Gi
- name: c4
  cpu:
    slots: [100, 500]
  memory:
    sizePerCPU: 2Gi
`

	memoryResourceConstraintTemplate = `
- name: c1
  cpu:
    slots: [2, 4, 8, 12, 24, 48]
  memory:
    sizePerCPU: 8Gi
- name: c2
  cpu:
    min: 2
    max: 128
    step: 2
  memory:
    sizePerCPU: 16Gi
`

	productionResourceConstraintTemplate = `
- name: c1
  cpu:
    min: "0.5"
    max: 64
    step: "0.5"
  memory:
    minPerCPU: 1Gi
    maxPerCPU: 32Gi
  storage:
    min: 1Gi
    max: 10Ti
`
)

type MockComponentResourceConstraintFactory struct {
	BaseFactory[appsv1alpha1.ComponentResourceConstraint, *appsv1alpha1.ComponentResourceConstraint, MockComponentResourceConstraintFactory]
}

func NewComponentResourceConstraintFactory(name string) *MockComponentResourceConstraintFactory {
	f := &MockComponentResourceConstraintFactory{}
	f.Init("", name, &appsv1alpha1.ComponentResourceConstraint{
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
		newConstraints []appsv1alpha1.ResourceConstraintRule
		constraints    = factory.Get().Spec.Rules
	)
	switch constraintTplType {
	case GeneralResourceConstraint:
		tpl = generalResourceConstraintTemplate
	case MemoryOptimizedResourceConstraint:
		tpl = memoryResourceConstraintTemplate
	case ProductionResourceConstraint:
		tpl = productionResourceConstraintTemplate
	}
	if err := yaml.Unmarshal([]byte(tpl), &newConstraints); err != nil {
		panic(err)
	}
	constraints = append(constraints, newConstraints...)
	factory.Get().Spec.Rules = constraints
	return factory
}

// AddSelector add a cluster resource constraint selector
// TODO(xingran): it will be deprecated in the future, use AddComponentSelector instead
func (factory *MockComponentResourceConstraintFactory) AddSelector(selector appsv1alpha1.ClusterResourceConstraintSelector) *MockComponentResourceConstraintFactory {
	factory.Get().Spec.Selector = append(factory.Get().Spec.Selector, selector)
	return factory
}

func (factory *MockComponentResourceConstraintFactory) AddComponentSelector(compSelector appsv1alpha1.ComponentResourceConstraintSelector) *MockComponentResourceConstraintFactory {
	factory.Get().Spec.ComponentSelector = append(factory.Get().Spec.ComponentSelector, compSelector)
	return factory
}
