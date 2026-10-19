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
	"encoding/json"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

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

func TestReconcileInstanceTemplateImages(t *testing.T) {
	pair := instanceTemplateImageSelection{CompDef: "selected", ServiceVersion: "1.0", RuntimeDefaults: []string{"db", "prepare"}}
	fresh := map[string]string{"db": "new.example/team-b/db:1", "prepare": "new.example/team-b/prepare:1", "kbagent": "new-tools"}
	build := func() (*component.SynthesizedComponent, *workloads.InstanceSet, *workloads.InstanceSet) {
		synth := &component.SynthesizedComponent{Instances: []appsv1.InstanceTemplate{{Name: "canary", CompDef: pair.CompDef, ServiceVersion: pair.ServiceVersion}}, InstanceImageDefaults: map[string][]string{"canary": {"db", "prepare"}}}
		old := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{constant.AppComponentLabelKey: pair.CompDef, constant.KBAppServiceVersionKey: pair.ServiceVersion}}, Spec: workloads.InstanceSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "old.example/team-a/db:1"}, {Name: "kbagent", Image: "old-tools"}}, InitContainers: []corev1.Container{{Name: "prepare", Image: "old.example/team-a/prepare:1"}}}}, Instances: []workloads.InstanceTemplate{{Name: "canary"}}}}
		desired := old.DeepCopy()
		desired.Spec.Instances[0].Images = map[string]string{}
		for k, v := range fresh {
			desired.Spec.Instances[0].Images[k] = v
		}
		return synth, old, desired
	}
	mark := func(old *workloads.InstanceSet, selection instanceTemplateImageSelection) {
		data, err := json.Marshal(map[string]instanceTemplateImageSelection{"canary": selection})
		if err != nil {
			t.Fatal(err)
		}
		old.Annotations[instanceTemplateImageSelectionsAnnotation] = string(data)
	}
	for _, tc := range []struct {
		name     string
		setup    func(*component.SynthesizedComponent, *workloads.InstanceSet, *workloads.InstanceSet)
		db, init string
	}{
		{name: "legacy nil map adopts parent", db: "old.example/team-a/db:1", init: "old.example/team-a/prepare:1"},
		{name: "legacy action-only map adopts parent", setup: func(_ *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			o.Spec.Instances[0].Images = map[string]string{"kbagent": "old-tools"}
		}, db: "old.example/team-a/db:1", init: "old.example/team-a/prepare:1"},
		{name: "legacy distinct selected definition is corrected", setup: func(s *component.SynthesizedComponent, _, _ *workloads.InstanceSet) {
			s.Instances[0].CompDef = "different"
		}},
		{name: "legacy existing CV override stays fresh", setup: func(_ *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			o.Spec.Instances[0].Images = map[string]string{"db": "old-override", "prepare": "old-prepare"}
		}},
		{name: "known same selection freezes exact refs", setup: func(_ *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			mark(o, pair)
			o.Spec.Instances[0].Images = map[string]string{"db": "old.example/custom/db:1", "prepare": "old.example/custom/prepare:1"}
		}, db: "old.example/custom/db:1", init: "old.example/custom/prepare:1"},
		{name: "independent parent upgrade retains selection", setup: func(_ *component.SynthesizedComponent, o, d *workloads.InstanceSet) {
			mark(o, pair)
			d.Annotations[constant.AppComponentLabelKey] = "new-parent"
		}, db: "old.example/team-a/db:1", init: "old.example/team-a/prepare:1"},
		{name: "explicit definition change keeps new namespace with same tag", setup: func(s *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			mark(o, pair)
			s.Instances[0].CompDef = "new-definition"
		}},
		{name: "explicit service version change", setup: func(s *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			mark(o, pair)
			s.Instances[0].ServiceVersion = "2.0"
		}},
		{name: "new template uses current registry", setup: func(_ *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			mark(o, pair)
			o.Spec.Instances = nil
		}},
		{name: "override becomes default uses current image", setup: func(_ *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			p := pair
			p.RuntimeDefaults = []string{"prepare"}
			mark(o, p)
			o.Spec.Instances[0].Images = map[string]string{"db": "old-override"}
		}, init: "old.example/team-a/prepare:1"},
		{name: "default becomes override uses current image", setup: func(s *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			mark(o, pair)
			s.InstanceImageDefaults["canary"] = []string{"prepare"}
		}, init: "old.example/team-a/prepare:1"},
		{name: "no continuously eligible entries stays fresh", setup: func(s *component.SynthesizedComponent, o, _ *workloads.InstanceSet) {
			mark(o, pair)
			s.InstanceImageDefaults["canary"] = nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			s, o, d := build()
			if tc.setup != nil {
				tc.setup(s, o, d)
			}
			before := o.DeepCopy()
			g.Expect(reconcileInstanceTemplateImages(s, o, d)).Should(Succeed())
			db, init := tc.db, tc.init
			if db == "" {
				db = fresh["db"]
			}
			if init == "" {
				init = fresh["prepare"]
			}
			g.Expect(d.Spec.Instances[0].Images).Should(Equal(map[string]string{"db": db, "prepare": init, "kbagent": "new-tools"}))
			g.Expect(o).Should(Equal(before))
			var state map[string]instanceTemplateImageSelection
			g.Expect(json.Unmarshal([]byte(d.Annotations[instanceTemplateImageSelectionsAnnotation]), &state)).Should(Succeed())
			g.Expect(state["canary"].CompDef).Should(Equal(s.Instances[0].CompDef))
			g.Expect(state["canary"].ServiceVersion).Should(Equal(s.Instances[0].ServiceVersion))
			g.Expect(state["canary"].RuntimeDefaults).Should(ConsistOf(s.InstanceImageDefaults["canary"]))
			_, _, retry := build()
			g.Expect(reconcileInstanceTemplateImages(s, o, retry)).Should(Succeed())
			g.Expect(retry.Spec.Instances).Should(Equal(d.Spec.Instances))
			_, _, restarted := build()
			g.Expect(reconcileInstanceTemplateImages(s, d, restarted)).Should(Succeed())
			g.Expect(restarted.Spec.Instances).Should(Equal(d.Spec.Instances))
		})
	}
	t.Run("clear remove and reselect", func(t *testing.T) {
		g := NewWithT(t)
		s, o, d := build()
		mark(o, pair)
		s.Instances[0].CompDef = ""
		s.Instances[0].ServiceVersion = ""
		d.Spec.Instances[0].Images = nil
		g.Expect(reconcileInstanceTemplateImages(s, o, d)).Should(Succeed())
		g.Expect(d.Annotations[instanceTemplateImageSelectionsAnnotation]).Should(Equal("{}"))
		merged, err := copyAndMergeITS(o, d)
		g.Expect(err).Should(Succeed())
		g.Expect(merged.Annotations[instanceTemplateImageSelectionsAnnotation]).Should(Equal("{}"))
		g.Expect(merged.Spec.Instances[0].Images).Should(BeNil())
		s, _, d = build()
		g.Expect(reconcileInstanceTemplateImages(s, merged, d)).Should(Succeed())
		g.Expect(d.Spec.Instances[0].Images).Should(Equal(fresh))
		s.Instances = nil
		d.Spec.Instances = nil
		g.Expect(reconcileInstanceTemplateImages(s, o, d)).Should(Succeed())
		g.Expect(d.Annotations[instanceTemplateImageSelectionsAnnotation]).Should(Equal("{}"))
	})
	t.Run("parent only leaves metadata untouched", func(t *testing.T) {
		g := NewWithT(t)
		s, _, d := build()
		s.Instances = nil
		d.Spec.Instances = nil
		g.Expect(reconcileInstanceTemplateImages(s, nil, d)).Should(Succeed())
		g.Expect(d.Annotations).ShouldNot(HaveKey(instanceTemplateImageSelectionsAnnotation))
	})
	t.Run("malformed owned state blocks publication", func(t *testing.T) {
		g := NewWithT(t)
		s, o, d := build()
		o.Annotations[instanceTemplateImageSelectionsAnnotation] = "invalid"
		g.Expect(reconcileInstanceTemplateImages(s, o, d)).ShouldNot(Succeed())
	})
	t.Run("stop and start retain images with their identity", func(t *testing.T) {
		g := NewWithT(t)
		s, o, d := build()
		mark(o, pair)
		s.Stop = ptr.To(true)
		s.Instances[0].CompDef = "next"
		g.Expect(reconcileInstanceTemplateImages(s, o, d)).Should(Succeed())
		transCtx := &componentTransformContext{}
		transformer := &componentWorkloadTransformer{}
		transformer.handleWorkloadStartNStop(transCtx, s, o, &d)
		g.Expect(d.Annotations[instanceTemplateImageSelectionsAnnotation]).Should(Equal(o.Annotations[instanceTemplateImageSelectionsAnnotation]))
		g.Expect(d.Spec.Instances).Should(Equal(o.Spec.Instances))
		o.Spec.Stop = ptr.To(true)
		s.Stop = nil
		transformer.handleWorkloadStartNStop(transCtx, s, o, &d)
		g.Expect(d.Annotations[instanceTemplateImageSelectionsAnnotation]).Should(Equal(o.Annotations[instanceTemplateImageSelectionsAnnotation]))
		g.Expect(d.Spec.Instances).Should(Equal(o.Spec.Instances))
	})
}
