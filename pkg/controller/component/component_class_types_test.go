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

package component

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

const resourceConstraintBytes = `
# API scope: cluster
apiVersion: "apps.kubeblocks.io/v1alpha1"
kind:       "ComponentResourceConstraint"
metadata:
  name: kb-resource-constraint-general
spec:
  rules:
  - cpu:
      min: 0.5
      max: 128
      step: 0.5
    memory:
      sizePerCPU: 4Gi
  - cpu:
      slots: [0.1, 0.2, 0.4, 0.6, 0.8, 1]
    memory:
      minPerCPU: 200Mi
  - cpu:
      min: 0.1
      max: 64
      step: 0.1
    memory:
      minPerCPU: 4Gi
      maxPerCPU: 8Gi
`

var buildClass = func(cpu string, memory string) *ComponentClassWithRef {
	return &ComponentClassWithRef{
		ComponentClass: appsv1alpha1.ComponentClass{
			CPU:    resource.MustParse(cpu),
			Memory: resource.MustParse(memory),
		},
	}
}

var buildResourceList = func(cpu string, memory string) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse(memory),
	}
}

func TestComponentClass(t *testing.T) {
	classes := []*ComponentClassWithRef{
		buildClass("1", "2Gi"),
		buildClass("1", "1Gi"),
		buildClass("2", "0.5Gi"),
		buildClass("1", "1Gi"),
		buildClass("0.5", "10Gi"),
	}
	sort.Sort(ByClassResource(classes))
	candidate := classes[0]
	if !candidate.CPU.Equal(resource.MustParse("0.5")) || !candidate.Memory.Equal(resource.MustParse("10Gi")) {
		t.Errorf("case failed")
	}
}

func TestComponentResourceConstraint(t *testing.T) {
	var cf appsv1alpha1.ComponentResourceConstraint
	err := yaml.Unmarshal([]byte(resourceConstraintBytes), &cf)
	if err != nil {
		panic("Failed to unmarshal resource constraint: %v" + err.Error())
	}
	var rules []appsv1alpha1.ResourceConstraintRule
	rules = append(rules, cf.Spec.Rules...)
	sort.Sort(ByRuleList(rules))
	resources := rules[0].GetMinimalResources()
	assert.Equal(t, resources.Cpu().Cmp(resource.MustParse("0.1")) == 0, true)
	assert.Equal(t, resources.Memory().Cmp(resource.MustParse("20Mi")) == 0, true)
}

func TestResourceList(t *testing.T) {
	rl := []corev1.ResourceList{
		buildResourceList("1", "2Gi"),
		buildResourceList("1", "1Gi"),
		buildResourceList("2", "0.5Gi"),
		buildResourceList("1", "1Gi"),
		buildResourceList("0.5", "10Gi"),
	}
	sort.Sort(ByResourceList(rl))

	candidate := rl[0]
	if !candidate.Cpu().Equal(resource.MustParse("0.5")) || !candidate.Memory().Equal(resource.MustParse("10Gi")) {
		t.Errorf("case failed")
	}
}
