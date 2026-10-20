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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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
	Context("copyAndMergeITS restart annotations", func() {
		const older = "2026-09-01T00:00:00Z"
		const newer = "2026-09-02T00:00:00Z"

		DescribeTable("merge restart timestamps", func(runningRestart, desiredRestart, expectedRestart string) {
			running := &workloads.InstanceSet{Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"custom": "old", "retained": "value"},
				}},
			}}
			desired := running.DeepCopy()
			desired.Spec.Template.Annotations = map[string]string{"custom": "new"}
			if runningRestart != "" {
				running.Spec.Template.Annotations[constant.RestartAnnotationKey] = runningRestart
			}
			if desiredRestart != "" {
				desired.Spec.Template.Annotations[constant.RestartAnnotationKey] = desiredRestart
			}
			before := running.DeepCopy()
			merged, err := copyAndMergeITS(running, desired)
			Expect(err).Should(Succeed())
			Expect(merged).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Annotations).Should(Equal(map[string]string{
				"custom": "new", "retained": "value", constant.RestartAnnotationKey: expectedRestart,
			}))
			Expect(running).Should(Equal(before))
			mergedAgain, err := copyAndMergeITS(merged, desired)
			Expect(err).Should(Succeed())
			Expect(mergedAgain).Should(BeNil())
		},
			Entry("preserve config restart against older ops", newer, older, newer),
			Entry("accept newer ops restart", older, newer, newer),
			Entry("equal timestamp", newer, newer, newer),
			Entry("compare timezone offsets", "2026-09-02T01:00:00+08:00", "2026-09-01T18:00:00Z", "2026-09-01T18:00:00Z"),
			Entry("preserve equal instant representation", newer, "2026-09-02T08:00:00+08:00", newer),
			Entry("compare fractional seconds", "2026-09-02T00:00:00.5Z", newer, "2026-09-02T00:00:00.5Z"),
			Entry("first restart", "", newer, newer),
			Entry("no incoming restart", newer, "", newer),
			Entry("replace invalid running value", "invalid", newer, newer),
			Entry("preserve opaque incoming value behavior", newer, "force-restart", "force-restart"),
		)
	})

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

	Context("Start generation", func() {
		It("publishes the desired generation only after the deferred workload update", func() {
			comp.Generation = 2
			comp.Status.Phase = appsv1.StartingComponentPhase
			compDef := &appsv1.ComponentDefinition{}
			synthesizeComp.CompDefName = "db"
			synthesizeComp.ServiceVersion = "1.0.0"
			synthesizeComp.Generation = "1"
			synthesizeComp.Replicas = 1
			synthesizeComp.PodSpec = &corev1.PodSpec{
				Containers: []corev1.Container{{Name: "db", Image: "db:1"}},
			}
			synthesizeComp.LifecycleActions.ComponentLifecycleActions = &appsv1.ComponentLifecycleActions{}
			synthesizeComp.Instances = []appsv1.InstanceTemplate{{Name: "canary", Replicas: ptr.To(int32(1)), ServiceVersion: "1.0.0"}}
			synthesizeComp.InstanceImages = map[string]map[string]string{"canary": {"db": "db:1"}}
			stopped, err := component.BuildInstanceSet(synthesizeComp, compDef)
			Expect(err).ShouldNot(HaveOccurred())
			stopped.Spec.Stop = ptr.To(true)
			cli := fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(stopped).Build()

			synthesizeComp.Generation = "2"
			synthesizeComp.Instances[0].ServiceVersion = "2.0.0"
			synthesizeComp.InstanceImages["canary"]["db"] = "db:2"
			publish := func() *workloads.InstanceSet {
				current := &workloads.InstanceSet{}
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(stopped), current)).Should(Succeed())
				graphCli := model.NewGraphClient(cli)
				dag := newDAG(graphCli, comp)
				transCtx := &componentTransformContext{
					Context:             ctx,
					Client:              graphCli,
					Logger:              logger,
					EventRecorder:       clusterRecorder,
					Component:           comp,
					ComponentOrig:       comp.DeepCopy(),
					CompDef:             compDef,
					SynthesizeComponent: synthesizeComp,
					RunningWorkload:     current,
				}
				Expect((&componentWorkloadTransformer{Client: cli}).Transform(transCtx, dag)).Should(Succeed())
				Expect(graphCli.IsAction(dag, current, model.ActionUpdatePtr())).Should(BeTrue())
				updated := graphCli.FindMatchedVertex(dag, current).(*model.ObjectVertex).Obj.(*workloads.InstanceSet)
				Expect(cli.Update(ctx, updated)).Should(Succeed())
				persisted := &workloads.InstanceSet{}
				Expect(cli.Get(ctx, client.ObjectKeyFromObject(current), persisted)).Should(Succeed())
				return persisted
			}

			started := publish()
			Expect(started.Spec.Stop).Should(BeNil())
			Expect(started.Spec.Instances[0].Images["db"]).Should(Equal("db:1"))
			Expect(started.Annotations[constant.KubeBlocksGenerationKey]).Should(Equal("1"))

			updated := publish()
			Expect(comp.Status.Phase).Should(Equal(appsv1.StartingComponentPhase))
			Expect(updated.Spec.Stop).Should(BeNil())
			Expect(updated.Spec.Instances[0].Images["db"]).Should(Equal("db:2"))
			Expect(updated.Annotations[constant.KubeBlocksGenerationKey]).Should(Equal("2"))
		})
	})

	Context("Data Replication Operations", func() {
		It("blocks scale-out when data actions have no source pod", func() {
			synthesizeComp.Replicas = 1
			synthesizeComp.LifecycleActions.DataDump = testapps.NewLifecycleAction("data-dump")
			synthesizeComp.LifecycleActions.DataLoad = testapps.NewLifecycleAction("data-load")

			runningITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"test-its", clusterName, compName).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(compName).
				AddAppManagedByLabel().
				SetReplicas(0).
				GetObject()
			protoITS := runningITS.DeepCopy()
			protoITS.Spec.Replicas = ptr.To(int32(1))

			transCtx := &componentTransformContext{
				Context:             ctx,
				Client:              graphCli,
				Logger:              logger,
				EventRecorder:       clusterRecorder,
				Component:           comp,
				SynthesizeComponent: synthesizeComp,
			}
			ops, err := newComponentWorkloadOps(transCtx, k8sClient, synthesizeComp,
				comp, runningITS, protoITS, dag)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(ops.scaleOut()).Should(MatchError("no available pod to dump data"))
		})
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
