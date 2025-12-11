/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package instancetemplate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("flat name builder tests", func() {
	DescribeTable("generates instance ordinals",
		func(its *workloads.InstanceSet, expected map[string]sets.Set[int32], expectError bool) {
			its.Spec.FlatInstanceOrdinal = true
			itsExt, err := BuildInstanceSetExt(its, nil)
			Expect(err).NotTo(HaveOccurred())
			err = ValidateInstanceTemplates(its, nil)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				ordinals, err := generateTemplateName2OrdinalMap(itsExt)
				Expect(err).Should(Or(BeNil(), Equal(ErrOrdinalsNotEnough)))
				Expect(ordinals).To(Equal(expected))
			}
		},

		Entry("a new instanceset", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1),
			"t1": sets.New[int32](2, 3),
			"t2": sets.New[int32](4),
		}, false),

		Entry("with running instances", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1},
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Ordinals: []int32{2, 4},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1),
			"t1": sets.New[int32](2, 4),
			"t2": sets.New[int32](3),
		}, false),

		Entry("deal with scale in", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](1),
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1},
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Ordinals: []int32{2, 3, 4},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1, 5),
			"t1": sets.New[int32](2),
			"t2": sets.New[int32](6),
		}, false),

		Entry("with ordinal spec", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Discrete: []int32{10, 11},
						},
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](3),
						Ordinals: kbappsv1.Ordinals{
							Ranges: []kbappsv1.Range{
								{
									Start: 2,
									End:   3,
								},
							},
							Discrete: []int32{0},
						},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"t1": sets.New[int32](10, 11),
			"t2": sets.New[int32](0, 2, 3),
		}, false),

		Entry("with ordinal spec - deal with scale out", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](4),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Ranges: []kbappsv1.Range{
								{
									Start: 100,
									End:   199,
								},
							},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1},
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Ordinals: []int32{100},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1),
			"t1": sets.New[int32](100, 101),
		}, false),

		Entry("with ordinal spec - a newly defined ordinal spec takes an ordinal of the default template", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Discrete: []int32{1, 2},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 3, 4),
			"t1": sets.New[int32](1, 2),
		}, false),

		Entry("with ordinal spec - replicas < length of ordinals range", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Ranges: []kbappsv1.Range{
								{
									Start: 100,
									End:   199,
								},
							},
						},
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
						Ordinals: kbappsv1.Ordinals{
							Ranges: []kbappsv1.Range{
								{
									Start: 500,
									End:   599,
								},
							},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Ordinals: []int32{150},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1),
			"t1": sets.New[int32](100, 150),
			"t2": sets.New[int32](500),
		}, false),

		Entry("with ordinal spec - zero replica", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](0),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](0),
						Ordinals: kbappsv1.Ordinals{
							Discrete: []int32{10, 11},
						},
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](0),
						Ordinals: kbappsv1.Ordinals{
							Ranges: []kbappsv1.Range{
								{
									Start: 2,
									End:   3,
								},
							},
							Discrete: []int32{0},
						},
					},
				},
			},
		}, map[string]sets.Set[int32]{"t1": {}, "t2": {}}, false),

		Entry("with ordinal spec - replicas > length of ordinals range", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](6),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Discrete: []int32{10, 11},
						},
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](4),
						Ordinals: kbappsv1.Ordinals{
							Ranges: []kbappsv1.Range{
								{
									Start: 2,
									End:   3,
								},
							},
							Discrete: []int32{0},
						},
					},
				},
			},
		}, nil, true),

		Entry("with ordinal spec replacing a normal one", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](4),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Ranges: []kbappsv1.Range{
								{
									Start: 2,
									End:   3,
								},
							},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2},
			},
		}, map[string]sets.Set[int32]{
			"t1": sets.New[int32](2, 3),
			"":   sets.New[int32](0, 1),
		}, false),

		Entry("partially exchange ordinal spec", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](4),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Discrete: []int32{0, 2},
						},
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Discrete: []int32{1, 3},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Ordinals: []int32{0, 1},
					},
					{
						Name:     "t2",
						Ordinals: []int32{2, 3},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"t1": sets.New[int32](0),
			"t2": sets.New[int32](3),
		}, false),

		Entry("with offline instances", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
					},
				},
				OfflineInstances: []string{"-2"},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2},
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Ordinals: []int32{3},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1, 4),
			"t1": sets.New[int32](3, 5),
		}, false),

		Entry("with offline instances conflicts template ordinals", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](3),
				DefaultTemplateOrdinals: kbappsv1.Ordinals{
					Discrete: []int32{2},
				},
				OfflineInstances: []string{"-1"},
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
						Ordinals: kbappsv1.Ordinals{
							Discrete: []int32{0, 1},
						},
					},
				},
			},
		}, nil, true),

		Entry("move replicas into instance templates", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](1),
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2, 3, 4},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1, 2),
			"t1": sets.New[int32](5),
			"t2": sets.New[int32](6),
		}, false),

		Entry("move replicas into instance templates - take over", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](1),
						Ordinals: workloads.Ordinals{
							Discrete: []int32{3},
						},
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
						Ordinals: workloads.Ordinals{
							Discrete: []int32{4},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2, 3, 4},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1, 2),
			"t1": sets.New[int32](3),
			"t2": sets.New[int32](4),
		}, false),

		Entry("move replicas into instance templates - take over last one", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](1),
						Ordinals: workloads.Ordinals{
							Discrete: []int32{0},
						},
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
						Ordinals: workloads.Ordinals{
							Discrete: []int32{1},
						},
					},
					{
						Name:     "t3",
						Replicas: ptr.To[int32](1),
						Ordinals: workloads.Ordinals{
							Discrete: []int32{2},
						},
					},
					{
						Name:     "t4",
						Replicas: ptr.To[int32](1),
						Ordinals: workloads.Ordinals{
							Discrete: []int32{3},
						},
					},
					{
						Name:     "t5",
						Replicas: ptr.To[int32](1),
						Ordinals: workloads.Ordinals{
							Discrete: []int32{4},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2, 3, 4},
			},
		}, map[string]sets.Set[int32]{
			"t1": sets.New[int32](0),
			"t2": sets.New[int32](1),
			"t3": sets.New[int32](2),
			"t4": sets.New[int32](3),
			"t5": sets.New[int32](4),
		}, false),

		Entry("move replicas into instance templates - take over and replace", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](1),
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
						Ordinals: workloads.Ordinals{
							Discrete: []int32{4},
						},
					},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2, 3, 4},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1, 2),
			"t1": sets.New[int32](5),
			"t2": sets.New[int32](4),
		}, false),

		Entry("move replicas out of instance templates", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2},
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Replicas: 1,
						Ordinals: []int32{3},
					},
					{
						Name:     "t2",
						Replicas: 1,
						Ordinals: []int32{4},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"": sets.New[int32](0, 1, 2, 5, 6), // replace 3 and 4 with 5 and 6
		}, false),

		Entry("move replicas out of instance templates - take back", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				DefaultTemplateOrdinals: workloads.Ordinals{
					Discrete: []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2},
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Replicas: 1,
						Ordinals: []int32{3},
					},
					{
						Name:     "t2",
						Replicas: 1,
						Ordinals: []int32{4},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"": sets.New[int32](0, 1, 2, 3, 4), // take back 3 and 4
		}, false),

		Entry("move replicas out of instance templates - take back and replace", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				DefaultTemplateOrdinals: workloads.Ordinals{
					Discrete: []int32{0, 1, 2, 3, 5, 6, 7, 8, 9}, // without ordinal 4
				},
			},
			Status: workloads.InstanceSetStatus{
				Ordinals: []int32{0, 1, 2},
				TemplatesStatus: []workloads.InstanceTemplateStatus{
					{
						Name:     "t1",
						Replicas: 1,
						Ordinals: []int32{3},
					},
					{
						Name:     "t2",
						Replicas: 1,
						Ordinals: []int32{4},
					},
				},
			},
		}, map[string]sets.Set[int32]{
			"": sets.New[int32](0, 1, 2, 3, 5), // take back 3 and replace 4 with 5
		}, false),
	)

	It("generates instance names", func() {
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](5),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
					},
					{
						Name:     "t2",
						Replicas: ptr.To[int32](1),
					},
				},
				FlatInstanceOrdinal: true,
			},
		}

		itsExt, err := BuildInstanceSetExt(its, nil)
		Expect(err).NotTo(HaveOccurred())
		builder, err := NewPodNameBuilder(itsExt, nil)
		Expect(err).NotTo(HaveOccurred())
		names, err := builder.GenerateAllInstanceNames()
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(Equal([]string{"foo-0", "foo-1", "foo-2", "foo-3", "foo-4"}))
	})

	Context("buildInstanceName2TemplateMap", func() {
		var its *workloads.InstanceSet
		BeforeEach(func() {
			its = builder.NewInstanceSetBuilder(namespace, name).
				SetReplicas(3).
				SetTemplate(template).
				SetVolumeClaimTemplates(volumeClaimTemplates...).
				SetFlatInstanceOrdinal(true).
				GetObject()
		})

		It("build an its with default template only", func() {
			itsExt, err := BuildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			builder, err := NewPodNameBuilder(itsExt, nil)
			Expect(err).NotTo(HaveOccurred())
			nameTemplate, err := builder.BuildInstanceName2TemplateMap()
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name0 := its.Name + "-0"
			Expect(nameTemplate).Should(HaveKey(name0))
			Expect(nameTemplate).Should(HaveKey(its.Name + "-1"))
			Expect(nameTemplate).Should(HaveKey(its.Name + "-2"))
			nameTemplate[name0].PodTemplateSpec.Spec.Volumes = nil
			defaultTemplate := its.Spec.Template.DeepCopy()
			Expect(nameTemplate[name0].PodTemplateSpec.Spec).Should(Equal(defaultTemplate.Spec))
		})

		It("build an its with one instance template override", func() {
			nameOverride := "name-override"
			annotationOverride := map[string]string{
				"foo": "bar",
			}
			labelOverride := map[string]string{
				"foo": "bar",
			}
			resources := corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("600m"),
				},
			}
			instance := workloads.InstanceTemplate{
				Name:        nameOverride,
				Annotations: annotationOverride,
				Labels:      labelOverride,
				Resources:   &resources,
			}
			its.Spec.Instances = append(its.Spec.Instances, instance)
			itsExt, err := BuildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			builder, err := NewPodNameBuilder(itsExt, nil)
			Expect(err).NotTo(HaveOccurred())
			nameTemplate, err := builder.BuildInstanceName2TemplateMap()
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name0 := its.Name + "-0"
			name1 := its.Name + "-1"
			nameOverridePodName := its.Name + "-2"
			Expect(nameTemplate).Should(HaveKey(name0))
			Expect(nameTemplate).Should(HaveKey(name1))
			Expect(nameTemplate).Should(HaveKey(nameOverridePodName))
			expectedTemplate := its.Spec.Template.DeepCopy()
			Expect(nameTemplate[name0].PodTemplateSpec.Spec).Should(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[name1].PodTemplateSpec.Spec).Should(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[nameOverridePodName].PodTemplateSpec.Spec).ShouldNot(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[nameOverridePodName].PodTemplateSpec.Annotations).Should(Equal(annotationOverride))
			Expect(nameTemplate[nameOverridePodName].PodTemplateSpec.Labels).Should(Equal(labelOverride))
			Expect(nameTemplate[nameOverridePodName].PodTemplateSpec.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU]).Should(Equal(resources.Limits[corev1.ResourceCPU]))
			Expect(nameTemplate[nameOverridePodName].PodTemplateSpec.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]).Should(Equal(its.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]))
		})
	})

	Describe("validateOrdinals", func() {
		It("should validate ordinals successfully", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
					Instances: []workloads.InstanceTemplate{
						{
							Name:     "template1",
							Replicas: ptr.To[int32](3),
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{0, 1, 2},
							},
						},
					},
					FlatInstanceOrdinal: true,
				},
			}
			err := ValidateInstanceTemplates(its, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail validation for negative ordinals", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
					Instances: []workloads.InstanceTemplate{
						{
							Name:     "template1",
							Replicas: ptr.To[int32](3),
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{-1, 0, 1},
							},
						},
					},
					FlatInstanceOrdinal: true,
				},
			}
			err := ValidateInstanceTemplates(its, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ordinal(-1) must >= 0"))
		})

		It("should fail validation for duplicate ordinals", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
					DefaultTemplateOrdinals: kbappsv1.Ordinals{
						Discrete: []int32{1},
					},
					Instances: []workloads.InstanceTemplate{
						{
							Name:     "template1",
							Replicas: ptr.To[int32](2),
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{0, 1},
							},
						},
					},
					FlatInstanceOrdinal: true,
				},
			}
			err := ValidateInstanceTemplates(its, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate ordinal(1)"))
		})

		It("has not enough available ordinals", func() {
			its := &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
					Instances: []workloads.InstanceTemplate{
						{
							Name:     "template1",
							Replicas: ptr.To[int32](3),
							Ordinals: kbappsv1.Ordinals{
								Discrete: []int32{0, 1, 2},
							},
						},
					},
					FlatInstanceOrdinal: true,
					OfflineInstances:    []string{"pod-0"},
				},
			}
			err := ValidateInstanceTemplates(its, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("available ordinals less than replicas"))
		})
	})
})
