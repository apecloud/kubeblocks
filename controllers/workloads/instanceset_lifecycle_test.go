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

package workloads

import (
	"context"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
)

type lifecycleEntry struct {
	cli         client.Client
	its         *workloads.InstanceSet
	instanceAPI bool
}

func newMemberLifecycleEntry(t *testing.T, instanceAPI bool, replicas int32, policy appsv1.PodManagementPolicyType) *lifecycleEntry {
	t.Helper()
	var its *workloads.InstanceSet
	var cli client.Client
	if instanceAPI {
		its, cli, _ = its2LifecycleFixture(t, replicas, false)
	} else {
		its = newDataLifecycleITS(replicas)
		its.Spec.LifecycleActions.DataDump, its.Spec.LifecycleActions.DataLoad = nil, nil
		its.Spec.LifecycleActions.MemberJoin = &apps.Action{Exec: &apps.ExecAction{Command: []string{"join"}}}
		its.Spec.LifecycleActions.MemberLeave = &apps.Action{Exec: &apps.ExecAction{Command: []string{"leave"}}}
		cli = newLifecycleEntryClient(t, its, interceptor.Funcs{})
	}
	its.Spec.PodManagementPolicy = policy
	its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	return &lifecycleEntry{cli: cli, its: its, instanceAPI: instanceAPI}
}

func newDataLifecycleEntry(t *testing.T, instanceAPI bool, replicas int32, policy appsv1.PodManagementPolicyType) *lifecycleEntry {
	t.Helper()
	var its *workloads.InstanceSet
	var cli client.Client
	if instanceAPI {
		its, cli, _ = its2LifecycleFixture(t, replicas, true)
	} else {
		its = newDataLifecycleITS(replicas)
		its.Spec.LifecycleActions.MemberJoin = &apps.Action{Exec: &apps.ExecAction{Command: []string{"join"}}}
		its.Spec.LifecycleActions.MemberLeave = &apps.Action{Exec: &apps.ExecAction{Command: []string{"leave"}}}
		cli = newLifecycleEntryClient(t, its, interceptor.Funcs{})
	}
	its.Spec.PodManagementPolicy = policy
	its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	return &lifecycleEntry{cli: cli, its: its, instanceAPI: instanceAPI}
}

func (f *lifecycleEntry) step(t *testing.T) ctrl.Result {
	t.Helper()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.its)}
	var result ctrl.Result
	var err error
	if f.instanceAPI {
		r := &InstanceSetReconciler2{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
		result, err = r.Reconcile(context.Background(), req)
	} else {
		r := &InstanceSetReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
		result, err = r.Reconcile(context.Background(), req)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Get(context.Background(), req.NamespacedName, f.its); err != nil {
		t.Fatal(err)
	}
	return result
}

func (f *lifecycleEntry) stepWithChildren(t *testing.T) ctrl.Result {
	t.Helper()
	result := f.step(t)
	if f.instanceAPI {
		children := &workloads.InstanceList{}
		if err := f.cli.List(context.Background(), children); err != nil {
			t.Fatal(err)
		}
		for i := range children.Items {
			leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
			if _, err := leaf.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&children.Items[i])}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return result
}

func (f *lifecycleEntry) setReady(t *testing.T, name string, ready bool) {
	t.Helper()
	ctx := context.Background()
	pod := &corev1.Pod{}
	if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: name}, pod); err != nil {
		t.Fatal(err)
	}
	pod.Status = corev1.PodStatus{InitContainerStatuses: pod.Status.InitContainerStatuses, Phase: corev1.PodRunning, PodIP: "10.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse, LastTransitionTime: metav1.Now()}}}
	if ready {
		pod.Status.Conditions[0].Status = corev1.ConditionTrue
	}
	for _, c := range pod.Spec.Containers {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{Name: c.Name, Image: c.Image, Ready: ready})
	}
	if err := f.cli.Status().Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	if f.instanceAPI {
		leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
		if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(pod)}); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *lifecycleEntry) bootstrap(t *testing.T, replicas int) {
	t.Helper()
	for round := 0; round < 24; round++ {
		result := f.stepWithChildren(t)
		pods := lifecycleEntryPods(t, f.cli)
		for _, pod := range pods {
			f.setReady(t, pod.Name, true)
		}
		if len(pods) == replicas && result.RequeueAfter == 5*time.Second && f.its.Status.ReadyReplicas == int32(replicas) {
			return
		}
	}
	t.Fatalf("bootstrap did not converge: %#v", f.its.Status)
}

func (f *lifecycleEntry) markDataLoaded(t *testing.T, name string) {
	t.Helper()
	markDataWorkerComplete(t, f.cli, client.ObjectKey{Namespace: f.its.Namespace, Name: name})
}

func TestInstanceSetLifecycleReplacementWaitsForJoin(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, data := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/data=%t", instanceAPI, data), func(t *testing.T) {
				ctx := context.Background()
				var f *lifecycleEntry
				if data {
					f = newDataLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
				} else {
					f = newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
				}
				f.bootstrap(t, 1)
				sourceName, targetName := f.its.Name+"-0", f.its.Name+"-1"
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{1}}
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
					if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: targetName}, &corev1.Pod{}); err == nil {
						break
					} else if !apierrors.IsNotFound(err) {
						t.Fatal(err)
					}
				}
				if data {
					f.markDataLoaded(t, targetName)
				}
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: sourceName}, &corev1.Pod{}); err != nil || len(agent.calls) != 0 {
					t.Fatalf("old member retired before replacement joined: %v calls=%v", err, agent.calls)
				}
				f.setReady(t, targetName, true)
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				if len(agent.calls) != 2 || agent.calls[0] != "memberJoin" || agent.calls[1] != "memberLeave" {
					t.Fatalf("replacement did not join before leave: %v", agent.calls)
				}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: sourceName}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("joined replacement did not release old member: %v", err)
				}
			})
		}
	}
}

