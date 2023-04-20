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
  constraints:
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

var (
	cf ComponentResourceConstraint
)

func init() {
	if err := yaml.Unmarshal([]byte(resourceConstraints), &cf); err != nil {
		panic("Failed to unmarshal resource constraints: %v" + err.Error())
	}
}

func TestResourceConstraints(t *testing.T) {
	cases := []struct {
		desc   string
		cpu    string
		memory string
		expect bool
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
			desc:   "test invalid memory",
			cpu:    "2",
			memory: "6Gi",
			expect: false,
		},
	}

	for _, item := range cases {
		var (
			cpu    = resource.MustParse(item.cpu)
			memory = resource.MustParse(item.memory)
		)
		requirements := &corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    cpu,
				corev1.ResourceMemory: memory,
			},
		}
		assert.Equal(t, item.expect, len(cf.FindMatchingConstraints(requirements)) > 0)

		class := &ComponentClassInstance{
			ComponentClass: ComponentClass{
				CPU:    cpu,
				Memory: memory,
			},
		}
		assert.Equal(t, item.expect, cf.MatchClass(class))
	}
}
