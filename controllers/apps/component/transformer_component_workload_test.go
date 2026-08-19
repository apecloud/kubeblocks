/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd
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

package component

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component Workload Operations Test", func() {
	const (
		clusterName    = "test-cluster"
		compName       = "test-comp"
		kubeblocksName = "kubeblocks"
	)

	var (
		reader         *appsutil.MockReader
		graphCli       model.GraphClient
		dag            *graph.DAG
		comp           *appsv1.Component
		synthesizeComp *component.SynthesizedComponent
	)

	roles := []appsv1.ReplicaRole{
		{Name: "leader", UpdatePriority: 3},
		{Name: "follower", UpdatePriority: 2},
	}

	newDAG := func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, comp, comp, model.ActionStatusPtr())
		return d
	}

	BeforeEach(func() {
		reader = &appsutil.MockReader{}
		comp = &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: appsv1.ComponentSpec{},
		}

		synthesizeComp = &component.SynthesizedComponent{
			Namespace:   testCtx.DefaultNamespace,
			ClusterName: clusterName,
			Name:        compName,
			Roles:       roles,
			LifecycleActions: component.SynthesizedLifecycleActions{
				ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{
					MemberJoin: &appsv1.Action{
						Exec: &appsv1.ExecAction{
							Image: "test-image",
						},
					},
					MemberLeave: &appsv1.Action{
						Exec: &appsv1.ExecAction{
							Image: "test-image",
						},
					},
					Switchover: &appsv1.Action{
						Exec: &appsv1.ExecAction{
							Image: "test-image",
						},
					},
				},
			},
		}

		graphCli = model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)
	})

	Context("Member Leave Operations", func() {
		var (
			ops  *componentWorkloadOps
			pod0 *corev1.Pod
			pod1 *corev1.Pod
			pods []*corev1.Pod
		)

		BeforeEach(func() {
			pod0 = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod-0").
				AddContainer(corev1.Container{
					Image: "test-image",
					Name:  "test-container",
				}).
				AddLabels(
					constant.AppManagedByLabelKey, kubeblocksName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, compName,
				).
				GetObject()

			pod1 = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod-1").
				AddContainer(corev1.Container{
					Image: "test-image",
					Name:  "test-container",
				}).
				AddLabels(
					constant.AppManagedByLabelKey, kubeblocksName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, compName,
				).
				GetObject()

			pods = []*corev1.Pod{pod0, pod1}

			container := corev1.Container{
				Name:            "mock-container-name",
				Image:           testapps.ApeCloudMySQLImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}

			mockITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"test-its", clusterName, compName).
				AddFinalizers([]string{constant.DBClusterFinalizerName}).
				AddContainer(container).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(compName).
				AddAppManagedByLabel().
				SetReplicas(2).
				SetRoles(roles).
				GetObject()

			ops = &componentWorkloadOps{
				transCtx: &componentTransformContext{
					Context:       ctx,
					Client:        graphCli,
					Logger:        logger,
					EventRecorder: clusterRecorder,
				},
				cli:            k8sClient,
				component:      comp,
				synthesizeComp: synthesizeComp,
				runningITS:     mockITS,
				protoITS:       mockITS.DeepCopy(),
				dag:            dag,
			}
		})

		It("should handle switchover for when scale in", func() {
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					GinkgoWriter.Printf("ActionRequest: %#v\n", req)
					switch req.Action {
					case "switchover":
						Expect(req.Parameters["KB_SWITCHOVER_CURRENT_NAME"]).Should(Equal(pod1.Name))
					case "memberLeave":
						Expect(req.Parameters["KB_LEAVE_MEMBER_POD_NAME"]).Should(Equal(pod1.Name))
					}
					rsp := kbagentproto.ActionResponse{Message: "mock success"}
					return rsp, nil
				})
			})

			By("setting up leader pod")
			pod1.Labels[constant.RoleLabelKey] = "follower"
			pod1.Labels[constant.RoleLabelKey] = "leader"

			By("executing leave member for leader")
			Expect(ops.leaveMemberForPod(pod1, pods)).Should(Succeed())
		})

		It("should emit events when member leave fails", func() {
			testapps.MockKBAgentClient(func(mockRecorder *kbacli.MockClientMockRecorder) {
				mockRecorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if req.Action == "memberLeave" {
						return kbagentproto.ActionResponse{
							Error:   kbagentproto.Error2Type(kbagentproto.ErrTimedOut),
							Message: "member leave timed out",
						}, nil
					}
					return kbagentproto.ActionResponse{}, nil
				}).Times(2)
			})

			eventRecorder := &capturingEventRecorder{}
			ops.transCtx.Component = comp
			ops.transCtx.EventRecorder = eventRecorder

			Expect(ops.leaveMemberForPod(pod1, pods)).ShouldNot(Succeed())
			Expect(eventRecorder.events).Should(HaveLen(1))
			for _, event := range eventRecorder.events {
				Expect(event.reason).Should(Equal(memberLeaveFailedEventReason))
				Expect(event.message).Should(ContainSubstring("pod " + pod1.Name))
			}
		})

		It("should emit events when member join fails", func() {
			testapps.MockKBAgentClient(func(mockRecorder *kbacli.MockClientMockRecorder) {
				mockRecorder.Action(gomock.Any(), gomock.Any()).Return(kbagentproto.ActionResponse{
					Error:   kbagentproto.Error2Type(kbagentproto.ErrFailed),
					Message: "member join failed",
				}, nil).Times(1)
			})

			eventRecorder := &capturingEventRecorder{}
			ops.transCtx.Component = comp
			ops.transCtx.EventRecorder = eventRecorder

			Expect(ops.joinMemberForPod(pod1, pods)).ShouldNot(Succeed())
			Expect(eventRecorder.events).Should(HaveLen(1))
			for _, event := range eventRecorder.events {
				Expect(event.reason).Should(Equal(memberJoinFailedEventReason))
				Expect(event.message).Should(ContainSubstring("pod " + pod1.Name))
			}
		})

		It("should emit a member action failure only when its fingerprint changes", func() {
			eventRecorder := &capturingEventRecorder{}
			ops.transCtx.Component = comp
			ops.transCtx.EventRecorder = eventRecorder
			actionErr := errors.New("member join failed")

			ops.reportMemberActionFailure(pod1, memberJoinFailureFingerprintAnnotationKey,
				memberJoinFailedEventReason, "memberJoin", actionErr)
			Expect(eventRecorder.events).Should(HaveLen(1))

			vertex := graphCli.FindMatchedVertex(dag, pod1)
			Expect(vertex).ShouldNot(BeNil())
			patchedPod := vertex.(*model.ObjectVertex).Obj.(*corev1.Pod)
			Expect(patchedPod.Annotations[memberJoinFailureFingerprintAnnotationKey]).ShouldNot(BeEmpty())

			ops.dag = newDAG(graphCli, comp)
			ops.reportMemberActionFailure(patchedPod, memberJoinFailureFingerprintAnnotationKey,
				memberJoinFailedEventReason, "memberJoin", actionErr)
			Expect(eventRecorder.events).Should(HaveLen(1))

			ops.reportMemberActionFailure(patchedPod, memberJoinFailureFingerprintAnnotationKey,
				memberJoinFailedEventReason, "memberJoin", errors.New("member join timed out"))
			Expect(eventRecorder.events).Should(HaveLen(2))
			changedVertex := graphCli.FindMatchedVertex(ops.dag, patchedPod)
			changedPod := changedVertex.(*model.ObjectVertex).Obj.(*corev1.Pod)

			ops.dag = newDAG(graphCli, comp)
			ops.clearMemberActionFailureFingerprint(changedPod, memberJoinFailureFingerprintAnnotationKey)
			clearedVertex := graphCli.FindMatchedVertex(ops.dag, changedPod)
			clearedPod := clearedVertex.(*model.ObjectVertex).Obj.(*corev1.Pod)
			Expect(clearedPod.Annotations).ShouldNot(HaveKey(memberJoinFailureFingerprintAnnotationKey))
		})
	})
})