func TestInstanceSetLifecyclePendingCopiesAllowCancellationAndOffline(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, operation := range []struct{ offline, loaded, noSource bool }{{}, {loaded: true}, {noSource: true}, {offline: true}} {
			t.Run(fmt.Sprintf("instanceAPI=%t/offline=%t/loaded=%t/noSource=%t", instanceAPI, operation.offline, operation.loaded, operation.noSource), func(t *testing.T) {
				ctx := context.Background()
				bootstrap := 1
				if operation.offline {
					bootstrap = 2
				}
				f := newDataLifecycleEntry(t, instanceAPI, int32(bootstrap), appsv1.ParallelPodManagement)
				if operation.offline {
					f.its.Spec.LifecycleActions.DataDump.TargetPodSelector = apps.OrdinalSelector
					f.its.Spec.LifecycleActions.DataDump.MatchingKey = "0"
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
				}
				f.bootstrap(t, bootstrap)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](3)
				f.its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(3))
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				for _, pod := range lifecycleEntryPods(t, f.cli) {
					if pod.Name == f.its.Name+"-0" || (operation.offline && pod.Name == f.its.Name+"-1") {
						continue
					}
					pod.Status.Phase = corev1.PodPending
					if err := f.cli.Status().Update(ctx, &pod); err != nil {
						t.Fatal(err)
					}
				}
				removeName := f.its.Name + "-2"
				pendingName := f.its.Name + "-1"
				if operation.noSource {
					f.its.Spec.LifecycleActions.DataDump.TargetPodSelector = apps.OrdinalSelector
					f.its.Spec.LifecycleActions.DataDump.MatchingKey = "99"
				}
				if operation.loaded {
					f.markDataLoaded(t, removeName)
					for round := 0; round < 6; round++ {
						f.stepWithChildren(t)
					}
				}
				if operation.offline {
					removeName = f.its.Name + "-0"
					pendingName = f.its.Name + "-2"
					f.its.Spec.OfflineInstances = []string{removeName}
				} else {
					f.its.Spec.Replicas = ptr.To[int32](2)
				}
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 16; round++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: removeName}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("another pending copy blocked requested runtime removal: %v", err)
				}
				status := f.its.FindInstanceStatus(pendingName)
				if status == nil || status.DataLoaded == nil || *status.DataLoaded || len(agent.calls) != 0 {
					t.Fatalf("uncompleted copy changed result or membership: %#v calls=%v", status, agent.calls)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleDeletionPreservesExternalDataPVC(t *testing.T) {
	for _, policy := range []apps.PersistentVolumeClaimRetentionPolicyType{"", apps.DeletePersistentVolumeClaimRetentionPolicyType, apps.RetainPersistentVolumeClaimRetentionPolicyType} {
		t.Run(string(policy), func(t *testing.T) {
			ctx := context.Background()
			its := newDataLifecycleITS(1)
			its.Spec.VolumeClaimTemplates = nil
			its.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "external-data"}}}}
			if policy != "" {
				its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenDeleted: policy}
			}
			cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "external-data", Namespace: its.Namespace, UID: "external-data", Annotations: map[string]string{"owner": "external"}}}
			if err := cli.Create(ctx, pvc); err != nil {
				t.Fatal(err)
			}
			f := &lifecycleEntry{cli: cli, its: its}
			f.bootstrap(t, 1)
			before := pvc.DeepCopy()
			if err := cli.Delete(ctx, its); err != nil {
				t.Fatal(err)
			}
			deleted := false
			for round := 0; round < 8; round++ {
				r := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
				if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(its)}); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(its), &workloads.InstanceSet{}); apierrors.IsNotFound(err) {
					deleted = true
					break
				} else if err != nil {
					t.Fatal(err)
				}
			}
			if !deleted {
				t.Fatal("external data PVC prevented InstanceSet deletion")
			}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
				t.Fatalf("external data PVC was deleted: %v", err)
			}
			if pvc.ResourceVersion != before.ResourceVersion || pvc.UID != before.UID || len(pvc.OwnerReferences) != 0 || pvc.Annotations["owner"] != "external" {
				t.Fatalf("external data PVC was modified: %#v", pvc)
			}
			if pods := lifecycleEntryPods(t, cli); len(pods) != 0 {
				t.Fatalf("owned Pods survived InstanceSet deletion: %v", pods)
			}
		})
	}
}

func TestInstanceSetLifecycleRetirementRespectsAlignmentAdmission(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			t.Run(fmt.Sprintf("instanceAPI=%t/%s", instanceAPI, policy), func(t *testing.T) {
				f := newMemberLifecycleEntry(t, instanceAPI, 3, policy)
				f.bootstrap(t, 3)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](1)
				f.its.Generation++
				if err := f.cli.Update(context.Background(), f.its); err != nil {
					t.Fatal(err)
				}
				if policy == appsv1.ParallelPodManagement {
					f.setReady(t, f.its.Name+"-0", false)
					f.step(t)
					if len(agent.calls) != 0 {
						t.Fatalf("Leave ran without parallel admission: %v", agent.calls)
					}
					f.setReady(t, f.its.Name+"-0", true)
				}
				for round := 0; round < 6 && len(agent.calls) == 0; round++ {
					f.step(t)
				}
				retired := f.its.FindInstanceStatus(f.its.Name + "-2")
				next := f.its.FindInstanceStatus(f.its.Name + "-1")
				if len(agent.calls) != 1 || agent.calls[0] != "memberLeave" || retired == nil || retired.MemberJoined == nil || *retired.MemberJoined || next == nil || next.MemberJoined != nil {
					t.Fatalf("membership bypassed retirement order: calls=%v status=%#v", agent.calls, f.its.Status.InstanceStatus)
				}
				if len(lifecycleEntryPods(t, f.cli)) != 3 {
					t.Fatal("member result round removed runtime")
				}
				if instanceAPI && ptr.Deref(its2LifecycleInstance(t, f.cli, f.its.Name+"-2").Spec.ScaledDown, false) {
					t.Fatal("member result round marked ScaledDown")
				}
				f.step(t)
				if len(agent.calls) != 1 {
					t.Fatal("another Leave ran while committing the first runtime removal")
				}
				if instanceAPI {
					if !ptr.Deref(its2LifecycleInstance(t, f.cli, f.its.Name+"-2").Spec.ScaledDown, false) {
						t.Fatal("committed Leave did not permit ScaledDown")
					}
				} else if len(lifecycleEntryPods(t, f.cli)) != 2 {
					t.Fatal("committed Leave did not permit deletion")
				}
			})
		}
	}
}

