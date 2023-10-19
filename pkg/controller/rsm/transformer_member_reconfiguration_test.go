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
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("member reconfiguration transformer test.", func() {
	buildMembersStatus := func(replicas int) []workloads.MemberStatus {
		var membersStatus []workloads.MemberStatus
		for i := 0; i < replicas; i++ {
			status := workloads.MemberStatus{
				PodName:     getPodName(rsm.Name, i),
				ReplicaRole: workloads.ReplicaRole{Name: "follower"},
			}
			membersStatus = append(membersStatus, status)
		}
		leaderIndex := 0
		if replicas > 1 {
			leaderIndex = 1
		}
		membersStatus[leaderIndex].ReplicaRole = workloads.ReplicaRole{Name: "leader", IsLeader: true}
		return membersStatus
	}
	setRSMStatus := func(replicas int) {
		membersStatus := buildMembersStatus(replicas)
		rsm.Status.InitReplicas = 3
		rsm.Status.ReadyInitReplicas = rsm.Status.InitReplicas
		rsm.Status.MembersStatus = membersStatus
		rsm.Status.Replicas = *rsm.Spec.Replicas
		rsm.Status.ReadyReplicas = rsm.Status.Replicas
		rsm.Status.AvailableReplicas = rsm.Status.Replicas
		rsm.Status.UpdatedReplicas = rsm.Status.Replicas
	}
	mockAction := func(ordinal int, actionType string, succeed bool) *batchv1.Job {
		actionName := getActionName(rsm.Name, int(rsm.Generation), ordinal, actionType)
		action := builder.NewJobBuilder(name, actionName).
			AddLabelsInMap(map[string]string{
				constant.AppInstanceLabelKey: rsm.Name,
				constant.KBManagedByKey:      kindReplicatedStateMachine,
				jobScenarioLabel:             jobScenarioMembership,
				jobTypeLabel:                 actionType,
				jobHandledLabel:              jobHandledFalse,
			}).
			SetSuspend(false).
			GetObject()
		if succeed {
			action.Status.Succeeded = 1
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []batchv1.Job{*action}
					return nil
				}).Times(1)
		}
		return action
	}
	mockDAG := func(stsOld, stsNew *apps.StatefulSet) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, transCtx.rsmOrig, transCtx.rsm, model.ActionStatusPtr())
		graphCli.Update(d, stsOld, stsNew)
		return d
	}
	expectStsNoopAction := func(d *graph.DAG, noop bool) {
		stsList := graphCli.FindAll(d, &apps.StatefulSet{})
		Expect(stsList).Should(HaveLen(1))
		sts, _ := stsList[0].(*apps.StatefulSet)
		Expect(graphCli.IsAction(d, sts, model.ActionNoopPtr())).Should(Equal(noop))
	}

	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			SetServiceName(headlessSvcName).
			AddMatchLabelsInMap(selectors).
			SetReplicas(3).
			SetRoles(roles).
			SetRoleProbe(roleProbe).
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

		dag = graph.NewDAG()
		graphCli.Root(dag, transCtx.rsmOrig, transCtx.rsm, model.ActionStatusPtr())
		transformer = &MemberReconfigurationTransformer{}
	})

	Context("roleful cluster initialization", func() {
		It("should initialize well", func() {
			By("initialReplicas=0")
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(rsm.Status.InitReplicas).Should(Equal(*rsm.Spec.Replicas))

			By("init one member")
			membersStatus := buildMembersStatus(1)
			rsm.Status.MembersStatus = membersStatus
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(rsm.Status.ReadyInitReplicas).Should(BeEquivalentTo(1))

			By("all members initialized")
			setRSMStatus(int(*rsm.Spec.Replicas))
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(rsm.Status.ReadyInitReplicas).Should(Equal(rsm.Status.InitReplicas))
		})
	})

	Context("stateful cluster initialization", func() {
		It("should work well", func() {
			By("set spec.roles to nil")
			rsm.Spec.Roles = nil
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			Expect(rsm.Status.InitReplicas).Should(BeEquivalentTo(0))
			Expect(rsm.Status.ReadyInitReplicas).Should(BeEquivalentTo(0))
		})
	})

	Context("scale-out", func() {
		It("should work well", func() {
			By("make rsm ready for scale-out")
			setRSMStatus(int(*rsm.Spec.Replicas))
			generation := int64(2)
			rsm.Generation = generation
			rsm.Status.ObservedGeneration = generation
			rsm.Status.CurrentGeneration = generation
			stsOld := mockUnderlyingSts(*rsm, rsm.Generation)
			// rsm spec updated
			rsm.Generation = 3
			replicas := int32(5)
			rsm.Spec.Replicas = &replicas
			sts := mockUnderlyingSts(*rsm, rsm.Generation)
			graphCli.Update(dag, stsOld, sts)

			By("update the underlying sts")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					*obj = *stsOld
					return nil
				}).Times(1)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, false)

			rsm.Status.ObservedGeneration = rsm.Generation
			rsm.Status.CurrentGeneration = rsm.Generation

			By("prepare member 3 joining")
			sts = mockUnderlyingSts(*rsm, rsm.Generation)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					*obj = *sts
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			dag = mockDAG(sts, sts)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, true)
			dagExpected := mockDAG(sts, sts)
			graphCli.Noop(dagExpected, sts)
			action := mockAction(3, jobTypeMemberJoinNotifying, false)
			graphCli.Create(dagExpected, action)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("make member 3 joining successfully and prepare member 4 joining")
			setRSMStatus(4)
			action = mockAction(3, jobTypeMemberJoinNotifying, true)
			dag = mockDAG(sts, sts)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					*obj = *sts
					return nil
				}).Times(1)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, true)
			dagExpected = mockDAG(sts, sts)
			graphCli.Noop(dagExpected, sts)
			graphCli.Update(dagExpected, action, action)
			action = mockAction(4, jobTypeMemberJoinNotifying, false)
			graphCli.Create(dagExpected, action)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("make member 4 joining successfully and cleanup")
			setRSMStatus(int(*rsm.Spec.Replicas))
			action = mockAction(4, jobTypeMemberJoinNotifying, true)
			dag = mockDAG(sts, sts)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, false)
			dagExpected = mockDAG(sts, sts)
			graphCli.Update(dagExpected, action, action)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})

	Context("scale-in", func() {
		It("should work well", func() {
			setRSMMembersStatus := func(replicas int) {
				membersStatus := buildMembersStatus(replicas)
				rsm.Status.InitReplicas = 3
				rsm.Status.ReadyInitReplicas = rsm.Status.InitReplicas
				rsm.Status.MembersStatus = membersStatus
				rsm.Status.UpdatedReplicas = int32(replicas)
			}
			By("make rsm ready for scale-in")
			setRSMStatus(int(*rsm.Spec.Replicas))
			generation := int64(2)
			rsm.Generation = generation
			rsm.Status.ObservedGeneration = generation
			rsm.Status.CurrentGeneration = generation
			stsOld := mockUnderlyingSts(*rsm, rsm.Generation)
			// rsm spec updated
			rsm.Generation = 3
			replicas := int32(1)
			rsm.Spec.Replicas = &replicas
			sts := mockUnderlyingSts(*rsm, rsm.Generation)
			graphCli.Update(dag, stsOld, sts)

			By("prepare member 2 leaving")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					*obj = *stsOld
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, true)
			dagExpected := mockDAG(stsOld, sts)
			graphCli.Noop(dagExpected, sts)
			action := mockAction(2, jobTypeMemberLeaveNotifying, false)
			graphCli.Create(dagExpected, action)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			// after the first reconciliation, observedGeneration should be updated
			rsm.Status.ObservedGeneration = rsm.Generation

			By("make member 2 leaving successfully and prepare member 1 switchover")
			setRSMMembersStatus(2)
			action = mockAction(2, jobTypeMemberLeaveNotifying, true)
			dag = mockDAG(stsOld, sts)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					*obj = *stsOld
					return nil
				}).Times(1)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, true)
			dagExpected = mockDAG(stsOld, sts)
			graphCli.Noop(dagExpected, sts)
			graphCli.Update(dagExpected, action, action)
			action = mockAction(1, jobTypeSwitchover, false)
			graphCli.Create(dagExpected, action)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("make member 1 switchover successfully and prepare member 1 leaving")
			membersStatus := []workloads.MemberStatus{
				{
					PodName:     getPodName(rsm.Name, 0),
					ReplicaRole: workloads.ReplicaRole{Name: "leader", IsLeader: true},
				},
				{
					PodName:     getPodName(rsm.Name, 1),
					ReplicaRole: workloads.ReplicaRole{Name: "follower"},
				},
			}
			rsm.Status.MembersStatus = membersStatus
			action = mockAction(1, jobTypeSwitchover, true)
			dag = mockDAG(stsOld, sts)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &apps.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *apps.StatefulSet, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					*obj = *stsOld
					return nil
				}).Times(1)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, true)
			dagExpected = mockDAG(stsOld, sts)
			graphCli.Noop(dagExpected, sts)
			graphCli.Update(dagExpected, action, action)
			action = mockAction(1, jobTypeMemberLeaveNotifying, false)
			graphCli.Create(dagExpected, action)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("make member 1 leaving successfully")
			setRSMMembersStatus(1)
			dag = mockDAG(stsOld, sts)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, false)
			dagExpected = mockDAG(stsOld, sts)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("update rsm status")
			rsm.Status.CurrentGeneration = rsm.Generation
			rsm.Status.Replicas = replicas
			rsm.Status.ReadyReplicas = replicas
			rsm.Status.AvailableReplicas = replicas
			rsm.Status.UpdatedReplicas = replicas
			action = mockAction(1, jobTypeMemberLeaveNotifying, true)
			dag = mockDAG(stsOld, sts)
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
			expectStsNoopAction(dag, false)
			dagExpected = mockDAG(stsOld, sts)
			graphCli.Update(dagExpected, action, action)
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())
		})
	})
})
