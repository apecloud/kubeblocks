/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package instanceset

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("utils test", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetRoles(roles).
			GetObject()
		priorityMap = ComposeRolePriorityMap(its.Spec.Roles)
	})

	Context("mergeMap", func() {
		It("should work well", func() {
			src := map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			}
			dst := map[string]string{
				"foo0": "bar0",
				"foo1": "bar",
			}
			mergeMap(&src, &dst)

			Expect(dst).Should(HaveLen(3))
			Expect(dst).Should(HaveKey("foo0"))
			Expect(dst).Should(HaveKey("foo1"))
			Expect(dst).Should(HaveKey("foo2"))
			Expect(dst["foo1"]).Should(Equal("bar1"))
		})
	})

	Context("ComposeRolePriorityMap function", func() {
		It("should work well", func() {
			Expect(priorityMap).ShouldNot(BeZero())
			Expect(priorityMap).Should(HaveLen(len(roles) + 1))
			for _, role := range roles {
				Expect(priorityMap[role.Name]).Should(Equal(role.UpdatePriority))
			}
		})
	})

	Context("SortPods function", func() {
		It("should work well", func() {
			pods := []corev1.Pod{
				*builder.NewPodBuilder(namespace, "pod-0").AddLabels(RoleLabelKey, "follower").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-1").AddLabels(RoleLabelKey, "logger").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-2").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-3").AddLabels(RoleLabelKey, "learner").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-4").AddLabels(RoleLabelKey, "candidate").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-5").AddLabels(RoleLabelKey, "leader").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-6").AddLabels(RoleLabelKey, "learner").GetObject(),
			}
			expectedOrder := []string{"pod-4", "pod-2", "pod-6", "pod-3", "pod-1", "pod-0", "pod-5"}

			SortPods(pods, priorityMap, false)
			for i, pod := range pods {
				Expect(pod.Name).Should(Equal(expectedOrder[i]))
			}
		})
	})

	Context("getRoleName function", func() {
		It("should work well", func() {
			pod := builder.NewPodBuilder(namespace, name).AddLabels(RoleLabelKey, "LEADER").GetObject()
			role := getRoleName(pod)
			Expect(role).Should(Equal("leader"))
		})
	})

	Context("AddAnnotationScope function", func() {
		It("should work well", func() {
			By("call with a nil map")
			var annotations map[string]string
			Expect(AddAnnotationScope(HeadlessServiceScope, annotations)).Should(BeNil())

			By("call with an empty map")
			annotations = make(map[string]string, 0)
			scopedAnnotations := AddAnnotationScope(HeadlessServiceScope, annotations)
			Expect(scopedAnnotations).ShouldNot(BeNil())
			Expect(scopedAnnotations).Should(HaveLen(0))

			By("call with none empty map")
			annotations["foo"] = "bar"
			annotations["foo/bar"] = "foo.bar"
			annotations["foo.bar/bar"] = "foo.bar.bar"
			scopedAnnotations = AddAnnotationScope(HeadlessServiceScope, annotations)
			Expect(scopedAnnotations).ShouldNot(BeNil())
			Expect(scopedAnnotations).Should(HaveLen(len(annotations)))
			for k, v := range annotations {
				nk := fmt.Sprintf("%s%s", k, HeadlessServiceScope)
				nv, ok := scopedAnnotations[nk]
				Expect(ok).Should(BeTrue())
				Expect(nv).Should(Equal(v))
			}
		})
	})

	Context("ParseAnnotationsOfScope function", func() {
		It("should work well", func() {
			By("call with a nil map")
			var scopedAnnotations map[string]string
			Expect(ParseAnnotationsOfScope(HeadlessServiceScope, scopedAnnotations)).Should(BeNil())

			By("call with an empty map")
			scopedAnnotations = make(map[string]string, 0)
			annotations := ParseAnnotationsOfScope(HeadlessServiceScope, scopedAnnotations)
			Expect(annotations).ShouldNot(BeNil())
			Expect(annotations).Should(HaveLen(0))

			By("call with RootScope")
			scopedAnnotations["foo"] = "bar"
			scopedAnnotations["foo.bar"] = "foo.bar"
			headlessK := "foo.headless.its"
			scopedAnnotations[headlessK] = headlessK
			annotations = ParseAnnotationsOfScope(RootScope, scopedAnnotations)
			Expect(annotations).ShouldNot(BeNil())
			Expect(annotations).Should(HaveLen(2))
			delete(scopedAnnotations, headlessK)
			for k, v := range scopedAnnotations {
				nv, ok := annotations[k]
				Expect(ok).Should(BeTrue())
				Expect(nv).Should(Equal(v))
			}

			By("call with none RootScope")
			scopedAnnotations[headlessK] = headlessK
			annotations = ParseAnnotationsOfScope(HeadlessServiceScope, scopedAnnotations)
			Expect(annotations).Should(HaveLen(1))
			Expect(annotations["foo"]).Should(Equal(headlessK))
		})
	})

	Context("IsInstanceSetReady", func() {
		It("should work well", func() {
			By("set its to nil")
			its = nil
			Expect(IsInstanceSetReady(its)).Should(BeFalse())

			By("set its to not initialized")
			replicas := int32(3)
			its = builder.NewInstanceSetBuilder(namespace, name).
				SetRoles(roles).
				SetReplicas(replicas).
				GetObject()
			its.Status = workloads.InstanceSetStatus{
				InitReplicas: replicas,
			}
			Expect(IsInstanceSetReady(its)).Should(BeFalse())

			By("set its.status.observedGeneration to not equal generation")
			its.Status.ReadyInitReplicas = replicas
			its.Generation = 1
			Expect(IsInstanceSetReady(its)).Should(BeFalse())

			By("set its.status.replicas to not as expected")
			its.Status.ObservedGeneration = its.Generation
			its.Status.Replicas = replicas - 1
			Expect(IsInstanceSetReady(its)).Should(BeFalse())

			By("set spec.minReadySeconds to not zero")
			its.Status.Replicas = replicas
			its.Status.ReadyReplicas = replicas
			its.Status.UpdatedReplicas = replicas
			its.Status.AvailableReplicas = replicas - 1
			its.Spec.MinReadySeconds = int32(5)
			Expect(IsInstanceSetReady(its)).Should(BeFalse())

			By("set its to role-less")
			its.Status.AvailableReplicas = replicas
			its.Spec.Roles = nil
			its.Spec.RoleProbe = nil
			Expect(IsInstanceSetReady(its)).Should(BeTrue())

			By("set its to role-ful")
			its.Spec.Roles = roles
			its.Spec.RoleProbe = &workloads.RoleProbe{}
			Expect(IsInstanceSetReady(its)).Should(BeFalse())

			By("set membersStatus to ready")
			its.Status.MembersStatus = []workloads.MemberStatus{
				{
					PodName:     name + "-0",
					ReplicaRole: &roles[0],
				},
				{
					PodName:     name + "-1",
					ReplicaRole: &roles[1],
				},
				{
					PodName:     name + "-2",
					ReplicaRole: &roles[2],
				},
			}
			Expect(IsInstanceSetReady(its)).Should(BeTrue())
		})
	})

	Context("CalculateConcurrencyReplicas function", func() {
		It("should work well", func() {
			By("concurrency = 50%, replicas = 10")
			concurrent := &intstr.IntOrString{Type: intstr.String, StrVal: "50%"}
			replicas := 10
			concurrencyReplicas, err := CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(5))

			By("concurrency = 50%, replicas = 0")
			concurrent = &intstr.IntOrString{Type: intstr.String, StrVal: "50%"}
			replicas = 0
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(1))

			By("concurrency = 0%, replicas = 10")
			concurrent = &intstr.IntOrString{Type: intstr.String, StrVal: "0%"}
			replicas = 10
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(1))

			By("concurrency = 5, replicas = 10")
			concurrent = &intstr.IntOrString{Type: intstr.Int, IntVal: 5}
			replicas = 10
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(5))

			By("concurrency = 5, replicas = 0")
			concurrent = &intstr.IntOrString{Type: intstr.Int, IntVal: 5}
			replicas = 0
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(1))

			By("concurrency = 0, replicas = 10")
			concurrent = &intstr.IntOrString{Type: intstr.Int, IntVal: 0}
			replicas = 10
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(1))

			By("concurrency = 10%, replicas = 1")
			concurrent = &intstr.IntOrString{Type: intstr.String, StrVal: "10%"}
			replicas = 1
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(1))

			By("concurrency = 60%, replicas = 2")
			concurrent = &intstr.IntOrString{Type: intstr.String, StrVal: "60%"}
			replicas = 2
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(1))

			By("concurrency is nil")
			replicas = 10
			concurrencyReplicas, err = CalculateConcurrencyReplicas(nil, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(10))

			By("concurrency is a string but not a percentage")
			replicas = 10
			concurrent = &intstr.IntOrString{Type: intstr.String, StrVal: "50"}
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			ErrConcurrencyNotPercentage := fmt.Errorf("invalid value for IntOrString: invalid type: string is not a percentage")
			Expect(err).Should(Equal(ErrConcurrencyNotPercentage))
			Expect(concurrencyReplicas).Should(Equal(0))

			By("concurrency percentage < 0%")
			replicas = 10
			concurrent = &intstr.IntOrString{Type: intstr.String, StrVal: "-50%"}
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(1))

			By("concurrency percentage > 100%")
			replicas = 10
			concurrent = &intstr.IntOrString{Type: intstr.String, StrVal: "150%"}
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(10))

			By("concurrency type neither int nor percentage")
			replicas = 10
			unknownType := intstr.Type(2)
			concurrent = &intstr.IntOrString{Type: unknownType, StrVal: "50%"}
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			ErrConcurrencyInvalidType := fmt.Errorf("invalid value for IntOrString: invalid type: neither int nor percentage")
			Expect(err).Should(Equal(ErrConcurrencyInvalidType))
			Expect(concurrencyReplicas).Should(Equal(0))

			By("concurrency > replicas")
			replicas = 10
			concurrent = &intstr.IntOrString{Type: intstr.Int, IntVal: 15}
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(10))

			By("concurrent is nil, replicas = 10")
			replicas = 10
			concurrent = nil
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(10))

			By("concurrent is nil, replicas = 0")
			replicas = 0
			concurrent = nil
			concurrencyReplicas, err = CalculateConcurrencyReplicas(concurrent, replicas)
			Expect(err).Should(BeNil())
			Expect(concurrencyReplicas).Should(Equal(1))
		})
	})
})
