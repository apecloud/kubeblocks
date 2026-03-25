/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("replicas alignment reconciler test", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetReplicas(3).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetRoles(roles).
			GetObject()
	})

	Context("PreCondition & Reconcile", func() {
		makePodAvailable := func(pod *corev1.Pod) {
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Second)),
				},
			}
		}

		It("should work well", func() {
			By("PreCondition")
			its.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			reconciler = NewReplicasAlignmentReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))

			By("prepare current tree")
			// desired: bar-0, bar-1, bar-2, bar-3, bar-foo-0, bar-foo-1, bar-hello-0
			// current: bar-1, bar-foo-0
			replicas := int32(7)
			its.Spec.Replicas = &replicas
			nameHello := "hello"
			instanceHello := workloads.InstanceTemplate{
				Name: nameHello,
			}
			its.Spec.Instances = append(its.Spec.Instances, instanceHello)
			nameFoo := "foo"
			replicasFoo := int32(2)
			instanceFoo := workloads.InstanceTemplate{
				Name:     nameFoo,
				Replicas: &replicasFoo,
			}
			its.Spec.Instances = append(its.Spec.Instances, instanceFoo)
			podFoo0 := builder.NewPodBuilder(namespace, its.Name+"-foo-0").GetObject()
			pvcFoo0 := builder.NewPVCBuilder(namespace, volumeClaimTemplates[0].Name+"-"+podFoo0.Name).GetObject()
			podBar1 := builder.NewPodBuilder(namespace, "bar-1").GetObject()
			pvcBar1 := builder.NewPVCBuilder(namespace, volumeClaimTemplates[0].Name+"-"+podBar1.Name).GetObject()
			Expect(tree.Add(podFoo0, pvcFoo0, podBar1, pvcBar1)).Should(Succeed())

			By("update revisions")
			revisionUpdateReconciler := NewRevisionUpdateReconciler()
			_, err := revisionUpdateReconciler.Reconcile(tree)
			Expect(err).Should(BeNil())

			By("do reconcile with OrderedReady(Serial) policy")
			orderedReadyTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			res, err := reconciler.Reconcile(orderedReadyTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			// desired: bar-0, bar-1, bar-foo-0
			pods := orderedReadyTree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(3))
			pvcs := orderedReadyTree.List(&corev1.PersistentVolumeClaim{})
			Expect(pvcs).Should(HaveLen(3))
			podBar0 := builder.NewPodBuilder(namespace, "bar-0").GetObject()
			for _, object := range []client.Object{podFoo0, podBar0, podBar1} {
				Expect(slices.IndexFunc(pods, func(item client.Object) bool {
					return item.GetName() == object.GetName()
				})).Should(BeNumerically(">=", 0))
				Expect(slices.IndexFunc(pvcs, func(item client.Object) bool {
					expectedPVCName := fmt.Sprintf("%s-%s", volumeClaimTemplates[0].Name, object.GetName())
					return expectedPVCName == item.GetName()
				})).Should(BeNumerically(">=", 0))
			}

			By("do reconcile with Parallel policy")
			parallelTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			parallelITS, ok := parallelTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			parallelITS.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			res, err = reconciler.Reconcile(parallelTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			// desired: bar-0, bar-1, bar-2, bar-3, bar-foo-0, bar-foo-1, bar-hello-0
			pods = parallelTree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(7))
			pvcs = parallelTree.List(&corev1.PersistentVolumeClaim{})
			Expect(pvcs).Should(HaveLen(7))
			podHello := builder.NewPodBuilder(namespace, its.Name+"-hello-0").GetObject()
			podFoo1 := builder.NewPodBuilder(namespace, its.Name+"-foo-1").GetObject()
			podBar2 := builder.NewPodBuilder(namespace, "bar-2").GetObject()
			podBar3 := builder.NewPodBuilder(namespace, "bar-3").GetObject()
			for _, object := range []client.Object{podHello, podFoo0, podFoo1, podBar0, podBar1, podBar2, podBar3} {
				Expect(slices.IndexFunc(pods, func(item client.Object) bool {
					return item.GetName() == object.GetName()
				})).Should(BeNumerically(">=", 0))
				Expect(slices.IndexFunc(pvcs, func(item client.Object) bool {
					expectedPVCName := fmt.Sprintf("%s-%s", volumeClaimTemplates[0].Name, object.GetName())
					return expectedPVCName == item.GetName()
				})).Should(BeNumerically(">=", 0))
			}

			By("do reconcile with Parallel policy, ParallelPodManagementConcurrency is 50%")
			parallelTree, err = tree.DeepCopy()
			Expect(err).Should(BeNil())
			parallelITS, ok = parallelTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			parallelITS.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			parallelITS.Spec.ParallelPodManagementConcurrency = &intstr.IntOrString{Type: intstr.String, StrVal: "50%"}
			res, err = reconciler.Reconcile(parallelTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			// replicas is 7, ParallelPodManagementConcurrency is 50%, so concurrency is 4.
			// since the original bar-1 and bar-foo-0 are not ready, only the new instances bar-0 and bar-2 will be added.
			// desired: bar-0, bar-1, bar-2, bar-foo-0
			pods = parallelTree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(4))
			pvcs = parallelTree.List(&corev1.PersistentVolumeClaim{})
			Expect(pvcs).Should(HaveLen(4))
			for _, object := range []client.Object{podFoo0, podBar0, podBar1, podBar2} {
				Expect(slices.IndexFunc(pods, func(item client.Object) bool {
					return item.GetName() == object.GetName()
				})).Should(BeNumerically(">=", 0))
				Expect(slices.IndexFunc(pvcs, func(item client.Object) bool {
					expectedPVCName := fmt.Sprintf("%s-%s", volumeClaimTemplates[0].Name, object.GetName())
					return expectedPVCName == item.GetName()
				})).Should(BeNumerically(">=", 0))
			}
		})

		It("handles nodeSelectorOnce Annotation", func() {
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			name := "bar-1"
			node := "test-1"
			Expect(MergeNodeSelectorOnceAnnotation(its, map[string]string{name: node})).To(Succeed())

			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			pods := tree.List(&corev1.Pod{})
			for _, obj := range pods {
				pod := obj.(*corev1.Pod)
				if pod.Name == name {
					Expect(pod.Spec.NodeSelector).To(Equal(map[string]string{
						corev1.LabelHostname: node,
					}))
				}
			}
		})

		It("serially advances scale-out lifecycle one step per reconcile", func() {
			var actions []kbagentproto.ActionRequest
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actions = append(actions, req)
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			its.Spec.LifecycleActions = &workloads.LifecycleActions{
				MemberJoin: testapps.NewLifecycleAction("member-join"),
				DataLoad:   testapps.NewLifecycleAction("data-load"),
			}
			its.Spec.MemberUpdateStrategy = ptr.To(workloads.SerialUpdateStrategy)
			its.Status.InstanceStatus = []workloads.InstanceStatus{
				{PodName: its.Name + "-0", Provisioned: true, MemberJoined: boolPtr(true)},
				{PodName: its.Name + "-1", Provisioned: true, DataLoaded: boolPtr(false), MemberJoined: boolPtr(false)},
			}

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			pod0 := builder.NewPodBuilder(namespace, its.Name+"-0").GetObject()
			pod1 := builder.NewPodBuilder(namespace, its.Name+"-1").GetObject()
			makePodAvailable(pod0)
			makePodAvailable(pod1)
			Expect(tree.Add(pod0, pod1)).Should(Succeed())

			r := &instanceAlignmentReconciler{}
			retry, err := r.reconcileScaleOutLifecycle(tree, its)
			Expect(err).Should(BeNil())
			Expect(retry).Should(BeTrue())
			Expect(actions).Should(HaveLen(1))
			Expect(actions[0].Action).Should(Equal("dataLoad"))
			Expect(actions[0].Parameters["KB_TARGET_POD_NAME"]).Should(Equal(pod1.Name))
			Expect(*findInstanceStatus(its, pod1.Name).DataLoaded).Should(BeTrue())
			Expect(*findInstanceStatus(its, pod1.Name).MemberJoined).Should(BeFalse())

			retry, err = r.reconcileScaleOutLifecycle(tree, its)
			Expect(err).Should(BeNil())
			Expect(retry).Should(BeTrue())
			Expect(actions).Should(HaveLen(2))
			Expect(actions[1].Action).Should(Equal("memberJoin"))
			Expect(actions[1].Parameters["KB_JOIN_MEMBER_POD_NAME"]).Should(Equal(pod1.Name))
			Expect(*findInstanceStatus(its, pod1.Name).MemberJoined).Should(BeTrue())
		})

		It("parallel lifecycle advances all pending scale-out replicas in one reconcile", func() {
			var actions []kbagentproto.ActionRequest
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actions = append(actions, req)
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			its.Spec.LifecycleActions = &workloads.LifecycleActions{
				MemberJoin: testapps.NewLifecycleAction("member-join"),
				DataLoad:   testapps.NewLifecycleAction("data-load"),
			}
			its.Spec.MemberUpdateStrategy = ptr.To(workloads.ParallelUpdateStrategy)
			its.Status.InstanceStatus = []workloads.InstanceStatus{
				{PodName: its.Name + "-0", Provisioned: true, MemberJoined: boolPtr(true)},
				{PodName: its.Name + "-1", Provisioned: true, DataLoaded: boolPtr(false), MemberJoined: boolPtr(false)},
				{PodName: its.Name + "-2", Provisioned: true, DataLoaded: boolPtr(false), MemberJoined: boolPtr(false)},
			}

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			for i := 0; i < 3; i++ {
				pod := builder.NewPodBuilder(namespace, fmt.Sprintf("%s-%d", its.Name, i)).GetObject()
				makePodAvailable(pod)
				Expect(tree.Add(pod)).Should(Succeed())
			}

			r := &instanceAlignmentReconciler{}
			retry, err := r.reconcileScaleOutLifecycle(tree, its)
			Expect(err).Should(BeNil())
			Expect(retry).Should(BeFalse())
			Expect(actions).Should(HaveLen(4))
			Expect(actions[0].Action).Should(Equal("dataLoad"))
			Expect(actions[1].Action).Should(Equal("memberJoin"))
			Expect(actions[2].Action).Should(Equal("dataLoad"))
			Expect(actions[3].Action).Should(Equal("memberJoin"))
			for _, podName := range []string{its.Name + "-1", its.Name + "-2"} {
				status := findInstanceStatus(its, podName)
				Expect(*status.DataLoaded).Should(BeTrue())
				Expect(*status.MemberJoined).Should(BeTrue())
			}
		})

		It("serially scales in one joined replica per reconcile", func() {
			var leaveNames []string
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if req.Action == "memberLeave" {
						leaveNames = append(leaveNames, req.Parameters["KB_LEAVE_MEMBER_POD_NAME"])
					}
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			replicas := int32(1)
			its.Spec.Replicas = &replicas
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			its.Spec.LifecycleActions = &workloads.LifecycleActions{
				MemberLeave: testapps.NewLifecycleAction("member-leave"),
			}
			its.Spec.MemberUpdateStrategy = ptr.To(workloads.SerialUpdateStrategy)
			its.Status.InstanceStatus = []workloads.InstanceStatus{
				{PodName: its.Name + "-0", Provisioned: true, MemberJoined: boolPtr(true)},
				{PodName: its.Name + "-1", Provisioned: true, MemberJoined: boolPtr(true)},
				{PodName: its.Name + "-2", Provisioned: true, MemberJoined: boolPtr(true)},
			}

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			for i := 0; i < 3; i++ {
				pod := builder.NewPodBuilder(namespace, fmt.Sprintf("%s-%d", its.Name, i)).GetObject()
				makePodAvailable(pod)
				Expect(tree.Add(pod)).Should(Succeed())
			}

			reconciler = NewReplicasAlignmentReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			Expect(leaveNames).Should(HaveLen(1))
			Expect(tree.List(&corev1.Pod{})).Should(HaveLen(2))
		})

		It("best-effort parallel lifecycle advances all pending scale-out replicas in one reconcile", func() {
			var actions []kbagentproto.ActionRequest
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actions = append(actions, req)
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			its.Spec.LifecycleActions = &workloads.LifecycleActions{
				MemberJoin: testapps.NewLifecycleAction("member-join"),
				DataLoad:   testapps.NewLifecycleAction("data-load"),
			}
			its.Spec.MemberUpdateStrategy = ptr.To(workloads.BestEffortParallelUpdateStrategy)
			its.Status.InstanceStatus = []workloads.InstanceStatus{
				{PodName: its.Name + "-0", Provisioned: true, MemberJoined: boolPtr(true)},
				{PodName: its.Name + "-1", Provisioned: true, DataLoaded: boolPtr(false), MemberJoined: boolPtr(false)},
				{PodName: its.Name + "-2", Provisioned: true, DataLoaded: boolPtr(false), MemberJoined: boolPtr(false)},
			}

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			for i := 0; i < 3; i++ {
				pod := builder.NewPodBuilder(namespace, fmt.Sprintf("%s-%d", its.Name, i)).GetObject()
				makePodAvailable(pod)
				Expect(tree.Add(pod)).Should(Succeed())
			}

			r := &instanceAlignmentReconciler{}
			retry, err := r.reconcileScaleOutLifecycle(tree, its)
			Expect(err).Should(BeNil())
			Expect(retry).Should(BeFalse())
			Expect(actions).Should(HaveLen(4))
			Expect(actions[0].Action).Should(Equal("dataLoad"))
			Expect(actions[1].Action).Should(Equal("memberJoin"))
			Expect(actions[2].Action).Should(Equal("dataLoad"))
			Expect(actions[3].Action).Should(Equal("memberJoin"))
			for _, podName := range []string{its.Name + "-1", its.Name + "-2"} {
				status := findInstanceStatus(its, podName)
				Expect(*status.DataLoaded).Should(BeTrue())
				Expect(*status.MemberJoined).Should(BeTrue())
			}
		})

		It("best-effort parallel scale-in processes the first role-safe batch in one reconcile", func() {
			var leaveNames []string
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if req.Action == "memberLeave" {
						leaveNames = append(leaveNames, req.Parameters["KB_LEAVE_MEMBER_POD_NAME"])
					}
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			replicas := int32(1)
			its.Spec.Replicas = &replicas
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			its.Spec.LifecycleActions = &workloads.LifecycleActions{
				MemberLeave: testapps.NewLifecycleAction("member-leave"),
			}
			its.Spec.MemberUpdateStrategy = ptr.To(workloads.BestEffortParallelUpdateStrategy)

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			roleNames := []string{"follower", "logger", "", "learner", "candidate", "leader", "learner"}
			for i, roleName := range roleNames {
				pod := builder.NewPodBuilder(namespace, fmt.Sprintf("%s-%d", its.Name, i)).GetObject()
				if len(roleName) > 0 {
					pod.Labels = map[string]string{RoleLabelKey: roleName}
				}
				makePodAvailable(pod)
				Expect(tree.Add(pod)).Should(Succeed())
			}
			its.Status.InstanceStatus = []workloads.InstanceStatus{
				{PodName: its.Name + "-0", Provisioned: true, MemberJoined: boolPtr(true), Role: "follower"},
				{PodName: its.Name + "-1", Provisioned: true, MemberJoined: boolPtr(true), Role: "logger"},
				{PodName: its.Name + "-2", Provisioned: true, MemberJoined: boolPtr(true)},
				{PodName: its.Name + "-3", Provisioned: true, MemberJoined: boolPtr(true), Role: "learner"},
				{PodName: its.Name + "-4", Provisioned: true, MemberJoined: boolPtr(true), Role: "candidate"},
				{PodName: its.Name + "-5", Provisioned: true, MemberJoined: boolPtr(true), Role: "leader"},
				{PodName: its.Name + "-6", Provisioned: true, MemberJoined: boolPtr(true), Role: "learner"},
			}

			reconciler = NewReplicasAlignmentReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			Expect(leaveNames).Should(ConsistOf(its.Name+"-1", its.Name+"-2", its.Name+"-3", its.Name+"-4", its.Name+"-6"))
			Expect(tree.List(&corev1.Pod{})).Should(HaveLen(2))
		})
	})
})
