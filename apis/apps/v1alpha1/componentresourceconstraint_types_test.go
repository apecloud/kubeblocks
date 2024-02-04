/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const resourceConstraints = `
# API scope: cluster
apiVersion: "apps.kubeblocks.io/v1alpha1"
kind:       "ComponentResourceConstraint"
metadata:
  name: kb-resource-constraint-general
spec:
  rules:
  - name: c1
    cpu:
      min: 0.5
      max: 128
      step: 0.5
    memory:
      sizePerCPU: 4Gi
    storage:
      min: 20Gi
  - name: c2
    cpu:
      slots: [0.1, 0.2, 0.4, 0.6, 0.8, 1]
    memory:
      minPerCPU: 200Mi
      maxPerCPU: 400Mi
    storage:
      min: 20Gi
      max: 100Ti
  - name: c3
    cpu:
      min: 0.1
      max: 64
      step: 0.1
    memory:
      minPerCPU: 4Gi
      maxPerCPU: 8Gi
    storage:
      max: 100Ti
  selector:
  - clusterDefRef: apecloud-mysql
    components:
    - componentDefRef: mysql
      rules:
      - "c1"
      - "c2"
      - "c3"
`
const (
	clusterDefRef   = "apecloud-mysql"
	componentDefRef = "mysql"
)

var (
	cf ComponentResourceConstraint
)

func init() {
	if err := yaml.Unmarshal([]byte(resourceConstraints), &cf); err != nil {
		panic("Failed to unmarshal resource constraints: %v" + err.Error())
	}
}

func TestGetMinimalResources(t *testing.T) {
	var resources corev1.ResourceList
	resources = cf.Spec.Rules[0].GetMinimalResources()
	resources.Cpu().Equal(resource.MustParse("0.5"))
	resources.Memory().Equal(resource.MustParse("2Gi"))

	resources = cf.Spec.Rules[1].GetMinimalResources()
	resources.Cpu().Equal(resource.MustParse("0.1"))
	resources.Memory().Equal(resource.MustParse("20Mi"))

	resources = cf.Spec.Rules[2].GetMinimalResources()
	resources.Cpu().Equal(resource.MustParse("0.1"))
	resources.Memory().Equal(resource.MustParse("0.4Gi"))
}

func TestCompleteResources(t *testing.T) {
	resources := corev1.ResourceList{
		corev1.ResourceCPU: resource.MustParse("1"),
	}
	cf.Spec.Rules[0].CompleteResources(resources)
	resources.Memory().Equal(resource.MustParse("4Gi"))

	cf.Spec.Rules[1].CompleteResources(resources)
	resources.Memory().Equal(resource.MustParse("200Mi"))

	cf.Spec.Rules[2].CompleteResources(resources)
	resources.Memory().Equal(resource.MustParse("4Gi"))
}

func TestResourceConstraints(t *testing.T) {
	cases := []struct {
		desc    string
		cpu     string
		memory  string
		expect  bool
		storage string
	}{
		{
			desc:   "test memory constraint with sizePerCPU",
			cpu:    "0.5",
			memory: "2Gi",
			expect: true,
		},
		{
			desc:   "test memory constraint with unit Mi",
			cpu:    "0.2",
			memory: "40Mi",
			expect: true,
		},
		{
			desc:   "test memory constraint with minPerCPU and maxPerCPU",
			cpu:    "1",
			memory: "6Gi",
			expect: true,
		},
		{
			desc:   "test cpu with decimal",
			cpu:    "0.3",
			memory: "1.2Gi",
			expect: true,
		},
		{
			desc:   "test CPU with invalid step",
			cpu:    "100.6",
			memory: "402.4Gi",
			expect: false,
		},
		{
			desc:   "test CPU with invalid step",
			cpu:    "1.05",
			memory: "4.2Gi",
			expect: false,
		},
		{
			desc:   "test invalid memory",
			cpu:    "2",
			memory: "20Gi",
			expect: false,
		},
		{
			desc:   "test with only memory",
			memory: "200Gi",
			expect: true,
		},
		{
			desc:   "test invalid memory",
			cpu:    "2",
			memory: "6Gi",
			expect: false,
		},
		{
			desc:    "test invalid storage",
			cpu:     "1",
			memory:  "200Mi",
			expect:  false,
			storage: "10Gi",
		},
		{
			desc:    "test invalid storage",
			cpu:     "1",
			memory:  "200Mi",
			expect:  false,
			storage: "200Ti",
		},
	}

	for _, item := range cases {
		requests := corev1.ResourceList{}
		if item.cpu != "" {
			requests[corev1.ResourceCPU] = resource.MustParse(item.cpu)
		}
		if item.memory != "" {
			requests[corev1.ResourceMemory] = resource.MustParse(item.memory)
		}
		if item.storage != "" {
			requests[corev1.ResourceStorage] = resource.MustParse(item.storage)
		}

		constraints := cf.FindMatchingRules(clusterDefRef, componentDefRef, requests)
		assert.Equal(t, item.expect, len(constraints) > 0)

		// if storage is empty, we should also validate function MatchClass which only consider cpu and memory
		if item.storage == "" {
			class := &ComponentClass{
				CPU:    *requests.Cpu(),
				Memory: *requests.Memory(),
			}
			assert.Equal(t, item.expect, cf.MatchClass(clusterDefRef, componentDefRef, class))
		}
	}
}
