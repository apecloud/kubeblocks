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
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("Template tests", func() {
	DescribeTable("generates instance ordinals",
		func(its *workloads.InstanceSet, expected map[string]sets.Set[int32], expectError bool) {
			Expect(validateOrdinals(its.Spec.Instances)).To(Succeed())
			tree := kubebuilderx.NewObjectTree()
			itsExt, err := BuildInstanceSetExt(its, tree)
			Expect(err).NotTo(HaveOccurred())
			ordinals, err := GenerateTemplateName2OrdinalMap(itsExt)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
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
				CurrentInstances: workloads.CurrentInstances{
					"":   []int32{0, 1},
					"t1": []int32{2, 4},
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
				CurrentInstances: workloads.CurrentInstances{
					"":   []int32{0, 1},
					"t1": []int32{2, 3, 4},
				},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1, 3),
			"t1": sets.New[int32](2),
			"t2": sets.New[int32](4),
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
				CurrentInstances: workloads.CurrentInstances{
					"": []int32{0, 1, 2},
				},
			},
		}, map[string]sets.Set[int32]{
			"t1": sets.New[int32](2, 3),
			"":   sets.New[int32](0, 1),
		}, false),

		Entry("with offline instances", &workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To[int32](4),
				Instances: []workloads.InstanceTemplate{
					{
						Name:     "t1",
						Replicas: ptr.To[int32](2),
					},
				},
				OfflineInstances: []string{"foo-2"},
			},
			Status: workloads.InstanceSetStatus{
				CurrentInstances: workloads.CurrentInstances{
					"":   []int32{0, 1, 2},
					"t1": []int32{3},
				},
			},
		}, map[string]sets.Set[int32]{
			"":   sets.New[int32](0, 1),
			"t1": sets.New[int32](3, 4),
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
			},
		}

		tree := kubebuilderx.NewObjectTree()
		itsExt, err := BuildInstanceSetExt(its, tree)
		Expect(err).NotTo(HaveOccurred())
		names, err := GenerateAllInstanceNames(itsExt)
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
				GetObject()
		})

		It("build an its with default template only", func() {
			itsExt, err := BuildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			nameTemplate, err := BuildInstanceName2TemplateMap(itsExt)
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
			nameTemplate, err := BuildInstanceName2TemplateMap(itsExt)
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
})