func TestInstanceSetLifecycleReachesUpdateAndPollsSuccessfulExits(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
			f.bootstrap(t, 1)
			f.its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: apps.OnDeleteStrategyType}
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			if result := f.step(t); result.RequeueAfter != 5*time.Second {
				t.Fatalf("successful OnDelete exit lost observation poll: %#v", result)
			}
			f.its.Spec.InstanceUpdateStrategy = nil
			f.its.Spec.Template.Spec.Containers[0].Image = "database:v2"
			f.its.Spec.Template.Spec.Containers[0].Command = []string{"database-v2"}
			f.its.Spec.PodUpdatePolicy = apps.ReCreatePodUpdatePolicyType
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			handedOff := false
			for round := 0; round < 6; round++ {
				result := f.step(t)
				if instanceAPI {
					child := its2LifecycleInstance(t, f.cli, f.its.Name+"-0")
					handedOff = child.Spec.Template.Spec.Containers[0].Image == "database:v2"
				} else {
					pods := lifecycleEntryPods(t, f.cli)
					handedOff = len(pods) == 0 || pods[0].Spec.Containers[0].Image == "database:v2"
				}
				if handedOff {
					if result.RequeueAfter == 0 {
						t.Fatal("successful update lost lifecycle poll")
					}
					break
				}
			}
			if !handedOff {
				t.Fatal("lifecycle alignment prevented actual rollout")
			}
		})
	}
}

func TestInstanceSetLifecyclePollsUnalignedUpdateExit(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
			f.bootstrap(t, 1)
			f.setReady(t, f.its.Name+"-0", false)
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			f.step(t)
			if result := f.step(t); result.RequeueAfter != 5*time.Second {
				t.Fatalf("unaligned update exit lost observation poll: %#v", result)
			}
			if len(lifecycleEntryPods(t, f.cli)) != 1 {
				t.Fatal("OrderedReady admitted creation before predecessor became available")
			}
		})
	}
}

func TestITS2LifecycleInstanceWithoutRuntimeIsNotProvisioned(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1, false)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	child := its2LifecycleInstance(t, cli, "lifecycle-0")
	for round := 0; round < 3; round++ {
		its2LifecycleRound(t, r, its)
	}
	stored := its2LifecycleRead(t, cli, its)
	status := stored.FindInstanceStatus(child.Name)
	if status == nil || status.Provisioned || status.MemberJoined != nil || len(lifecycleEntryPods(t, cli)) != 0 {
		t.Fatalf("Instance spec invented runtime or member result: %#v", stored.Status.InstanceStatus)
	}
	stored.Spec.Replicas = ptr.To[int32](0)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	agent := &its2LifecycleAgent{}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	its2LifecycleRound(t, r, its)
	if len(agent.calls) != 0 || !ptr.Deref(its2LifecycleInstance(t, cli, child.Name).Spec.ScaledDown, false) {
		t.Fatalf("unprovisioned identity required an invented Leave: %v", agent.calls)
	}
}

func TestInstanceSetLifecycleAllocationSwitchSeparatesCreationAndMemberResult(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newMemberLifecycleEntry(t, instanceAPI, 2, appsv1.ParallelPodManagement)
			f.bootstrap(t, 2)
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			f.its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{0, 2}}
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			f.step(t)
			if len(agent.calls) != 0 || len(lifecycleEntryPods(t, f.cli)) != 2 {
				t.Fatal("registration changed runtime or membership")
			}
			f.step(t)
			if len(agent.calls) != 0 {
				t.Fatal("member action shared an uncommitted creation round")
			}
			if instanceAPI {
				leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
				for i := 0; i < 2; i++ {
					if _, err := leaf.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-2"}}); err != nil {
						t.Fatal(err)
					}
				}
			}
			f.setReady(t, f.its.Name+"-2", true)
			for round := 0; round < 6; round++ {
				f.step(t)
				if status := f.its.FindInstanceStatus(f.its.Name + "-2"); status != nil && ptr.Deref(status.MemberJoined, false) {
					break
				}
			}
			if len(agent.calls) != 1 || agent.calls[0] != "memberJoin" || len(lifecycleEntryPods(t, f.cli)) != 3 {
				t.Fatalf("new member did not join before retirement: calls=%v status=%#v", agent.calls, f.its.Status.InstanceStatus)
			}
			faulty := &its2LifecycleClient{Client: f.cli, failStatus: true}
			f.cli = faulty
			result := f.step(t)
			old := f.its.FindInstanceStatus(f.its.Name + "-1")
			if !result.Requeue || len(agent.calls) != 2 || agent.calls[1] != "memberLeave" || old == nil || old.MemberJoined != nil || len(lifecycleEntryPods(t, f.cli)) != 3 {
				t.Fatalf("member result conflict leaked runtime mutation: calls=%v status=%#v result=%#v", agent.calls, f.its.Status.InstanceStatus, result)
			}
			if instanceAPI && ptr.Deref(its2LifecycleInstance(t, f.cli, f.its.Name+"-1").Spec.ScaledDown, false) {
				t.Fatal("member result conflict leaked ScaledDown")
			}
			f.step(t)
			old = f.its.FindInstanceStatus(f.its.Name + "-1")
			wantCalls := 3
			if instanceAPI {
				wantCalls = 2
			}
			if len(agent.calls) != wantCalls || old == nil || old.MemberJoined == nil || *old.MemberJoined || len(lifecycleEntryPods(t, f.cli)) != 3 {
				t.Fatalf("member result retry did not commit alone: calls=%v status=%#v", agent.calls, f.its.Status.InstanceStatus)
			}
		})
	}
}

