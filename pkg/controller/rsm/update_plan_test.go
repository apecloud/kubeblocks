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

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("update plan test.", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).SetRoles(roles).GetObject()
		rsm.Status.UpdateRevision = newRevision
	})

	Context("plan build&execute", func() {
		var pod0, pod1, pod2, pod3, pod4, pod5, pod6 *corev1.Pod

		resetPods := func() {
			pod0 = builder.NewPodBuilder(namespace, getPodName(name, 0)).
				AddLabels(roleLabelKey, "follower").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()

			pod1 = builder.NewPodBuilder(namespace, getPodName(name, 1)).
				AddLabels(roleLabelKey, "logger").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()

			pod2 = builder.NewPodBuilder(namespace, getPodName(name, 2)).
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()

			pod3 = builder.NewPodBuilder(namespace, getPodName(name, 3)).
				AddLabels(roleLabelKey, "learner").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()

			pod4 = builder.NewPodBuilder(namespace, getPodName(name, 4)).
				AddLabels(roleLabelKey, "candidate").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()

			pod5 = builder.NewPodBuilder(namespace, getPodName(name, 5)).
				AddLabels(roleLabelKey, "leader").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()

			pod6 = builder.NewPodBuilder(namespace, getPodName(name, 6)).
				AddLabels(roleLabelKey, "learner").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()
		}

		buildPodList := func() []corev1.Pod {
			return []corev1.Pod{*pod0, *pod1, *pod2, *pod3, *pod4, *pod5, *pod6}
		}

		toPodList := func(pods []*corev1.Pod) []corev1.Pod {
			var list []corev1.Pod
			for _, pod := range pods {
				list = append(list, *pod)
			}
			return list
		}

		equalPodList := func(podList1, podList2 []corev1.Pod) bool {
			set1 := sets.New[string]()
			set2 := sets.New[string]()
			for _, pod := range podList1 {
				set1.Insert(pod.Name)
			}
			for _, pod := range podList2 {
				set2.Insert(pod.Name)
			}
			return set1.Equal(set2)
		}

		checkPlan := func(expectedPlan [][]*corev1.Pod) {
			for i, expectedPods := range expectedPlan {
				if i > 0 {
					makePodUpdateReady(newRevision, expectedPlan[i-1]...)
				}
				pods := buildPodList()
				plan := newUpdatePlan(*rsm, pods)
				podUpdateList, err := plan.execute()
				Expect(err).Should(BeNil())
				podList := toPodList(podUpdateList)
				expectedPodList := toPodList(expectedPods)
				Expect(equalPodList(podList, expectedPodList)).Should(BeTrue())
			}
		}

		BeforeEach(func() {
			resetPods()
		})

		It("should work well in a serial plan", func() {
			By("build a serial plan")
			strategy := workloads.SerialUpdateStrategy
			rsm.Spec.MemberUpdateStrategy = &strategy
			expectedPlan := [][]*corev1.Pod{
				{pod4},
				{pod2},
				{pod3},
				{pod6},
				{pod1},
				{pod0},
				{pod5},
			}
			checkPlan(expectedPlan)
		})

		It("should work well in a parallel plan", func() {
			By("build a parallel plan")
			strategy := workloads.ParallelUpdateStrategy
			rsm.Spec.MemberUpdateStrategy = &strategy
			expectedPlan := [][]*corev1.Pod{
				{pod0, pod1, pod2, pod3, pod4, pod5, pod6},
			}
			checkPlan(expectedPlan)
		})

		It("should work well in a best effort parallel", func() {
			By("build a best effort parallel plan")
			strategy := workloads.BestEffortParallelUpdateStrategy
			rsm.Spec.MemberUpdateStrategy = &strategy
			expectedPlan := [][]*corev1.Pod{
				{pod2, pod3, pod4, pod6},
				{pod1},
				{pod0},
				{pod5},
			}
			checkPlan(expectedPlan)
		})
	})
})
