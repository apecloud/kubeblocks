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

package instanceset

import (
	"context"
	"fmt"
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	publiclifecycle "github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
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
		It("should work well", func() {
			By("PreCondition")
			its.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			reconciler = NewReplicasAlignmentReconciler(nil)
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
	})
})

func memberWorkload() *workloads.InstanceSet {
	return &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1}, Spec: workloads.InstanceSetSpec{LifecycleActions: &workloads.LifecycleActions{MemberJoin: testAction("join"), MemberLeave: testAction("leave")}}}
}
func memberPod() *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db"}, {Name: kbagent.ContainerName}}}}
}
func testAction(command string) *apps.Action {
	return &apps.Action{Exec: &apps.ExecAction{Command: []string{"/bin/sh", "-c", command}}}
}

func TestRegistrationSeparatesBootstrapAndLaterAllocation(t *testing.T) {
	its := memberWorkload()
	assignments := []instancetemplate.Assignment{{InstanceName: "demo-0"}, {InstanceName: "demo-1", TemplateName: "alternate"}}
	if !registerMemberStatus(its, its.DeepCopy(), assignments, sets.New[string](), sets.New[string]()) {
		t.Fatal("bootstrap not registered")
	}
	for _, status := range its.Status.InstanceStatus {
		if status.MemberJoined != nil || status.Provisioned {
			t.Fatalf("bootstrap invented results: %+v", status)
		}
	}
	assignments = append(assignments, instancetemplate.Assignment{InstanceName: "demo-2"})
	if !registerMemberStatus(its, its.DeepCopy(), assignments, sets.New[string](), sets.New[string]()) {
		t.Fatal("new identity not registered")
	}
	status := its.FindInstanceStatus("demo-2")
	if status.MemberJoined == nil || *status.MemberJoined {
		t.Fatalf("new empty allocation not tracked: %+v", status)
	}
	its = memberWorkload()
	if registerMemberStatus(its, its.DeepCopy(), nil, sets.New[string](), sets.New[string]()) || its.Status.ObservedGeneration != 0 {
		t.Fatal("empty allocation changed revision publication")
	}
	its.Status.ObservedGeneration = its.Generation
	if !registerMemberStatus(its, its.DeepCopy(), assignments[:1], sets.New[string](), sets.New[string]()) || its.FindInstanceStatus("demo-0").MemberJoined == nil {
		t.Fatal("zero replica increase was treated as bootstrap")
	}
	its = memberWorkload()
	registerMemberStatus(its, its.DeepCopy(), assignments, sets.New("demo-1"), sets.New("demo-0"))
	if its.FindInstanceStatus("demo-1").MemberJoined != nil {
		t.Fatal("unobserved old resources were classified as empty")
	}
}

type memberAgent struct{ calls int }

func (a *memberAgent) Close() error { return nil }

func (a *memberAgent) Action(_ context.Context, _ proto.ActionRequest) (proto.ActionResponse, error) {
	a.calls++
	return proto.ActionResponse{}, nil
}

