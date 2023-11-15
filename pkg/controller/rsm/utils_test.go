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

package rsm

import (
	"context"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("utils test", func() {
	var priorityMap map[string]int

	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetService(&corev1.Service{}).
			SetRoles(roles).
			GetObject()
		priorityMap = ComposeRolePriorityMap(rsm.Spec.Roles)
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
				*builder.NewPodBuilder(namespace, "pod-0").AddLabels(roleLabelKey, "follower").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-1").AddLabels(roleLabelKey, "logger").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-2").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-3").AddLabels(roleLabelKey, "learner").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-4").AddLabels(roleLabelKey, "candidate").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-5").AddLabels(roleLabelKey, "leader").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-6").AddLabels(roleLabelKey, "learner").GetObject(),
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
					ReplicaRole: workloads.ReplicaRole{Name: "follower"},
				},
				{
					PodName:     "pod-1",
					ReplicaRole: workloads.ReplicaRole{Name: "learner"},
				},
				{
					PodName:     "pod-2",
					ReplicaRole: workloads.ReplicaRole{Name: "learner"},
				},
				{
					PodName:     "pod-3",
					ReplicaRole: workloads.ReplicaRole{Name: "leader"},
				},
				{
					PodName:     "pod-4",
					ReplicaRole: workloads.ReplicaRole{Name: "logger"},
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
				*builder.NewPodBuilder(namespace, "pod-0").AddLabels(roleLabelKey, "follower").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-1").AddLabels(roleLabelKey, "leader").GetObject(),
				*builder.NewPodBuilder(namespace, "pod-2").AddLabels(roleLabelKey, "follower").GetObject(),
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
					ReplicaRole: workloads.ReplicaRole{Name: "leader"},
				},
				{
					PodName:     "pod-1",
					ReplicaRole: workloads.ReplicaRole{Name: "follower"},
				},
				{
					PodName:     "pod-2",
					ReplicaRole: workloads.ReplicaRole{Name: "follower"},
				},
			}
			replicas := int32(3)
			rsm.Spec.Replicas = &replicas
			rsm.Status.MembersStatus = oldMembersStatus
			setMembersStatus(rsm, pods)

			Expect(rsm.Status.MembersStatus).Should(HaveLen(2))
			Expect(rsm.Status.MembersStatus[0].PodName).Should(Equal("pod-1"))
			Expect(rsm.Status.MembersStatus[0].Name).Should(Equal("leader"))
			Expect(rsm.Status.MembersStatus[1].PodName).Should(Equal("pod-0"))
			Expect(rsm.Status.MembersStatus[1].Name).Should(Equal("follower"))
		})
	})

	Context("getRoleName function", func() {
		It("should work well", func() {
			pod := builder.NewPodBuilder(namespace, name).AddLabels(roleLabelKey, "LEADER").GetObject()
			role := getRoleName(*pod)
			Expect(role).Should(Equal("leader"))
		})
	})

	Context("getPodsOfStatefulSet function", func() {
		It("should work well", func() {
			sts := builder.NewStatefulSetBuilder(namespace, name).
				AddMatchLabels(constant.KBManagedByKey, kindReplicatedStateMachine).
				AddMatchLabels(constant.AppInstanceLabelKey, name).
				GetObject()
			pod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				AddLabels(constant.KBManagedByKey, kindReplicatedStateMachine).
				AddLabels(constant.AppInstanceLabelKey, name).
				GetObject()
			k8sMock.EXPECT().
				List(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, podList *corev1.PodList, _ ...client.ListOption) error {
					Expect(podList).ShouldNot(BeNil())
					podList.Items = []corev1.Pod{*pod}
					return nil
				}).Times(1)

			pods, err := getPodsOfStatefulSet(ctx, k8sMock, sts)
			Expect(err).Should(BeNil())
			Expect(pods).Should(HaveLen(1))
			Expect(pods[0].Namespace).Should(Equal(pod.Namespace))
			Expect(pods[0].Name).Should(Equal(pod.Name))
		})
	})

	Context("getHeadlessSvcName function", func() {
		It("should work well", func() {
			Expect(getHeadlessSvcName(*rsm)).Should(Equal("bar-headless"))
		})
	})

	Context("findSvcPort function", func() {
		It("should work well", func() {
			By("set port name")
			rsm.Spec.Service.Spec.Ports = []corev1.ServicePort{
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
			rsm.Spec.Template = corev1.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}
			Expect(findSvcPort(*rsm)).Should(BeEquivalentTo(containerPort))

			By("set port number")
			rsm.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "svc-port",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromInt(int(containerPort)),
				},
			}
			Expect(findSvcPort(*rsm)).Should(BeEquivalentTo(containerPort))

			By("set no matched port")
			rsm.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "svc-port",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromInt(int(containerPort - 1)),
				},
			}
			Expect(findSvcPort(*rsm)).Should(BeZero())
		})
	})

	Context("getPodName function", func() {
		It("should work well", func() {
			Expect(getPodName(name, 1)).Should(Equal("bar-1"))
		})
	})

	Context("getActionName function", func() {
		It("should work well", func() {
			Expect(getActionName(name, 1, 2, jobTypeSwitchover)).Should(Equal("bar-1-2-switchover"))
		})
	})

	Context("getLeaderPodName function", func() {
		It("should work well", func() {
			By("set leader")
			membersStatus := []workloads.MemberStatus{
				{
					PodName:     "pod-0",
					ReplicaRole: workloads.ReplicaRole{Name: "leader", IsLeader: true},
				},
				{
					PodName:     "pod-1",
					ReplicaRole: workloads.ReplicaRole{Name: "follower"},
				},
				{
					PodName:     "pod-2",
					ReplicaRole: workloads.ReplicaRole{Name: "follower"},
				},
			}
			Expect(getLeaderPodName(membersStatus)).Should(Equal(membersStatus[0].PodName))

			By("set no leader")
			membersStatus[0].IsLeader = false
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

	Context("findActionImage function", func() {
		It("should work well", func() {
			Expect(findActionImage(&workloads.MembershipReconfiguration{}, jobTypePromote)).Should(Equal(defaultActionImage))
		})
	})

	Context("getActionCommand function", func() {
		It("should work well", func() {
			reconfiguration := &workloads.MembershipReconfiguration{
				SwitchoverAction:  &workloads.Action{Command: []string{"switchover"}},
				MemberJoinAction:  &workloads.Action{Command: []string{"member-join"}},
				MemberLeaveAction: &workloads.Action{Command: []string{"member-leave"}},
				LogSyncAction:     &workloads.Action{Command: []string{"log-sync"}},
				PromoteAction:     &workloads.Action{Command: []string{"promote"}},
			}

			Expect(getActionCommand(reconfiguration, jobTypeSwitchover)).Should(Equal(reconfiguration.SwitchoverAction.Command))
			Expect(getActionCommand(reconfiguration, jobTypeMemberJoinNotifying)).Should(Equal(reconfiguration.MemberJoinAction.Command))
			Expect(getActionCommand(reconfiguration, jobTypeMemberLeaveNotifying)).Should(Equal(reconfiguration.MemberLeaveAction.Command))
			Expect(getActionCommand(reconfiguration, jobTypeLogSync)).Should(Equal(reconfiguration.LogSyncAction.Command))
			Expect(getActionCommand(reconfiguration, jobTypePromote)).Should(Equal(reconfiguration.PromoteAction.Command))
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
			rsm := &workloads.ReplicatedStateMachine{}
			Expect(IsOwnedByRsm(rsm)).Should(BeFalse())

			By("call with ownerReference's kind is rsm")
			t := true
			rsm.OwnerReferences = []metav1.OwnerReference{
				{
					Kind:       kindReplicatedStateMachine,
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
