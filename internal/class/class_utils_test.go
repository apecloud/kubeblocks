/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package class

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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

	Context("get classes", func() {
		It("should succeed", func() {
			var (
				err             error
				specClassName   = testapps.Class1c1gName
				statusClassName = "general-100c100g"
				compClasses     map[string]map[string]*v1alpha1.ComponentClassInstance
				compType        = "mysql"
			)

			classDef := testapps.NewComponentClassDefinitionFactory("custom", "apecloud-mysql", compType).
				AddClasses(testapps.DefaultResourceConstraintName, []string{specClassName}).
				GetObject()

			By("class definition status is out of date")
			classDef.SetGeneration(1)
			classDef.Status.ObservedGeneration = 0
			classDef.Status.Classes = []v1alpha1.ComponentClassInstance{
				{
					ComponentClass: v1alpha1.ComponentClass{
						Name:   statusClassName,
						CPU:    resource.MustParse("100"),
						Memory: resource.MustParse("100Gi"),
					},
					ResourceConstraintRef: "",
				},
			}
			compClasses, err = GetClasses(v1alpha1.ComponentClassDefinitionList{
				Items: []v1alpha1.ComponentClassDefinition{*classDef},
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(compClasses[compType][specClassName]).ShouldNot(BeNil())
			Expect(compClasses[compType][statusClassName]).Should(BeNil())

			By("class definition status is in sync with the class definition spec")
			classDef.Status.ObservedGeneration = 1
			compClasses, err = GetClasses(v1alpha1.ComponentClassDefinitionList{
				Items: []v1alpha1.ComponentClassDefinition{*classDef},
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(compClasses[compType][specClassName]).Should(BeNil())
			Expect(compClasses[compType][statusClassName]).ShouldNot(BeNil())
		})
	})
})
