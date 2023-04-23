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
	"reflect"

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

	buildResourceList := func(cpu string, memory string) corev1.ResourceList {
		result := make(corev1.ResourceList)
		if cpu != "" {
			result[corev1.ResourceCPU] = resource.MustParse(cpu)
		}
		if memory != "" {
			result[corev1.ResourceMemory] = resource.MustParse(memory)
		}
		return result
	}

	Context("validate component class", func() {
		var (
			specClassName = testapps.Class1c1gName
			comp1Name     = "component-have-class-definition"
			comp2Name     = "component-does-not-have-class-definition"
			compClasses   map[string]map[string]*v1alpha1.ComponentClassInstance
		)

		BeforeEach(func() {
			var err error
			classDef := testapps.NewComponentClassDefinitionFactory("custom", "apecloud-mysql", comp1Name).
				AddClasses(testapps.DefaultResourceConstraintName, []string{specClassName}).
				GetObject()
			compClasses, err = GetClasses(v1alpha1.ComponentClassDefinitionList{
				Items: []v1alpha1.ComponentClassDefinition{
					*classDef,
				},
			})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should succeed if component have class definition and with valid classDefRef", func() {
			comp := &v1alpha1.ClusterComponentSpec{
				ComponentDefRef: comp1Name,
				ClassDefRef:     &v1alpha1.ClassDefRef{Class: specClassName},
			}
			cls, err := ValidateComponentClass(comp, compClasses)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(reflect.DeepEqual(cls.ComponentClass, testapps.Class1c1g)).Should(BeTrue())
		})

		It("should fail if component have class definition and with invalid classDefRef", func() {
			comp := &v1alpha1.ClusterComponentSpec{
				ComponentDefRef: comp1Name,
				ClassDefRef:     &v1alpha1.ClassDefRef{Class: "class-not-exists"},
			}
			_, err := ValidateComponentClass(comp, compClasses)
			Expect(err).Should(HaveOccurred())
		})

		It("should succeed if component have class definition and with valid resource", func() {
			comp := &v1alpha1.ClusterComponentSpec{
				ComponentDefRef: comp1Name,
				Resources: corev1.ResourceRequirements{
					Requests: buildResourceList("1", "1Gi"),
				},
			}
			cls, err := ValidateComponentClass(comp, compClasses)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(reflect.DeepEqual(cls.ComponentClass, testapps.Class1c1g)).Should(BeTrue())
		})

		It("should fail if component have class definition and with invalid resource", func() {
			comp := &v1alpha1.ClusterComponentSpec{
				ComponentDefRef: comp1Name,
				Resources: corev1.ResourceRequirements{
					Requests: buildResourceList("100", "200Gi"),
				},
			}
			_, err := ValidateComponentClass(comp, compClasses)
			Expect(err).Should(HaveOccurred())
		})

		It("should succeed if component does not have class definition and without classDefRef", func() {
			comp := &v1alpha1.ClusterComponentSpec{
				ComponentDefRef: comp2Name,
			}
			cls, err := ValidateComponentClass(comp, compClasses)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cls).Should(BeNil())
		})

		It("should fail if component does not have class definition and with classDefRef", func() {
			comp := &v1alpha1.ClusterComponentSpec{
				ComponentDefRef: comp2Name,
				ClassDefRef:     &v1alpha1.ClassDefRef{Class: specClassName},
			}
			_, err := ValidateComponentClass(comp, compClasses)
			Expect(err).Should(HaveOccurred())
		})

		It("should succeed if component does not have class definition and without classDefRef", func() {
			comp := &v1alpha1.ClusterComponentSpec{
				ComponentDefRef: comp2Name,
				Resources: corev1.ResourceRequirements{
					Requests: buildResourceList("100", "200Gi"),
				},
			}
			cls, err := ValidateComponentClass(comp, compClasses)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cls).Should(BeNil())
		})
	})

	Context("sort component classes", func() {
		It("should match minial class if cpu and memory are empty", func() {
			class := ChooseComponentClasses(classes, buildResourceList("", ""))
			Expect(class).ShouldNot(BeNil())
			Expect(class.CPU.String()).Should(Equal("1"))
			Expect(class.Memory.String()).Should(Equal("4Gi"))
		})

		It("should match one class by cpu and memory", func() {
			class := ChooseComponentClasses(classes, buildResourceList("1", "4Gi"))
			Expect(class).ShouldNot(BeNil())
			Expect(class.CPU.String()).Should(Equal("1"))
			Expect(class.Memory.String()).Should(Equal("4Gi"))
		})

		It("match multiple classes by cpu", func() {
			class := ChooseComponentClasses(classes, buildResourceList("1", ""))
			Expect(class).ShouldNot(BeNil())
			Expect(class.CPU.String()).Should(Equal("1"))
			Expect(class.Memory.String()).Should(Equal("4Gi"))
		})

		It("match multiple classes by memory", func() {
			class := ChooseComponentClasses(classes, buildResourceList("", "16Gi"))
			Expect(class).ShouldNot(BeNil())
			Expect(class.CPU.String()).Should(Equal("1"))
			Expect(class.Memory.String()).Should(Equal("16Gi"))
		})

		It("not match any classes by cpu", func() {
			class := ChooseComponentClasses(classes, buildResourceList(fmt.Sprintf("%d", cpuMax+1), ""))
			Expect(class).Should(BeNil())
		})

		It("not match any classes by memory", func() {
			class := ChooseComponentClasses(classes, buildResourceList("", "1Pi"))
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
