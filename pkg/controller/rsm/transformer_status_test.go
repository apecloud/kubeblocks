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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("object status transformer test.", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			AddMatchLabelsInMap(selectors).
			SetServiceName(headlessSvcName).
			SetReplicas(3).
			SetRoles(roles).
			SetMembershipReconfiguration(&reconfiguration).
			SetService(service).
			GetObject()

		transCtx = &rsmTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			rsmOrig:       rsm.DeepCopy(),
			rsm:           rsm,
		}

		dag = mockDAG()

		nodeAssignment := []workloads.NodeAssignment{
			{
				Name: name + "1",
			},
			{
				Name: name + "2",
			},
			{
				Name: name + "3",
			},
		}
		rsmForPods = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			AddMatchLabelsInMap(selectors).
			SetServiceName(headlessSvcName).
			SetRsmTransformPolicy(workloads.ToPod).
			SetReplicas(3).
			SetNodeAssignment(nodeAssignment).
			SetMembershipReconfiguration(&reconfiguration).
			SetService(service).
			GetObject()

		transCtxForPods = &rsmTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			rsmOrig:       rsmForPods.DeepCopy(),
			rsm:           rsmForPods,
		}

		dagForPods = mockDAGForPods()

		transformer = &ObjectStatusTransformer{}
	})

	Context("rsm deletion", func() {
		It("should return directly", func() {
			ts := metav1.NewTime(time.Now())
			transCtx.rsmOrig.DeletionTimestamp = &ts
			transCtx.rsm.DeletionTimestamp = &ts
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			dagExpected := mockDAG()
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})

	Context("rsm creation when manage pods", func() {
		It("should return directly", func() {
			ts := metav1.NewTime(time.Now())
			transCtxForPods.rsmOrig.DeletionTimestamp = &ts
			transCtxForPods.rsm.DeletionTimestamp = &ts
			Expect(transformer.Transform(transCtxForPods, dagForPods)).Should(Succeed())
			dagExpected := mockDAG()
			Expect(dagForPods.Equals(dagExpected, less)).Should(BeTrue())
		})
	})

	Context("rsm update", func() {
		It("should work well", func() {
			generation := int64(2)
			rsm.Generation = generation
			rsm.Status.ObservedGeneration = generation - 1
			transCtx.rsmOrig = rsm.DeepCopy()
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			dagExpected := mockDAG()
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
			root, err := model.FindRootVertex(dag)
			Expect(err).Should(BeNil())
			Expect(root.Action).ShouldNot(BeNil())
			Expect(*root.Action).Should(Equal(model.STATUS))
			rsmNew, ok := root.Obj.(*workloads.ReplicatedStateMachine)
			Expect(ok).Should(BeTrue())
			Expect(rsmNew.Generation).Should(Equal(generation))
			Expect(rsmNew.Status.ObservedGeneration).Should(Equal(generation))
		})
	})

	Context("rsm status update", func() {
		It("should work well", func() {
			generation := int64(2)
			rsm.Generation = generation
			rsm.Status.ObservedGeneration = generation
			rsm.Status.UpdateRevision = newRevision
			transCtx.rsmOrig = rsm.DeepCopy()
			sts := mockUnderlyingSts(*rsm, 1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					*obj = *sts
					return nil
				}).Times(1)
			pod0 := builder.NewPodBuilder(namespace, getPodName(rsm.Name, 0)).
				AddLabels(roleLabelKey, "follower").
				GetObject()
			pod1 := builder.NewPodBuilder(namespace, getPodName(name, 1)).
				AddLabels(roleLabelKey, "leader").
				GetObject()
			pod2 := builder.NewPodBuilder(namespace, getPodName(name, 2)).
				AddLabels(roleLabelKey, "follower").
				GetObject()
			makePodUpdateReady(newRevision, pod0, pod1, pod2)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Pod{*pod0, *pod1, *pod2}
					return nil
				}).Times(1)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			dagExpected := mockDAG()
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
			root, err := model.FindRootVertex(dag)
			Expect(err).Should(BeNil())
			Expect(root.Action).ShouldNot(BeNil())
			Expect(*root.Action).Should(Equal(model.STATUS))
			rsmNew, ok := root.Obj.(*workloads.ReplicatedStateMachine)
			Expect(ok).Should(BeTrue())
			Expect(rsmNew.Status.ObservedGeneration).Should(Equal(generation))
			// the only difference between rsm.status.StatefulSetStatus and sts.status is ObservedGeneration
			// for less coding
			rsmNew.Status.ObservedGeneration = sts.Status.ObservedGeneration
			Expect(rsmNew.Status.StatefulSetStatus).Should(Equal(sts.Status))
			pods := []*corev1.Pod{pod0, pod1, pod2}
			for _, pod := range pods {
				matched := false
				for _, status := range rsmNew.Status.MembersStatus {
					if status.PodName == pod.Name && status.ReplicaRole.Name == pod.Labels[roleLabelKey] {
						matched = true
					}
				}
				Expect(matched).Should(BeTrue())
			}
		})
	})

	Context("rsm status update when manages pods", func() {
		It("should work well", func() {
			generation := int64(2)
			rsmForPods.Generation = generation
			rsmForPods.Status.ObservedGeneration = generation
			rsmForPods.Status.UpdateRevision = newRevision
			transCtxForPods.rsmOrig = rsmForPods.DeepCopy()
			pods := buildPods(*rsmForPods)
			makePodUpdateReady(newRevision, pods...)
			podList := make([]corev1.Pod, len(pods))
			for i := range pods {
				podList[i] = *pods[i]
			}
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = podList
					return nil
				}).Times(1)
			Expect(transformer.Transform(transCtxForPods, dagForPods)).Should(Succeed())
			dagExpected := mockDAGForPods()
			Expect(dagForPods.Equals(dagExpected, less)).Should(BeTrue())
			root, err := model.FindRootVertex(dagForPods)
			Expect(err).Should(BeNil())
			Expect(root.Action).ShouldNot(BeNil())
			Expect(*root.Action).Should(Equal(model.STATUS))
			rsmNew, ok := root.Obj.(*workloads.ReplicatedStateMachine)
			Expect(ok).Should(BeTrue())
			Expect(rsmNew.Status.ObservedGeneration).Should(Equal(generation))

			rsmNew.Status.Replicas = int32(len(pods))
			rsmNew.Status.AvailableReplicas = rsmNew.Status.Replicas
			rsmNew.Status.ReadyReplicas = rsmNew.Status.Replicas
			rsmNew.Status.UpdatedReplicas = rsmNew.Status.Replicas
			Expect(rsmNew.Status.StatefulSetStatus).Should(Equal(rsmForPods.Status.StatefulSetStatus))
		})
	})
})