func TestValidatedExecutionPreservesActionSelection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		action   *apps.Action
		podCount int
		calls    int
	}{
		{name: "default", action: testAction("join"), podCount: 2, calls: 1},
		{name: "any", action: &apps.Action{TargetPodSelector: apps.AnyReplica, Exec: testAction("join").Exec}, podCount: 2, calls: 1},
		{name: "legacy any", action: &apps.Action{Exec: &apps.ExecAction{Command: []string{"join"}, TargetPodSelector: apps.AnyReplica}}, podCount: 2, calls: 1},
		{name: "all", action: &apps.Action{TargetPodSelector: apps.AllReplicas, Exec: testAction("join").Exec}, podCount: 2, calls: 2},
		{name: "single all", action: &apps.Action{TargetPodSelector: apps.AllReplicas, Exec: testAction("join").Exec}, podCount: 1, calls: 1},
		{name: "role", action: &apps.Action{TargetPodSelector: apps.RoleSelector, MatchingKey: "replica", Exec: testAction("join").Exec}, podCount: 2, calls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			its := memberWorkload()
			its.Spec.LifecycleActions.MemberJoin = tc.action
			pods := []*corev1.Pod{memberPod(), memberPod()}
			pods[1].Name = "demo-2"
			pods = pods[:tc.podCount]
			for _, pod := range pods {
				pod.Labels = map[string]string{constant.RoleLabelKey: "replica"}
				pod.Status.PodIP = "10.0.0.1"
				pod.Spec.InitContainers = nil
				pod.Spec.Containers[1] = corev1.Container{Name: kbagent.ContainerName, Ports: []corev1.ContainerPort{{Name: kbagent.DefaultHTTPPortName, ContainerPort: 3501}}}
			}
			opts, err := validateMemberExecution(tc.action, pods[0], pods)
			if err != nil {
				t.Fatal(err)
			}
			if (tc.name == "any" || tc.name == "legacy any") && (opts == nil || opts.TargetPodName == "") {
				t.Fatal("random executor was not retained for execution")
			}
			if tc.name == "single all" && opts != nil {
				t.Fatal("AllReplicas lost its aggregate-error semantics")
			}
			lfa, err := publiclifecycle.New(its.Namespace, "", "", &apps.ComponentLifecycleActions{MemberJoin: tc.action}, its.Spec.LifecycleActions.TemplateVars, pods[0], pods)
			if err != nil {
				t.Fatal(err)
			}
			agent := &memberAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			if err := lfa.MemberJoin(context.Background(), nil, opts); err != nil {
				t.Fatal(err)
			}
			if agent.calls != tc.calls {
				t.Fatalf("action executed %d times, want %d", agent.calls, tc.calls)
			}
		})
	}
}

func TestValidatedExecutionWaitsWithoutExecutors(t *testing.T) {
	pod := memberPod()
	if _, err := validateMemberExecution(testAction("join"), pod, []*corev1.Pod{pod}); err == nil {
		t.Fatal("missing agent endpoint would record success")
	}
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			defer func() {
				if got := recover(); got != nil {
					t.Fatalf("missing executor must return an error instead of panic: %v", got)
				}
			}()
			action := testAction("leave")
			if legacy {
				action.Exec.TargetPodSelector = apps.AnyReplica
			} else {
				action.TargetPodSelector = apps.AnyReplica
			}
			if _, err := validateMemberExecution(action, memberPod(), nil); err == nil {
				t.Fatal("missing executor was accepted")
			}
		})
	}
}

func TestValidationRejectsUnownedPreconditionsAndNonblockingActions(t *testing.T) {
	if err := validateMemberActions(memberWorkload()); err != nil {
		t.Fatalf("blocking actions were rejected: %v", err)
	}
	for _, condition := range []apps.PreConditionType{apps.RuntimeReadyPreConditionType, apps.ComponentReadyPreConditionType, apps.ClusterReadyPreConditionType} {
		its := memberWorkload()
		its.Spec.LifecycleActions.MemberJoin.PreCondition = ptr.To(condition)
		if err := validateMemberActions(its); err == nil {
			t.Fatalf("accepted unowned precondition %s", condition)
		}
	}
	for _, name := range []string{"join", "leave", "switch"} {
		its := memberWorkload()
		switch name {
		case "join":
			its.Spec.LifecycleActions.MemberJoin.NonBlocking = true
		case "leave":
			its.Spec.LifecycleActions.MemberLeave.NonBlocking = true
		case "switch":
			its.Spec.LifecycleActions.Switchover = testAction("true")
			its.Spec.LifecycleActions.Switchover.NonBlocking = true
		}
		if err := validateMemberActions(its); err == nil {
			t.Fatalf("accepted nonblocking %s without a completion contract", name)
		}
	}
	its := memberWorkload()
	its.Spec.LifecycleActions.Switchover = testAction("switch")
	its.Spec.LifecycleActions.Switchover.PreCondition = ptr.To(apps.ComponentReadyPreConditionType)
	if err := validateMemberActions(its); err == nil {
		t.Fatal("new switchover accepts a Component dependency")
	}
}
