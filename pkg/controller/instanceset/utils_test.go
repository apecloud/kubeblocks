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
	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("utils test", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetService(&corev1.Service{}).
			SetRoles(roles).
			GetObject()
		priorityMap = ComposeRolePriorityMap(its.Spec.Roles)
	})

	Context("mergeList", func() {
		It("should work well", func() {
			src := []corev1.Volume{
				{
					Name: "pvc1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc1-pod-0",
						},
					},
				},
				{
					Name: "pvc2",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc2-pod-0",
						},
					},
				},
			}
			dst := []corev1.Volume{
				{
					Name: "pvc0",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc0-pod-0",
						},
					},
				},
				{
					Name: "pvc1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc-pod-0",
						},
					},
				},
			}
			mergeList(&src, &dst, func(v corev1.Volume) func(corev1.Volume) bool {
				return func(volume corev1.Volume) bool {
					return v.Name == volume.Name
				}
			})

			Expect(dst).Should(HaveLen(3))
			slices.SortStableFunc(dst, func(a, b corev1.Volume) bool {
				return a.Name < b.Name
			})
			Expect(dst[0].Name).Should(Equal("pvc0"))
			Expect(dst[1].Name).Should(Equal("pvc1"))
			Expect(dst[1].PersistentVolumeClaim).ShouldNot(BeNil())
			Expect(dst[1].PersistentVolumeClaim.ClaimName).Should(Equal("pvc1-pod-0"))
			Expect(dst[2].Name).Should(Equal("pvc2"))
		})
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
			priorityList := []int{
				leaderPriority,
				followerReadonlyPriority,
				followerNonePriority,
				learnerPriority,
			}
			Expect(priorityMap).ShouldNot(BeZero())
			Expect(priorityMap).Should(HaveLen(len(roles) + 1))
			for i, role := range roles {
				Expect(priorityMap[role.Name]).Should(Equal(priorityList[i]))
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
			expectedOrder := []string{"pod-4", "pod-2", "pod-3", "pod-6", "pod-1", "pod-0", "pod-5"}

			SortPods(pods, priorityMap, false)
			for i, pod := range pods {
				Expect(pod.Name).Should(Equal(expectedOrder[i]))
			}
		})
	})

	Context("sortMembersStatus function", func() {
		It("should work well", func() {
			// 1(learner)->2(learner)->4(logger)->0(follower)->3(leader)
			membersStatus := []workloads.MemberStatus{
				{
					PodName:     "pod-0",
					ReplicaRole: &workloads.ReplicaRole{Name: "follower"},
				},
				{
					PodName:     "pod-1",
					ReplicaRole: &workloads.ReplicaRole{Name: "learner"},
				},
				{
					PodName:     "pod-2",
					ReplicaRole: &workloads.ReplicaRole{Name: "learner"},
				},
				{
					PodName:     "pod-3",
					ReplicaRole: &workloads.ReplicaRole{Name: "leader"},
				},
				{
					PodName:     "pod-4",
					ReplicaRole: &workloads.ReplicaRole{Name: "logger"},
				},
			}
			expectedOrder := []string{"pod-3", "pod-0", "pod-4", "pod-2", "pod-1"}

			sortMembersStatus(membersStatus, priorityMap)
			for i, status := range membersStatus {
				Expect(status.PodName).Should(Equal(expectedOrder[i]))
			}
		})
	})

	Context("setMembersStatus function", func() {
		It("should work well", func() {
			pods := []corev1.Pod{
				*builder.NewPodBuilder(namespace, "pod-0").AddLabels(RoleLabelKey, "follower").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-1").AddLabels(RoleLabelKey, "leader").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-2").AddLabels(RoleLabelKey, "follower").GetObject(),
			}
			readyCondition := corev1.PodCondition{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}
			pods[0].Status.Conditions = append(pods[0].Status.Conditions, readyCondition)
			pods[1].Status.Conditions = append(pods[1].Status.Conditions, readyCondition)
			oldMembersStatus := []workloads.MemberStatus{
				{
					PodName:     "pod-0",
					ReplicaRole: &workloads.ReplicaRole{Name: "leader"},
				},
				{
					PodName:     "pod-1",
					ReplicaRole: &workloads.ReplicaRole{Name: "follower"},
				},
				{
					PodName:     "pod-2",
					ReplicaRole: &workloads.ReplicaRole{Name: "follower"},
				},
			}
			replicas := int32(3)
			its.Spec.Replicas = &replicas
			its.Status.MembersStatus = oldMembersStatus
			setMembersStatus(its, &pods)

			Expect(its.Status.MembersStatus).Should(HaveLen(2))
			Expect(its.Status.MembersStatus[0].PodName).Should(Equal("pod-1"))
			Expect(its.Status.MembersStatus[0].ReplicaRole.Name).Should(Equal("leader"))
			Expect(its.Status.MembersStatus[1].PodName).Should(Equal("pod-0"))
			Expect(its.Status.MembersStatus[1].ReplicaRole.Name).Should(Equal("follower"))
		})
	})

	Context("GetRoleName function", func() {
		It("should work well", func() {
			pod := builder.NewPodBuilder(namespace, name).AddLabels(RoleLabelKey, "LEADER").GetObject()
			role := GetRoleName(*pod)
			Expect(role).Should(Equal("leader"))
		})
	})

	Context("getHeadlessSvcName function", func() {
		It("should work well", func() {
			Expect(getHeadlessSvcName(*its)).Should(Equal("bar-headless"))
		})
	})

	Context("findSvcPort function", func() {
		It("should work well", func() {
			By("set port name")
			its.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "svc-port",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromString("my-service"),
				},
			}
			containerPort := int32(54321)
			container := corev1.Container{
				Name: name,
				Ports: []corev1.ContainerPort{
					{
						Name:          "my-service",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: containerPort,
					},
				},
			}
			pod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetContainers([]corev1.Container{container}).
				GetObject()
			its.Spec.Template = corev1.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}
			Expect(findSvcPort(its)).Should(BeEquivalentTo(containerPort))

			By("set port number")
			its.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "svc-port",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromInt(int(containerPort)),
				},
			}
			Expect(findSvcPort(its)).Should(BeEquivalentTo(containerPort))

			By("set no matched port")
			its.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "svc-port",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromInt(int(containerPort - 1)),
				},
			}
			Expect(findSvcPort(its)).Should(BeZero())
		})
	})

	Context("getPodName function", func() {
		It("should work well", func() {
			Expect(getPodName(name, 1)).Should(Equal("bar-1"))
		})
	})

	Context("getLeaderPodName function", func() {
		It("should work well", func() {
			By("set leader")
			membersStatus := []workloads.MemberStatus{
				{
					PodName:     "pod-0",
					ReplicaRole: &workloads.ReplicaRole{Name: "leader", IsLeader: true},
				},
				{
					PodName:     "pod-1",
					ReplicaRole: &workloads.ReplicaRole{Name: "follower"},
				},
				{
					PodName:     "pod-2",
					ReplicaRole: &workloads.ReplicaRole{Name: "follower"},
				},
			}
			Expect(getLeaderPodName(membersStatus)).Should(Equal(membersStatus[0].PodName))

			By("set no leader")
			membersStatus[0].ReplicaRole.IsLeader = false
			Expect(getLeaderPodName(membersStatus)).Should(BeZero())
		})
	})

	Context("getPodOrdinal function", func() {
		It("should work well", func() {
			ordinal, err := getPodOrdinal("pod-5")
			Expect(err).Should(BeNil())
			Expect(ordinal).Should(Equal(5))

			_, err = getPodOrdinal("foo-bar")
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("wrong pod name"))
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
			headlessK := "foo.headless.rsm"
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

	Context("IsOwnedByRsm function", func() {
		It("should work well", func() {
			By("call without ownerReferences")
			rsm := &workloads.InstanceSet{}
			Expect(IsOwnedByRsm(rsm)).Should(BeFalse())

			By("call with ownerReference's kind is rsm")
			t := true
			rsm.OwnerReferences = []metav1.OwnerReference{
				{
					Kind:       KindInstanceSet,
					Controller: &t,
				},
			}
			Expect(IsOwnedByRsm(rsm)).Should(BeTrue())

			By("call with ownerReference's is not rsm")
			rsm.OwnerReferences = []metav1.OwnerReference{
				{
					Kind:       reflect.TypeOf(v1alpha1.Cluster{}).Name(),
					Controller: &t,
				},
			}
			Expect(IsOwnedByRsm(rsm)).Should(BeFalse())
		})
	})
})
