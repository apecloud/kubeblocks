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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
)

var _ = Describe("utils test", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	var (
		roles       []workloads.ReplicaRole
		rsm         *workloads.ReplicatedStateMachine
		priorityMap map[string]int
	)

	BeforeEach(func() {
		roles = []workloads.ReplicaRole{
			{
				Name:       "leader",
				IsLeader:   true,
				CanVote:    true,
				AccessMode: workloads.ReadWriteMode,
			},
			{
				Name:       "follower",
				IsLeader:   false,
				CanVote:    true,
				AccessMode: workloads.ReadonlyMode,
			},
			{
				Name:       "logger",
				IsLeader:   false,
				CanVote:    true,
				AccessMode: workloads.NoneMode,
			},
			{
				Name:       "learner",
				IsLeader:   false,
				CanVote:    false,
				AccessMode: workloads.ReadonlyMode,
			},
		}
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).SetRoles(roles).GetObject()
		priorityMap = composeRolePriorityMap(*rsm)
	})

	Context("composeRolePriorityMap function", func() {
		It("should work well", func() {
			priorityList := []int{
				leaderPriority,
				followerReadonlyPriority,
				followerNonePriority,
				learnerPriority,
			}
			Expect(priorityMap).ShouldNot(BeZero())
			Expect(len(priorityMap)).Should(Equal(len(roles) + 1))
			for i, role := range roles {
				Expect(priorityMap[role.Name]).Should(Equal(priorityList[i]))
			}
		})
	})

	Context("sortPods function", func() {
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

			sortPods(pods, priorityMap, false)
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
				Type: corev1.PodReady,
				Status: corev1.ConditionTrue,
			}
			pods[0].Status.Conditions = append(pods[0].Status.Conditions, readyCondition)
			pods[1].Status.Conditions = append(pods[1].Status.Conditions, readyCondition)
			oldMembersStatus := []workloads.MemberStatus{
				{
					PodName: "pod-0",
					ReplicaRole: workloads.ReplicaRole{Name: "leader"},
				},
				{
					PodName: "pod-1",
					ReplicaRole: workloads.ReplicaRole{Name: "follower"},
				},
				{
					PodName: "pod-2",
					ReplicaRole: workloads.ReplicaRole{Name: "follower"},
				},
			}
			rsm.Spec.Replicas = 3
			rsm.Status.MembersStatus = oldMembersStatus
			setMembersStatus(rsm, pods)

			Expect(len(rsm.Status.MembersStatus)).Should(Equal(len(oldMembersStatus)))
			Expect(rsm.Status.MembersStatus[0].PodName).Should(Equal("pod-1"))
			Expect(rsm.Status.MembersStatus[0].Name).Should(Equal("leader"))
			Expect(rsm.Status.MembersStatus[1].PodName).Should(Equal("pod-2"))
			Expect(rsm.Status.MembersStatus[1].Name).Should(Equal("follower"))
			Expect(rsm.Status.MembersStatus[2].PodName).Should(Equal("pod-0"))
			Expect(rsm.Status.MembersStatus[2].Name).Should(Equal("follower"))
		})
	})

	Context("getRoleName function", func() {
		It("should work well", func() {
			pod := builder.NewPodBuilder(namespace, name).AddLabels(roleLabelKey, "LEADER").GetObject()
			role := getRoleName(*pod)
			Expect(role).Should(Equal("leader"))
		})
	})
})