func TestITS2LifecycleZeroRegistrationPrecedesExpansion(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 0, true)
	its2LifecycleRound(t, r, its)
	stored := its2LifecycleRead(t, cli, its)
	if stored.Status.ObservedGeneration != stored.Generation || len(stored.Status.InstanceStatus) != 0 {
		t.Fatalf("zero bootstrap was not registered: %#v", stored.Status)
	}
	stored.Spec.Replicas = ptr.To[int32](1)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	cli.failStatus = true
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	children := &workloads.InstanceList{}
	if err := cli.List(context.Background(), children); err != nil {
		t.Fatal(err)
	}
	if len(children.Items) != 0 || len(stored.Status.InstanceStatus) != 0 || stored.Status.ObservedGeneration != 1 {
		t.Fatalf("new-generation registration conflict created runtime: %#v %#v", children.Items, stored.Status)
	}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	status := stored.FindInstanceStatus("lifecycle-0")
	if status == nil || status.DataLoaded == nil || *status.DataLoaded || status.MemberJoined == nil || *status.MemberJoined {
		t.Fatalf("zero-to-one was not registered as expansion: %#v", stored.Status)
	}
}

func TestInstanceSetLifecycleResumesDataSourceBeforeEarlierNamedTarget(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			t.Run(fmt.Sprintf("instanceAPI=%t/%s", instanceAPI, policy), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, policy)
				its, cli := f.its, f.cli
				its.Spec.Instances = []workloads.InstanceTemplate{{Name: "z", Replicas: ptr.To[int32](1)}}
				if err := cli.Update(ctx, its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 1)
				sourceName, targetName := its.Name+"-z-0", its.Name+"-a-0"
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Spec.Instances = append(f.its.Spec.Instances, workloads.InstanceTemplate{Name: "a", Replicas: ptr.To[int32](1)})
				f.its.Generation++
				if err := cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 6; round++ {
					f.stepWithChildren(t)
				}
				target := &corev1.Pod{}
				if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: targetName}, target); err != nil {
					t.Fatal(err)
				}
				if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" || ptr.Deref(f.its.FindInstanceStatus(targetName).DataLoaded, true) {
					t.Fatalf("expansion was not waiting on its data task: task=%s err=%v status=%#v", task, err, f.its.Status.InstanceStatus)
				}
				f.its.Spec.Stop = ptr.To(true)
				f.its.Generation++
				if err := cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				if len(lifecycleEntryPods(t, cli)) != 0 || len(agent.calls) != 0 {
					t.Fatalf("Stop did not remove runtimes without membership actions: %v", agent.calls)
				}
				if instanceAPI {
					for _, name := range []string{sourceName, targetName} {
						child := its2LifecycleInstance(t, cli, name)
						if !ptr.Deref(child.Spec.Stop, false) || child.Status.CurrentState != workloads.InstanceCurrentStateAbsent || child.Status.ObservedGeneration != child.Generation {
							t.Fatalf("Instance did not acknowledge Stop: %#v", child)
						}
					}
				}
				f.its.Spec.Stop = ptr.To(false)
				f.its.Generation++
				if err := cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				waitForPod := func(name string) *corev1.Pod {
					t.Helper()
					for round := 0; round < 8; round++ {
						f.stepWithChildren(t)
						pod := &corev1.Pod{}
						err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: name}, pod)
						if err == nil {
							return pod
						}
						if !apierrors.IsNotFound(err) {
							t.Fatal(err)
						}
					}
					t.Fatalf("pending data target blocked resume of %s: %#v", name, f.its.Status.Conditions)
					return nil
				}
				waitForPod(sourceName)
				f.setReady(t, sourceName, true)
				target = waitForPod(targetName)
				if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" || len(agent.calls) != 0 {
					t.Fatalf("resume lost pending loading or executed membership early: task=%s calls=%v err=%v", task, agent.calls, err)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleKeepsReplacementDataSourceUntilLoadCompletes(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			t.Run(fmt.Sprintf("instanceAPI=%t/%s", instanceAPI, policy), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, policy)
				f.bootstrap(t, 1)
				sourceName, targetName := f.its.Name+"-0", f.its.Name+"-1"
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.setReady(t, sourceName, false)
				f.its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{1}}
				f.its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(2))
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 6; round++ {
					f.stepWithChildren(t)
				}
				if source := f.its.FindInstanceStatus(sourceName); source == nil || source.DesiredState != workloads.InstanceDesiredStateReleased {
					t.Fatalf("replacement did not release the old allocation: %#v", source)
				}
				f.setReady(t, sourceName, true)
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				target := &corev1.Pod{}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: targetName}, target); err != nil {
					t.Fatalf("recovered data source did not unblock replacement: %v", err)
				}
				if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" {
					t.Fatalf("replacement has no data task: %s %v", task, err)
				}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: sourceName}, &corev1.Pod{}); err != nil || len(agent.calls) != 0 {
					t.Fatalf("data source retired before loading completed: %v calls=%v", err, agent.calls)
				}
				name, err := lifecycleTestDataPVCName(f.its, target)
				if err != nil {
					t.Fatal(err)
				}
				pvc := &corev1.PersistentVolumeClaim{}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: name}, pvc); err != nil {
					t.Fatal(err)
				}
				if pvc.Annotations == nil {
					pvc.Annotations = map[string]string{}
				}
				f.markDataLoaded(t, targetName)
				if err := f.cli.Update(ctx, pvc); err != nil {
					t.Fatal(err)
				}
				f.setReady(t, targetName, true)
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				status := f.its.FindInstanceStatus(targetName)
				if status == nil || !ptr.Deref(status.DataLoaded, false) || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 2 || agent.calls[0] != "memberJoin" || agent.calls[1] != "memberLeave" {
					t.Fatalf("loaded replacement did not complete membership: status=%#v calls=%v", status, agent.calls)
				}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: sourceName}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("completed replacement retained old runtime: %v", err)
				}
			})
		}
	}
}

