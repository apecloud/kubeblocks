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

package class

import (
	"sort"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

const classFamilyBytes = `
# API scope: cluster
# ClusterClassFamily
apiVersion: "apps.kubeblocks.io/v1alpha1"
kind:       "ClassFamily"
metadata:
  name: kb-class-family-general
spec:
  models:
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

func TestClassFamily_ByClassCPUAndMemory(t *testing.T) {
	buildClass := func(cpu string, memory string) *ComponentClass {
		return &ComponentClass{CPU: resource.MustParse(cpu), Memory: resource.MustParse(memory)}
	}
	classes := []*ComponentClass{
		buildClass("1", "2Gi"),
		buildClass("1", "1Gi"),
		buildClass("2", "0.5Gi"),
		buildClass("1", "1Gi"),
		buildClass("0.5", "10Gi"),
	}
	sort.Sort(ByClassCPUAndMemory(classes))
	candidate := classes[0]
	if candidate.CPU != resource.MustParse("0.5") || candidate.Memory != resource.MustParse("10Gi") {
		t.Errorf("case failed")
	}
}

func TestClassFamily_ModelList(t *testing.T) {
	var cf appsv1alpha1.ClassFamily
	err := yaml.Unmarshal([]byte(classFamilyBytes), &cf)
	if err != nil {
		panic("Failed to unmarshal class family: %v" + err.Error())
	}
	var models []ClassModelWithFamilyName
	for _, model := range cf.Spec.Models {
		models = append(models, ClassModelWithFamilyName{Family: cf.Name, Model: model})
	}
	resource.MustParse("200Mi")
	sort.Sort(ByModelList(models))
	cpu, memory := GetMinCPUAndMemory(models[0].Model)
	assert.Equal(t, cpu.Cmp(resource.MustParse("0.1")) == 0, true)
	assert.Equal(t, memory.Cmp(resource.MustParse("20Mi")) == 0, true)
}
