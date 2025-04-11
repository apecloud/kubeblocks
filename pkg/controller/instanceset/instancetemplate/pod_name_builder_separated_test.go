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
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("Separated Name builder tests", func() {
	DescribeTable("generates instance ordinals",
		// expected doesn't need its name prefix
		func(its *workloads.InstanceSet, expected []string, expectError bool) {
			itsExt, err := BuildInstanceSetExt(its, nil)
			Expect(err).NotTo(HaveOccurred())
			builder, err := NewPodNameBuilder(itsExt, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(builder.Validate()).To(Succeed())
			instanceNames, err := builder.GenerateAllInstanceNames()
			expectedFull := make([]string, len(expected))
			for i, name := range expected {
				expectedFull[i] = its.Name + name
			}
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(instanceNames).To(Equal(expectedFull))
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
		}, []string{"-0", "-1", "-t1-0", "-t1-1", "-t2-0"}, false),

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
		}, []string{"-t1-10", "-t1-11", "-t2-0", "-t2-2", "-t2-3"}, false),

		Entry("with offline instances", &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](4),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
					},
				},
				OfflineInstances: []string{"foo-1"},
			},
		}, []string{"-0", "-2", "-t1-0", "-t1-1"}, false),
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
				PodNamingRule: workloads.PodNamingRuleSeparated,
			},
		}

		itsExt, err := BuildInstanceSetExt(its, nil)
		Expect(err).NotTo(HaveOccurred())
		builder, err := NewPodNameBuilder(itsExt, nil)
		Expect(err).NotTo(HaveOccurred())
		names, err := builder.GenerateAllInstanceNames()
		Expect(err).NotTo(HaveOccurred())
		expected := []string{"-0", "-1", "-t1-0", "-t1-1", "-t2-0"}
		expectedFull := make([]string, len(expected))
		for i, name := range expected {
			expectedFull[i] = its.Name + name
		}
		Expect(names).To(Equal(expectedFull))
	})

	Context("buildInstanceName2TemplateMap", func() {
		var its *workloads.InstanceSet
		BeforeEach(func() {
			its = builder.NewInstanceSetBuilder(namespace, name).
				SetReplicas(3).
				SetTemplate(template).
				SetVolumeClaimTemplates(volumeClaimTemplates...).
				SetPodNamingRule(workloads.PodNamingRuleSeparated).
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
			nameOverridePodName := its.Name + "-" + nameOverride + "-0"
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
})
