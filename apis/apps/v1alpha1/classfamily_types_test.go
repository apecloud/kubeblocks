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

func TestClassFamily_ValidateResourceRequirements(t *testing.T) {
	var cf ClassFamily
	err := yaml.Unmarshal([]byte(classFamilyBytes), &cf)
	if err != nil {
		panic("Failed to unmarshal class family: %v" + err.Error())
	}
	cases := []struct {
		cpu    string
		memory string
		expect bool
	}{
		{cpu: "0.5", memory: "2Gi", expect: true},
		{cpu: "0.2", memory: "40Mi", expect: true},
		{cpu: "1", memory: "6Gi", expect: true},
		{cpu: "2", memory: "20Gi", expect: false},
		{cpu: "2", memory: "6Gi", expect: false},
	}

	for _, item := range cases {
		requirements := &corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse(item.cpu),
				corev1.ResourceMemory: resource.MustParse(item.memory),
			},
		}
		assert.Equal(t, item.expect, len(cf.FindMatchingModels(requirements)) > 0)
	}
}
