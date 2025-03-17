package instancetemplate

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var _ = Describe("GenerateAllInstanceNames", func() {
	DescribeTable("generates instance ordinals",
		func(its *workloads.InstanceSet, expected map[string]sets.Set[int32], expectError bool) {
			Expect(validateOrdinals(its.Spec.Instances)).To(Succeed())
			ordinals, err := GenerateTemplateName2OrdinalMap(its)
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

		names, err := GenerateAllInstanceNames(its)
		Expect(err).NotTo(HaveOccurred())
		Expect(names).To(Equal([]string{"foo-0", "foo-1", "foo-2", "foo-3", "foo-4"}))
	})
})