func TestInstanceSetLifecyclePendingDataDoesNotBlockStopOrCancellation(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, stop := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/stop=%t", instanceAPI, stop), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
				f.bootstrap(t, 1)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				targetName := f.its.Name + "-1"
				if status := f.its.FindInstanceStatus(targetName); status == nil || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("expansion is not waiting for data: %#v", status)
				}
				if stop {
					f.its.Spec.Stop = ptr.To(true)
				} else {
					f.its.Spec.Replicas = ptr.To[int32](1)
				}
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				wantPods := 1
				if stop {
					wantPods = 0
				}
				if pods := lifecycleEntryPods(t, f.cli); len(pods) != wantPods || len(agent.calls) != 0 {
					t.Fatalf("pending data blocked removal or caused membership calls: pods=%v calls=%v", pods, agent.calls)
				}
				if stop {
					status := f.its.FindInstanceStatus(targetName)
					if status == nil || status.DesiredState != workloads.InstanceDesiredStateOffline || status.DataLoaded == nil || *status.DataLoaded {
						t.Fatalf("Stop changed pending data result: %#v", status)
					}
				} else if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: targetName}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("cancelled data target still exists: %v", err)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleRegistersExpansionWhileStopped(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, data := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/data=%t", instanceAPI, data), func(t *testing.T) {
				ctx := context.Background()
				var f *lifecycleEntry
				if data {
					f = newDataLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
				} else {
					f = newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
				}
				f.bootstrap(t, 1)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Stop = ptr.To(true)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 6; round++ {
					f.stepWithChildren(t)
				}
				targetName := f.its.Name + "-1"
				status := f.its.FindInstanceStatus(targetName)
				if status == nil || status.EffectiveDesiredState() != workloads.InstanceDesiredStateOffline || status.MemberJoined == nil || *status.MemberJoined || (data && (status.DataLoaded == nil || *status.DataLoaded)) {
					t.Fatalf("stopped expansion lost loading or membership requirements: %#v", f.its.Status.InstanceStatus)
				}
				if len(lifecycleEntryPods(t, f.cli)) != 0 || len(agent.calls) != 0 {
					t.Fatal("stopped allocation created runtime or executed a member action")
				}
				f.its.Spec.Stop = ptr.To(false)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 4; round++ {
					f.stepWithChildren(t)
				}
				f.setReady(t, f.its.Name+"-0", true)
				for round := 0; round < 6; round++ {
					f.stepWithChildren(t)
				}
				target := &corev1.Pod{}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: targetName}, target); err != nil {
					t.Fatal(err)
				}
				if data {
					if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" || len(agent.calls) != 0 {
						t.Fatalf("resume bypassed loading before Join: task=%s calls=%v err=%v", task, agent.calls, err)
					}
					name, err := lifecycleTestDataPVCName(f.its, target)
					if err != nil {
						t.Fatal(err)
					}
					pvc := &corev1.PersistentVolumeClaim{}
					if err := f.cli.Get(ctx, client.ObjectKey{Namespace: target.Namespace, Name: name}, pvc); err != nil {
						t.Fatal(err)
					}
					if pvc.Annotations == nil {
						pvc.Annotations = map[string]string{}
					}
					f.markDataLoaded(t, targetName)
					if err := f.cli.Update(ctx, pvc); err != nil {
						t.Fatal(err)
					}
				}
				f.setReady(t, targetName, true)
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				status = f.its.FindInstanceStatus(targetName)
				if status == nil || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 1 || agent.calls[0] != "memberJoin" || (data && !ptr.Deref(status.DataLoaded, false)) {
					t.Fatalf("resumed allocation did not complete loading and Join: %#v calls=%v", status, agent.calls)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleKeepsHealthyDataSource(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newDataLifecycleEntry(t, instanceAPI, 2, appsv1.ParallelPodManagement)
			f.bootstrap(t, 2)
			source := &corev1.Pod{}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}, source); err != nil {
				t.Fatal(err)
			}
			source.Status.PodIP = "10.0.0.2"
			if err := f.cli.Status().Update(ctx, source); err != nil {
				t.Fatal(err)
			}
			f.its.Spec.Replicas = ptr.To[int32](3)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 6; round++ {
				f.stepWithChildren(t)
			}
			target := &corev1.Pod{}
			key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-2"}
			if err := f.cli.Get(ctx, key, target); err != nil {
				t.Fatal(err)
			}
			target.Status.Phase = corev1.PodPending
			if err := f.cli.Status().Update(ctx, target); err != nil {
				t.Fatal(err)
			}
			task, err := workloadlifecycle.GeneratedTask(&target.Spec)
			if err != nil || task == "" {
				t.Fatalf("expansion did not create a data task: %s %v", task, err)
			}
			for round := 0; round < 32; round++ {
				f.stepWithChildren(t)
				if err := f.cli.Get(ctx, key, target); err != nil {
					t.Fatalf("unchanged healthy donors caused target recreation: %v", err)
				}
				if current, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || current != task {
					t.Fatalf("unchanged healthy donors replaced the running task: before=%s after=%s err=%v", task, current, err)
				}
			}
		})
	}
}

