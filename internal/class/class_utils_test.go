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

package class

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("utils", func() {
	buildResource := func(cpu string, memory string) corev1.ResourceList {
		result := make(corev1.ResourceList)
		if cpu != "" {
			result[corev1.ResourceCPU] = resource.MustParse(cpu)
		}
		if memory != "" {
			result[corev1.ResourceMemory] = resource.MustParse(memory)
		}
		return result
	}

	buildClass := func(name string, cpu string, mem string) v1alpha1.ComponentClass {
		return v1alpha1.ComponentClass{
			Name:   name,
			CPU:    resource.MustParse(cpu),
			Memory: resource.MustParse(mem),
		}
	}

	Context("validate component class", func() {
		var (
			kbClassDefinitionObjName     = "kb"
			customClassDefinitionObjName = "custom"
			clusterDefinitionName        = "apecloud-mysql"
			compType1                    = "component-have-class-definition"
			compType2                    = "component-does-not-have-class-definition"
			clsMgr                       *Manager
		)

		BeforeEach(func() {
			var err error
			kbClassFactory := testapps.NewComponentClassDefinitionFactory(kbClassDefinitionObjName, clusterDefinitionName, compType1)
			kbClassFactory.AddClasses(testapps.DefaultResourceConstraintName, []v1alpha1.ComponentClass{
				buildClass("general-1c1g", "1", "1Gi"),
				buildClass("general-1c4g", "1", "4Gi"),
				buildClass("general-2c4g", "2", "4Gi"),
				buildClass("general-2c8g", "2", "8Gi"),
				buildClass("large", "500", "1000Gi"),
			})

			customClassFactory := testapps.NewComponentClassDefinitionFactory(customClassDefinitionObjName, clusterDefinitionName, compType1)
			customClassFactory.AddClasses(testapps.DefaultResourceConstraintName, []v1alpha1.ComponentClass{
				buildClass("large", "100", "200Gi"),
			})

			constraint := testapps.NewComponentResourceConstraintFactory(testapps.DefaultResourceConstraintName).
				AddConstraints(testapps.GeneralResourceConstraint).
				GetObject()

			clsMgr, err = NewManager(v1alpha1.ComponentClassDefinitionList{
				Items: []v1alpha1.ComponentClassDefinition{
					*kbClassFactory.GetObject(),
					*customClassFactory.GetObject(),
				},
			}, v1alpha1.ComponentResourceConstraintList{Items: []v1alpha1.ComponentResourceConstraint{*constraint}})

			Expect(err).ShouldNot(HaveOccurred())
		})

		When("component have class definition", func() {
			It("should succeed with valid classDefRef", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
					ClassDefRef: &v1alpha1.ClassDefRef{
						Name:  kbClassDefinitionObjName,
						Class: testapps.Class1c1g.Name,
					},
				}
				cls, err := clsMgr.ChooseClass(comp)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cls.CPU.Equal(testapps.Class1c1g.CPU)).Should(BeTrue())
				Expect(cls.Memory.Equal(testapps.Class1c1g.Memory)).Should(BeTrue())
			})

			It("should match minial class with partial classDefRef", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
					ClassDefRef: &v1alpha1.ClassDefRef{
						Class: "large",
					},
				}
				cls, err := clsMgr.ChooseClass(comp)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cls.CPU.String()).Should(Equal("100"))
				Expect(cls.Memory.String()).Should(Equal("200Gi"))
			})

			It("should fail with invalid classDefRef", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
					ClassDefRef:     &v1alpha1.ClassDefRef{Class: "class-not-exists"},
				}
				_, err := clsMgr.ChooseClass(comp)
				Expect(err).Should(HaveOccurred())
			})

			It("should succeed with valid resource", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
					Resources: corev1.ResourceRequirements{
						Requests: buildResource("1", "1Gi"),
					},
				}
				cls, err := clsMgr.ChooseClass(comp)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cls.CPU.Equal(testapps.Class1c1g.CPU)).Should(BeTrue())
				Expect(cls.Memory.Equal(testapps.Class1c1g.Memory)).Should(BeTrue())
			})

			It("should fail with invalid cpu resource", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
					Resources: corev1.ResourceRequirements{
						Requests: buildResource("100", "2Gi"),
					},
				}
				_, err := clsMgr.ChooseClass(comp)
				Expect(err).Should(HaveOccurred())
			})

			It("should fail with invalid memory resource", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
					Resources: corev1.ResourceRequirements{
						Requests: buildResource("1", "200Gi"),
					},
				}
				_, err := clsMgr.ChooseClass(comp)
				Expect(err).Should(HaveOccurred())
			})

			It("should match minial class with empty resource", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
				}
				cls, err := clsMgr.ChooseClass(comp)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cls.CPU.String()).Should(Equal("1"))
				Expect(cls.Memory.String()).Should(Equal("1Gi"))
			})

			It("should match minial memory if with only cpu", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
					Resources: corev1.ResourceRequirements{
						Requests: buildResource("2", ""),
					},
				}
				cls, err := clsMgr.ChooseClass(comp)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cls.CPU.String()).Should(Equal("2"))
				Expect(cls.Memory.String()).Should(Equal("4Gi"))
			})

			It("should match minial cpu if with only memory", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType1,
					Resources: corev1.ResourceRequirements{
						Requests: buildResource("", "4Gi"),
					},
				}
				cls, err := clsMgr.ChooseClass(comp)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cls.CPU.String()).Should(Equal("1"))
				Expect(cls.Memory.String()).Should(Equal("4Gi"))
			})
		})

		When("component without class definition", func() {
			It("should succeed without classDefRef", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType2,
				}
				cls, err := clsMgr.ChooseClass(comp)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cls).Should(BeNil())
			})

			It("should fail with classDefRef", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType2,
					ClassDefRef:     &v1alpha1.ClassDefRef{Class: testapps.Class1c1gName},
				}
				_, err := clsMgr.ChooseClass(comp)
				Expect(err).Should(HaveOccurred())
			})

			It("should succeed without classDefRef", func() {
				comp := &v1alpha1.ClusterComponentSpec{
					ComponentDefRef: compType2,
					Resources: corev1.ResourceRequirements{
						Requests: buildResource("100", "200Gi"),
					},
				}
				cls, err := clsMgr.ChooseClass(comp)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(cls).Should(BeNil())
			})
		})
	})

	It("get classes should succeed", func() {
		var (
			err                    error
			classDefinitionObjName = "custom"
			specClassDefRef        = v1alpha1.ClassDefRef{Name: classDefinitionObjName, Class: testapps.Class1c1gName}
			statusClassDefRef      = v1alpha1.ClassDefRef{Name: classDefinitionObjName, Class: "general-100c100g"}
			clsMgr                 *Manager
			compType               = "mysql"
		)

		classDef := testapps.NewComponentClassDefinitionFactory(classDefinitionObjName, "apecloud-mysql", compType).
			AddClasses(testapps.DefaultResourceConstraintName, []v1alpha1.ComponentClass{testapps.Class1c1g}).
			GetObject()

		By("class definition status is out of date")
		classDef.SetGeneration(1)
		classDef.Status.ObservedGeneration = 0
		classDef.Status.Classes = []v1alpha1.ComponentClassInstance{
			{
				ComponentClass: v1alpha1.ComponentClass{
					Name:   statusClassDefRef.Class,
					CPU:    resource.MustParse("100"),
					Memory: resource.MustParse("100Gi"),
				},
				ResourceConstraintRef: "",
			},
		}
		clsMgr, err = NewManager(v1alpha1.ComponentClassDefinitionList{
			Items: []v1alpha1.ComponentClassDefinition{*classDef},
		}, v1alpha1.ComponentResourceConstraintList{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(clsMgr.HasClass(compType, specClassDefRef)).Should(BeTrue())
		Expect(clsMgr.HasClass(compType, statusClassDefRef)).Should(BeFalse())

		By("class definition status is in sync with the class definition spec")
		classDef.Status.ObservedGeneration = 1
		clsMgr, err = NewManager(v1alpha1.ComponentClassDefinitionList{
			Items: []v1alpha1.ComponentClassDefinition{*classDef},
		}, v1alpha1.ComponentResourceConstraintList{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(clsMgr.HasClass(compType, specClassDefRef)).Should(BeFalse())
		Expect(clsMgr.HasClass(compType, statusClassDefRef)).Should(BeTrue())
	})
})
