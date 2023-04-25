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
	"k8s.io/apimachinery/pkg/util/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ResourceConstraintTplType string

const (
	GeneralResourceConstraint         ResourceConstraintTplType = "general"
	MemoryOptimizedResourceConstraint ResourceConstraintTplType = "memory-optimized"
	ProductionResourceConstraint      ResourceConstraintTplType = "production"

	generalResourceConstraintTemplate = `
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
- cpu:
    slots: [2, 4, 8, 16, 24, 32, 48, 64, 96, 128]
  memory:
    sizePerCPU: 4Gi
`

	memoryResourceConstraintTemplate = `
- cpu:
    slots: [2, 4, 8, 12, 24, 48]
  memory:
    sizePerCPU: 8Gi
- cpu:
    min: 2
    max: 128
    step: 2
  memory:
    sizePerCPU: 16Gi
`

	productionResourceConstraintTemplate = `
- cpu:
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
	factory.get().Spec.Constraints = constraints
	return factory
}