func TestInstanceSetLifecycleRetainedPendingDataRecovery(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, removePVC := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/removePVC=%t", instanceAPI, removePVC), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
				f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: apps.RetainPersistentVolumeClaimRetentionPolicyType}
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 1)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				targetKey := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
				target := &corev1.Pod{}
				if err := f.cli.Get(ctx, targetKey, target); err != nil {
					t.Fatal(err)
				}
				if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" {
					t.Fatalf("initial expansion has no data task: %q %v", task, err)
				}
				if status := f.its.FindInstanceStatus(target.Name); status == nil || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("initial expansion did not record pending data: %#v", status)
				}
				pvcName, err := lifecycleTestDataPVCName(f.its, target)
				if err != nil {
					t.Fatal(err)
				}
				target.Status.Phase = corev1.PodPending
				if err := f.cli.Status().Update(ctx, target); err != nil {
					t.Fatal(err)
				}
				f.its.Spec.Replicas = ptr.To[int32](1)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 24; round++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(ctx, targetKey, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("canceled expansion kept its Pod: %v", err)
				}
				pvc := &corev1.PersistentVolumeClaim{}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: pvcName}, pvc); err != nil {
					t.Fatal("cancellation did not retain the incomplete data PVC", err)
				}
				if len(agent.calls) != 0 {
					t.Fatalf("cancellation invented data or membership completion: pvc=%#v calls=%v", pvc.Annotations, agent.calls)
				}
				if status := f.its.FindInstanceStatus(targetKey.Name); status == nil || status.DesiredState != workloads.InstanceDesiredStateReleased || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("retained incomplete PVC lost its loading result: %#v", status)
				}
				if removePVC {
					pvc.Finalizers = nil
					if err := f.cli.Update(ctx, pvc); err != nil {
						t.Fatal(err)
					}
					if err := f.cli.Delete(ctx, pvc); err != nil {
						t.Fatal(err)
					}
					for round := 0; round < 12; round++ {
						f.stepWithChildren(t)
					}
					if status := f.its.FindInstanceStatus(targetKey.Name); status != nil {
						t.Fatalf("removed retained PVC left a released identity: %#v", status)
					}
					return
				}
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 16; round++ {
					f.stepWithChildren(t)
				}
				status := f.its.FindInstanceStatus(targetKey.Name)
				if status == nil || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("re-expansion lost the incomplete data requirement: %#v", status)
				}
				if err := f.cli.Get(ctx, targetKey, target); err != nil {
					t.Fatal(err)
				}
				task := its2TaskInput(&workloads.Instance{Spec: workloads.InstanceSpec{Template: corev1.PodTemplateSpec{Spec: target.Spec}}})
				if task == "" || len(agent.calls) != 0 {
					t.Fatalf("re-expansion did not restore the task before joining: task=%s calls=%v", task, agent.calls)
				}
				f.markDataLoaded(t, target.Name)
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				f.setReady(t, target.Name, true)
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				status = f.its.FindInstanceStatus(target.Name)
				if status == nil || !ptr.Deref(status.DataLoaded, false) || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 1 || agent.calls[0] != "memberJoin" {
					t.Fatalf("re-expanded target did not complete data and join: status=%#v calls=%v", status, agent.calls)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleRetiresMemberWithoutPod(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newMemberLifecycleEntry(t, instanceAPI, 2, appsv1.ParallelPodManagement)
			f.its.Spec.LifecycleActions.MemberLeave.TargetPodSelector = apps.AnyReplica
			f.its.Spec.VolumeClaimTemplates = newDataLifecycleITS(2).Spec.VolumeClaimTemplates
			f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: apps.DeletePersistentVolumeClaimRetentionPolicyType}
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			f.bootstrap(t, 2)
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			targetKey := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
			target := &corev1.Pod{}
			if err := f.cli.Get(ctx, targetKey, target); err != nil {
				t.Fatal(err)
			}
			pvcKey := client.ObjectKey{Namespace: f.its.Namespace, Name: "data-" + target.Name}
			if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); err != nil {
				t.Fatal(err)
			}
			target.Finalizers = nil
			if err := f.cli.Update(ctx, target); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Delete(ctx, target); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Get(ctx, targetKey, &corev1.Pod{}); !apierrors.IsNotFound(err) {
				t.Fatalf("Pod deletion did not complete: %v", err)
			}
			f.its.Spec.Replicas = ptr.To[int32](1)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 24; round++ {
				f.stepWithChildren(t)
			}
			if len(agent.calls) != 1 || agent.calls[0] != "memberLeave" {
				t.Fatalf("absent member did not leave through its healthy peer: calls=%v status=%#v", agent.calls, f.its.FindInstanceStatus(target.Name))
			}
			if status := f.its.FindInstanceStatus(target.Name); status != nil {
				t.Fatalf("completed Leave retained the released identity: %#v", status)
			}
			if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); !apierrors.IsNotFound(err) {
				t.Fatalf("completed Leave did not clean the released PVC: %v", err)
			}
		})
	}
}

