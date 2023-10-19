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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("update strategy transformer test.", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			SetReplicas(3).
			SetRoles(roles).
			SetMembershipReconfiguration(&reconfiguration).
			SetService(service).
			GetObject()
		rsm.Status.UpdateRevision = newRevision
		membersStatus := []workloads.MemberStatus{
			{
				PodName:     getPodName(rsm.Name, 1),
				ReplicaRole: workloads.ReplicaRole{Name: "leader", IsLeader: true},
			},
			{
				PodName:     getPodName(rsm.Name, 0),
				ReplicaRole: workloads.ReplicaRole{Name: "follower"},
			},
			{
				PodName:     getPodName(rsm.Name, 2),
				ReplicaRole: workloads.ReplicaRole{Name: "follower"},
			},
		}
		rsm.Status.MembersStatus = membersStatus

		transCtx = &rsmTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			rsmOrig:       rsm.DeepCopy(),
			rsm:           rsm,
		}

		dag = mockDAG()
		transformer = &UpdateStrategyTransformer{}
	})

	Context("RSM is not in status updating", func() {
		It("should return directly", func() {
			transCtx.rsmOrig.Generation = 2
			transCtx.rsmOrig.Status.ObservedGeneration = 1
			dagExpected := mockDAG()

			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})

	Context("the underlying sts is not ready", func() {
		It("should return directly", func() {
			transCtx.rsmOrig.Generation = 2
			transCtx.rsmOrig.Status.ObservedGeneration = 2
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					obj.Namespace = objKey.Namespace
					obj.Name = objKey.Name
					obj.Generation = 2
					obj.Status.ObservedGeneration = 1
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			dagExpected := mockDAG()

			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})

	Context("pods are not ready", func() {
		It("should return directly", func() {
			transCtx.rsmOrig.Generation = 2
			transCtx.rsmOrig.Status.ObservedGeneration = 2
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					obj.Namespace = objKey.Namespace
					obj.Name = objKey.Name
					obj.Generation = 2
					obj.Status.ObservedGeneration = obj.Generation
					obj.Spec.Replicas = rsm.Spec.Replicas
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			dagExpected := mockDAG()

			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})

	Context("all ready for updating", func() {
		It("should update all pods", func() {
			transCtx.rsmOrig.Generation = 2
			transCtx.rsmOrig.Status.ObservedGeneration = 2
			strategy := workloads.SerialUpdateStrategy
			rsm.Spec.MemberUpdateStrategy = &strategy
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					obj.Namespace = objKey.Namespace
					obj.Name = objKey.Name
					obj.Generation = 2
					obj.Status.ObservedGeneration = obj.Generation
					obj.Spec.Replicas = rsm.Spec.Replicas
					return nil
				}).Times(4)
			pod0 := builder.NewPodBuilder(namespace, getPodName(rsm.Name, 0)).
				AddLabels(roleLabelKey, "follower").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()
			pod1 := builder.NewPodBuilder(namespace, getPodName(name, 1)).
				AddLabels(roleLabelKey, "leader").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()
			pod2 := builder.NewPodBuilder(namespace, getPodName(name, 2)).
				AddLabels(roleLabelKey, "follower").
				AddLabels(apps.StatefulSetRevisionLabel, oldRevision).
				GetObject()
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Pod{*pod0, *pod1, *pod2}
					return nil
				}).Times(1)

			By("update the first pod")
			dagExpected := mockDAG()
			graphCli.Delete(dagExpected, pod0)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("update the second pod")
			makePodUpdateReady(newRevision, pod0)
			dagExpected = mockDAG()
			graphCli.Delete(dagExpected, pod2)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Pod{*pod0, *pod1, *pod2}
					return nil
				}).Times(1)
			dag = mockDAG()
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("switchover")
			makePodUpdateReady(newRevision, pod2)
			dagExpected = mockDAG()
			actionName := getActionName(rsm.Name, int(rsm.Generation), 1, jobTypeSwitchover)
			action := builder.NewJobBuilder(name, actionName).GetObject()
			graphCli.Create(dagExpected, action)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Pod{*pod0, *pod1, *pod2}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			dag = mockDAG()
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("update the last(leader) pod")
			dagExpected = mockDAG()
			action = builder.NewJobBuilder(name, actionName).
				AddLabelsInMap(map[string]string{
					constant.AppInstanceLabelKey: rsm.Name,
					constant.KBManagedByKey:      kindReplicatedStateMachine,
					jobScenarioLabel:             jobScenarioUpdate,
					jobTypeLabel:                 jobTypeSwitchover,
					jobHandledLabel:              jobHandledFalse,
				}).
				SetSuspend(false).
				GetObject()
			action.Status.Succeeded = 1
			graphCli.Update(dagExpected, action, action)
			graphCli.Delete(dagExpected, pod1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Pod{*pod0, *pod1, *pod2}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []batchv1.Job{*action}
					return nil
				}).Times(1)
			dag = mockDAG()
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})
})
