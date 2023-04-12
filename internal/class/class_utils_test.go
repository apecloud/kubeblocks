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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var _ = Describe("utils", func() {
	var (
		cpuMin  = 1
		cpuMax  = 64
		scales  = []int{4, 8, 16}
		classes map[string]*v1alpha1.ComponentClassInstance
	)

	genComponentClasses := func(cpuMin int, cpuMax int, scales []int) map[string]*v1alpha1.ComponentClassInstance {
		results := make(map[string]*v1alpha1.ComponentClassInstance)
		for cpu := cpuMin; cpu <= cpuMax; cpu++ {
			for _, scale := range scales {
				var (
					clsName = fmt.Sprintf("cpu-%d-scale-%d", cpu, scale)
				)
				results[clsName] = &v1alpha1.ComponentClassInstance{
					ComponentClass: v1alpha1.ComponentClass{
						Name:   clsName,
						CPU:    resource.MustParse(fmt.Sprintf("%d", cpu)),
						Memory: resource.MustParse(fmt.Sprintf("%dGi", cpu*scale)),
					},
				}
			}
		}
		return results
	}

	BeforeEach(func() {
		classes = genComponentClasses(cpuMin, cpuMax, scales)
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	buildFilters := func(cpu string, memory string) map[corev1.ResourceName]resource.Quantity {
		result := make(map[corev1.ResourceName]resource.Quantity)
		if cpu != "" {
			result[corev1.ResourceCPU] = resource.MustParse(cpu)
		}
		if memory != "" {
			result[corev1.ResourceMemory] = resource.MustParse(memory)
		}
		return result
	}

	Context("sort component classes", func() {
		It("should match one class by cpu and memory", func() {
			class := ChooseComponentClasses(classes, buildFilters("1", "4Gi"))
			Expect(class.CPU.String()).Should(Equal("1"))
			Expect(class.Memory.String()).Should(Equal("4Gi"))
		})

		It("match multiple classes by cpu", func() {
			class := ChooseComponentClasses(classes, buildFilters("1", ""))
			Expect(class.CPU.String()).Should(Equal("1"))
			Expect(class.Memory.String()).Should(Equal("4Gi"))
		})

		It("match multiple classes by memory", func() {
			class := ChooseComponentClasses(classes, buildFilters("", "16Gi"))
			Expect(class.CPU.String()).Should(Equal("1"))
			Expect(class.Memory.String()).Should(Equal("16Gi"))
		})

		It("not match any classes by cpu", func() {
			class := ChooseComponentClasses(classes, buildFilters(fmt.Sprintf("%d", cpuMax+1), ""))
			Expect(class).Should(BeNil())
		})

		It("not match any classes by memory", func() {
			class := ChooseComponentClasses(classes, buildFilters("", "1Pi"))
			Expect(class).Should(BeNil())
		})
	})
})