func TestInstanceSetLifecycleLeaveWaitsWithoutExecutor(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			t.Run(fmt.Sprintf("instanceAPI=%t/%s", instanceAPI, policy), func(t *testing.T) {
				defer func() {
					if got := recover(); got != nil {
						t.Fatalf("missing executor must wait instead of panic: %v", got)
					}
				}()
				ctx := context.Background()
				f := newMemberLifecycleEntry(t, instanceAPI, 1, policy)
				f.its.Spec.LifecycleActions.MemberLeave.TargetPodSelector = apps.AnyReplica
				f.its.Spec.VolumeClaimTemplates = newDataLifecycleITS(1).Spec.VolumeClaimTemplates
				f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: apps.DeletePersistentVolumeClaimRetentionPolicyType}
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 1)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-0"}
				pod := &corev1.Pod{}
				if err := f.cli.Get(ctx, key, pod); err != nil {
					t.Fatal(err)
				}
				// Observe completed Pod deletion before its controller gets a chance to recreate it.
				pod.Finalizers = nil
				if err := f.cli.Update(ctx, pod); err != nil {
					t.Fatal(err)
				}
				if err := f.cli.Delete(ctx, pod); err != nil {
					t.Fatal(err)
				}
				f.its.Spec.Replicas = ptr.To[int32](0)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 6; round++ {
					result := f.step(t)
					status := f.its.FindInstanceStatus(key.Name)
					if !workloadlifecycle.NeedsLeave(f.its, status) || result.RequeueAfter == 0 || len(agent.calls) != 0 {
						t.Fatalf("missing executor lost the pending Leave or retry: status=%#v result=%#v calls=%v", status, result, agent.calls)
					}
				}
				pvcKey := client.ObjectKey{Namespace: key.Namespace, Name: "data-" + key.Name}
				if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); err != nil {
					t.Fatalf("waiting for Leave removed storage: %v", err)
				}
				f.its.Spec.Replicas = ptr.To[int32](1)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 1)
				f.its.Spec.Replicas = ptr.To[int32](0)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 16; round++ {
					f.stepWithChildren(t)
				}
				if len(agent.calls) != 1 || agent.calls[0] != "memberLeave" || f.its.FindInstanceStatus(key.Name) != nil {
					t.Fatalf("restored executor did not finish Leave: calls=%v status=%#v", agent.calls, f.its.FindInstanceStatus(key.Name))
				}
				if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); !apierrors.IsNotFound(err) {
					t.Fatalf("completed Leave did not clean storage: %v", err)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleStopScaleThenResume(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			for _, retention := range []apps.PersistentVolumeClaimRetentionPolicyType{apps.DeletePersistentVolumeClaimRetentionPolicyType, apps.RetainPersistentVolumeClaimRetentionPolicyType} {
				t.Run(fmt.Sprintf("instanceAPI=%t/%s/%s", instanceAPI, policy, retention), func(t *testing.T) {
					ctx := context.Background()
					f := newMemberLifecycleEntry(t, instanceAPI, 2, policy)
					f.its.Spec.LifecycleActions.MemberLeave.TargetPodSelector = apps.AnyReplica
					f.its.Spec.VolumeClaimTemplates = newDataLifecycleITS(2).Spec.VolumeClaimTemplates
					f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: retention}
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					f.bootstrap(t, 2)
					key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
					pod := &corev1.Pod{}
					if err := f.cli.Get(ctx, key, pod); err != nil {
						t.Fatal(err)
					}
					wantDNS := pod.Spec.Hostname + "." + intctrlutil.ServiceFQDN(pod.Namespace, pod.Spec.Subdomain)
					failed := &unavailableMemberAgent{}
					kbacli.SetMockClient(failed, nil)
					defer kbacli.UnsetMockClient()
					f.its.Spec.Stop = ptr.To(true)
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for round := 0; round < 24; round++ {
						f.stepWithChildren(t)
					}
					if pods := lifecycleEntryPods(t, f.cli); len(pods) != 0 {
						t.Fatalf("Stop did not remove all Pods: %v", pods)
					}
					f.its.Spec.Replicas = ptr.To[int32](1)
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for round := 0; round < 8; round++ {
						f.stepWithChildren(t)
					}
					if failed.calls != 0 {
						t.Fatalf("Stop executed membership actions: %d", failed.calls)
					}
					f.its.Spec.Stop = ptr.To(false)
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for round := 0; round < 24; round++ {
						f.stepWithChildren(t)
						for _, current := range lifecycleEntryPods(t, f.cli) {
							if current.Name == f.its.Name+"-0" {
								f.setReady(t, current.Name, true)
							}
						}
					}
					if failed.calls == 0 || !workloadlifecycle.NeedsLeave(f.its, f.its.FindInstanceStatus(key.Name)) {
						t.Fatalf("resumed peer did not retry the failed Leave: calls=%d status=%#v", failed.calls, f.its.FindInstanceStatus(key.Name))
					}
					pvcKey := client.ObjectKey{Namespace: key.Namespace, Name: "data-" + key.Name}
					if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); err != nil {
						t.Fatalf("failed Leave removed storage: %v", err)
					}
					agent := &memberDNSAgent{}
					kbacli.SetMockClient(agent, nil)
					for round := 0; round < 16; round++ {
						f.stepWithChildren(t)
					}
					assertMemberDNSRequest(t, agent.requests, "memberLeave", "KB_LEAVE_MEMBER_POD_FQDN", key.Name, wantDNS)
					if status := f.its.FindInstanceStatus(key.Name); status != nil {
						t.Fatalf("completed Leave retained the released identity: %#v", status)
					}
					if instanceAPI {
						if err := f.cli.Get(ctx, key, &workloads.Instance{}); !apierrors.IsNotFound(err) {
							t.Fatalf("completed Leave retained the excluded Instance: %v", err)
						}
					}
					if err := f.cli.Get(ctx, key, &corev1.Pod{}); !apierrors.IsNotFound(err) {
						t.Fatalf("completed retirement retained a Pod: %v", err)
					}
					storageErr := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{})
					if retention == apps.DeletePersistentVolumeClaimRetentionPolicyType && !apierrors.IsNotFound(storageErr) {
						t.Fatalf("WhenScaled=Delete did not remove storage: %v", storageErr)
					}
					if retention == apps.RetainPersistentVolumeClaimRetentionPolicyType && storageErr != nil {
						t.Fatalf("WhenScaled=Retain did not preserve storage: %v", storageErr)
					}
				})
			}
		}
	}
}

func markDataWorkerComplete(t *testing.T, cli client.Client, key client.ObjectKey) {
	t.Helper()
	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), key, pod); err != nil {
		t.Fatal(err)
	}
	task, err := workloadlifecycle.GeneratedTask(&pod.Spec)
	if err != nil || task == "" {
		t.Fatalf("worker completion requires a data task: %q %v", task, err)
	}
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceSetLifecycleDataResultRetryAndCleanup(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newDataLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
			f.bootstrap(t, 1)
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
			for i := 0; i < 12; i++ {
				f.stepWithChildren(t)
				if err := f.cli.Get(ctx, key, &corev1.Pod{}); err == nil {
					break
				}
			}
			pod := &corev1.Pod{}
			if err := f.cli.Get(ctx, key, pod); err != nil {
				t.Fatal(err)
			}
			before, err := workloadlifecycle.GeneratedTask(&pod.Spec)
			if err != nil || before == "" {
				t.Fatalf("missing pending task: %s %v", before, err)
			}
			f.its.Spec.MinReadySeconds = 5
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			source := &corev1.Pod{}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-0"}, source); err != nil {
				t.Fatal(err)
			}
			for i := range source.Status.Conditions {
				source.Status.Conditions[i].LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Minute))
			}
			if err := f.cli.Status().Update(ctx, source); err != nil {
				t.Fatal(err)
			}
			// A failed attempt remains pending; a later successful attempt can be retried safely.
			pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}}}
			if err := f.cli.Status().Update(ctx, pod); err != nil {
				t.Fatal(err)
			}
			f.stepWithChildren(t)
			if ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) {
				t.Fatal("failed worker completed loading")
			}
			markDataWorkerComplete(t, f.cli, key)
			faulty := &its2LifecycleClient{Client: f.cli, rejectDataResult: true}
			f.cli = faulty
			for i := 0; i < 3; i++ {
				f.stepWithChildren(t)
			}
			if faulty.dataResultFailures == 0 || ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) {
				t.Fatal("completion failure was not exercised")
			}
			if instanceAPI {
				inst := its2LifecycleInstance(t, f.cli, key.Name)
				if ptr.Deref(inst.Status.DataLoaded, false) || its2TaskInput(inst) == "" {
					t.Fatal("failed status write cleaned Instance task")
				}
			}
			if err := f.cli.Get(ctx, key, pod); err != nil {
				t.Fatal(err)
			}
			if task, err := workloadlifecycle.GeneratedTask(&pod.Spec); err != nil || task != before {
				t.Fatalf("pending task changed: %s %v", task, err)
			}
			faulty.rejectDataResult = false
			for i := 0; i < 8; i++ {
				f.stepWithChildren(t)
			}
			if !ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) {
				t.Fatal("successful worker result not recovered")
			}
			if f.its.FindInstanceStatus(key.Name).Ready {
				t.Fatal("fixture did not exercise completion before Pod readiness")
			}
			pod.Finalizers = nil
			if err := f.cli.Update(ctx, pod); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Delete(ctx, pod); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 8; i++ {
				f.stepWithChildren(t)
			}
			if err := f.cli.Get(ctx, key, pod); err != nil {
				t.Fatal(err)
			}
			if task, err := workloadlifecycle.GeneratedTask(&pod.Spec); err != nil || task != "" {
				t.Fatalf("completed instance regenerated loading: %s %v", task, err)
			}
		})
	}
}

func TestITS2MembershipResultRetriesInstanceStatusConflict(t *testing.T) {
	ctx := context.Background()
	reject := true
	failures := 0
	storage := fake.NewClientBuilder().WithScheme(model.GetScheme()).
		WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}).
		WithInterceptorFuncs(interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, cli client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if inst, ok := obj.(*workloads.Instance); ok && sub == "status" && ptr.Deref(inst.Status.MemberJoined, false) && reject {
				failures++
				return apierrors.NewConflict(workloads.Resource("instances"), inst.Name, fmt.Errorf("injected member status conflict"))
			}
			return cli.SubResource(sub).Update(ctx, obj, opts...)
		}}).Build()
	its, cli, _ := its2LifecycleFixture(t, 1, false, storage)
	f := &lifecycleEntry{cli: cli, its: its, instanceAPI: true}
	f.bootstrap(t, 1)
	agent := &its2LifecycleAgent{}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	f.its.Spec.Replicas = ptr.To[int32](2)
	f.its.Generation++
	if err := cli.Update(ctx, f.its); err != nil {
		t.Fatal(err)
	}
	key := client.ObjectKey{Namespace: its.Namespace, Name: "lifecycle-1"}
	for i := 0; i < 12; i++ {
		f.stepWithChildren(t)
		if err := cli.Get(ctx, key, &corev1.Pod{}); err == nil {
			break
		}
	}
	f.setReady(t, key.Name, true)
	for i := 0; i < 3; i++ {
		f.stepWithChildren(t)
	}
	if failures == 0 || ptr.Deref(its2LifecycleInstance(t, cli, key.Name).Status.MemberJoined, false) || ptr.Deref(f.its.FindInstanceStatus(key.Name).MemberJoined, false) {
		t.Fatal("failed child status commit leaked membership success")
	}
	reject = false
	for i := 0; i < 6; i++ {
		f.stepWithChildren(t)
	}
	if !ptr.Deref(its2LifecycleInstance(t, cli, key.Name).Status.MemberJoined, false) || !ptr.Deref(f.its.FindInstanceStatus(key.Name).MemberJoined, false) {
		t.Fatal("membership did not recover through Instance status")
	}
	calls := len(agent.calls)
	for i := 0; i < 3; i++ {
		f.stepWithChildren(t)
	}
	if len(agent.calls) != calls {
		t.Fatal("committed membership was called again")
	}
}

func lifecycleTestDataPVCName(_ *workloads.InstanceSet, pod *corev1.Pod) (string, error) {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == "data" && volume.PersistentVolumeClaim != nil {
			return volume.PersistentVolumeClaim.ClaimName, nil
		}
	}
	return "", fmt.Errorf("fixture has no data PVC")
}
