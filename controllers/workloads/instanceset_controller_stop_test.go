/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package workloads

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// The fake API server needs to invalidate child observations after spec writes.
type setStopClient struct {
	client.Client
	failChildWriteAt  int
	childWrites       int
	failResumeWriteAt int
	resumeWrites      int
}

type failSetStopStatusClient struct {
	client.Client
	fail bool
}

func (c *failSetStopStatusClient) Status() client.SubResourceWriter {
	return &failSetStopStatusWriter{SubResourceWriter: c.Client.Status(), owner: c}
}

type failSetStopStatusWriter struct {
	client.SubResourceWriter
	owner *failSetStopStatusClient
}

func (w *failSetStopStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if _, ok := obj.(*workloads.InstanceSet); ok && w.owner.fail {
		w.owner.fail = false
		return errors.New("injected parent status failure")
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

func TestInstanceSetStopRecoversAfterParentStatusFailure(t *testing.T) {
	its := setStopFixture()
	its.Spec.EnableInstanceAPI = ptr.To(true)
	cli := newSetStopClient(t, its)
	reconcileSetStop(t, cli, false, 5)
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(true)
		its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
	})
	faulty := &failSetStopStatusClient{Client: cli, fail: true}
	if _, err := reconcileSetStopParent(faulty, false); err == nil {
		t.Fatal("parent status failure was not reached")
	}
	stopped := 0
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) {
			stopped++
		}
	}
	if stopped != 1 {
		t.Fatalf("root failure did not follow persisted child handoff: %d", stopped)
	}
	if _, err := reconcileSetStopParent(cli, false); err != nil {
		t.Fatal(err)
	}
	stopped = 0
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) {
			stopped++
		}
	}
	if stopped != 1 {
		t.Fatal("root status retry repeated Stop beyond admitted budget")
	}
	reconcileSetStop(t, cli, false, 10)
	if len(listSetStopPods(t, cli)) != 0 || readSetStop(t, cli).Status.Replicas != 0 {
		t.Fatal("root status failure did not converge")
	}
}

func (c *setStopClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if inst, ok := obj.(*workloads.Instance); ok {
		old := &workloads.Instance{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(inst), old); err != nil {
			return err
		}
		if !reflect.DeepEqual(old.Spec, inst.Spec) {
			c.childWrites++
			if ptr.Deref(old.Spec.Stop, false) && !ptr.Deref(inst.Spec.Stop, false) {
				c.resumeWrites++
				if c.failResumeWriteAt > 0 && c.resumeWrites == c.failResumeWriteAt {
					return errors.New("injected resume write failure")
				}
			}
			if c.failChildWriteAt > 0 && c.childWrites == c.failChildWriteAt {
				return errors.New("injected child write failure")
			}
			inst.Generation = old.Generation + 1
		}
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *setStopClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if inst, ok := obj.(*workloads.Instance); ok && inst.Generation == 0 {
		inst.Generation = 1
	}
	return c.Client.Create(ctx, obj, opts...)
}

func TestInstanceSetStopIgnoresMalformedObservedConfig(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(map[bool]string{true: "legacy", false: "instances"}[legacy], func(t *testing.T) {
			its := setStopFixture()
			its.Spec.EnableInstanceAPI = ptr.To(!legacy)
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, legacy, 5)
			for _, pod := range listSetStopPods(t, cli) {
				if pod.Annotations == nil {
					pod.Annotations = map[string]string{}
				}
				pod.Annotations[constant.CMInsConfigurationHashLabelKey] = "invalid-json"
				if err := cli.Update(context.Background(), &pod); err != nil {
					t.Fatal(err)
				}
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			reconcileSetStop(t, cli, legacy, 8)
			if len(listSetStopPods(t, cli)) != 0 || readSetStop(t, cli).Status.Replicas != 0 {
				t.Fatal("malformed runtime config blocked Stop")
			}
		})
	}
}

func TestInstanceSetStopKeepsClaimsOfRemovedTemplate(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(map[bool]string{true: "legacy", false: "instances"}[legacy], func(t *testing.T) {
			its := setStopFixture()
			its.Spec.EnableInstanceAPI = ptr.To(!legacy)
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, legacy, 5)
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			reconcileSetStop(t, cli, legacy, 8)
			claims := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), claims); err != nil {
				t.Fatal(err)
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.VolumeClaimTemplates = nil; its.Spec.Stop = ptr.To(false) })
			reconcileSetStop(t, cli, legacy, 5)
			retained := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), retained); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(claims.Items, retained.Items) {
				t.Fatal("removing a claim template deleted allocated-identity storage")
			}
		})
	}
}

func TestInstanceSetStopSynchronizesClonedAssistantsBeforeNameAlignment(t *testing.T) {
	its := setStopFixture()
	its.Spec.EnableInstanceAPI = ptr.To(true)
	its.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "local"}
	its.Spec.InstanceAssistantObjects = []corev1.ObjectReference{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "demo-config"}}
	cli := newSetStopClient(t, its)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo-config", Namespace: "default"}, Data: map[string]string{"version": "v1"}}
	if err := cli.Create(context.Background(), cm); err != nil {
		t.Fatal(err)
	}
	reconcileSetStop(t, cli, false, 5)
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	reconcileSetStop(t, cli, false, 8)
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cm), cm); err != nil {
		t.Fatal(err)
	}
	cm.Data["version"] = "v2"
	if err := cli.Update(context.Background(), cm); err != nil {
		t.Fatal(err)
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(3)) })
	reconcileSetStop(t, cli, false, 4)
	children := listSetStopChildren(t, cli)
	if len(children) != 2 || len(listSetStopPods(t, cli)) != 0 {
		t.Fatal("Stop did not defer topology and runtime creation")
	}
	for _, child := range children {
		if !ptr.Deref(child.Spec.Stop, false) || len(child.Spec.InstanceAssistantObjects) != 1 || child.Spec.InstanceAssistantObjects[0].ConfigMap.Data["version"] != "v2" {
			t.Fatal("stopped clone did not receive desired assistant before name alignment")
		}
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cm), cm); err != nil {
		t.Fatal(err)
	}
	if cm.Data["version"] != "v2" {
		t.Fatal("stopped child assistant reconciliation overwrote current desired content")
	}
}

func TestInstanceSetStopRuntimeSummaryRequiresFreshAbsence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    workloads.InstanceCurrentState
		stale    bool
		replicas int32
	}{
		{name: "fresh absent", state: workloads.InstanceCurrentStateAbsent},
		{name: "stale absent", state: workloads.InstanceCurrentStateAbsent, stale: true, replicas: 1},
		{name: "unknown", stale: true, replicas: 1},
		{name: "terminating", state: workloads.InstanceCurrentStateTerminating, replicas: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			its := setStopFixture()
			its.Spec.Replicas = ptr.To(int32(1))
			its.Spec.EnableInstanceAPI = ptr.To(true)
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, false, 5)
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			reconcileSetStop(t, cli, false, 8)
			child := listSetStopChildren(t, cli)[0]
			child.Status.CurrentState = tc.state
			child.Status.ObservedGeneration = child.Generation
			if tc.stale {
				child.Status.ObservedGeneration--
			}
			child.Status.Ready = true
			child.Status.Available = true
			child.Status.UpToDate = true
			child.Status.Conditions = []metav1.Condition{{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue}, {Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue}, {Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue}}
			if err := cli.Status().Update(context.Background(), &child); err != nil {
				t.Fatal(err)
			}
			if _, err := reconcileSetStopParent(cli, false); err != nil {
				t.Fatal(err)
			}
			got := readSetStop(t, cli)
			if got.Status.Replicas != tc.replicas || got.Status.ReadyReplicas != 0 || got.Status.AvailableReplicas != 0 || got.Status.CurrentReplicas != 0 || got.Status.UpdatedReplicas != 0 {
				t.Fatalf("runtime summary contradicts child observation: %#v", got.Status)
			}
		})
	}
}

func TestInstanceSetStopBypassesMinReadyStatusCutoff(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(map[bool]string{true: "legacy", false: "instances"}[legacy], func(t *testing.T) {
			its := setStopFixture()
			its.Spec.EnableInstanceAPI = ptr.To(!legacy)
			its.Spec.MinReadySeconds = 3600
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, legacy, 5)
			for _, pod := range listSetStopPods(t, cli) {
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
				if err := cli.Status().Update(context.Background(), &pod); err != nil {
					t.Fatal(err)
				}
			}
			reconcileSetStop(t, cli, legacy, 3)
			got := readSetStop(t, cli)
			if got.Status.ReadyReplicas != 2 || got.Status.AvailableReplicas != 0 {
				t.Fatalf("fixture did not exercise the MinReadySeconds cutoff: %#v", got.Status)
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
			})
			reconcileSetStop(t, cli, legacy, 10)
			if len(listSetStopPods(t, cli)) != 0 || readSetStop(t, cli).Status.Replicas != 0 {
				t.Fatal("MinReadySeconds status cutoff prevented ordered Stop")
			}
		})
	}
}

func reconcileSetStopParent(cli client.Client, legacy bool) (ctrl.Result, error) {
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo"}}
	recorder := record.NewFakeRecorder(1000)
	if legacy {
		return (&InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: recorder}).Reconcile(ctx, req)
	}
	return (&InstanceSetReconciler2{Client: cli, Scheme: cli.Scheme(), Recorder: recorder}).Reconcile(ctx, req)
}

func readSetStop(t *testing.T, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func mutateSetStop(t *testing.T, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := readSetStop(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func listSetStopPods(t *testing.T, cli client.Client) []corev1.Pod {
	t.Helper()
	list := &corev1.PodList{}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func listSetStopChildren(t *testing.T, cli client.Client) []workloads.Instance {
	t.Helper()
	list := &workloads.InstanceList{}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func TestInstanceSetStopResumesLatestPayload(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(map[bool]string{true: "legacy", false: "instances"}[legacy], func(t *testing.T) {
			its := setStopFixture()
			its.Spec.EnableInstanceAPI = ptr.To(!legacy)
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, legacy, 5)
			if len(listSetStopPods(t, cli)) != 2 {
				t.Fatal("running setup did not create both Pods")
			}
			claims := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), claims); err != nil {
				t.Fatal(err)
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.MinReadySeconds = 3600
				its.Spec.PodUpdatePolicy = apps.StrictInPlacePodUpdatePolicyType
				its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: apps.OnDeleteStrategyType}
			})
			reconcileSetStop(t, cli, legacy, 8)
			if len(listSetStopPods(t, cli)) != 0 {
				t.Fatal("Stop was blocked by ordinary update gates")
			}
			stopped := readSetStop(t, cli)
			if stopped.Status.Replicas != 0 || stopped.Status.CurrentReplicas != 0 || stopped.Status.UpdatedReplicas != 0 {
				t.Fatalf("retained identities counted as runtime: %#v", stopped.Status)
			}
			beforeRevision := stopped.Status.UpdateRevision
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			reconcileSetStop(t, cli, legacy, 3)
			if readSetStop(t, cli).Status.UpdateRevision != beforeRevision {
				t.Fatal("Stop changed revision")
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.Containers[0].Image = "db:v2"
				its.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{Name: "db", ContainerPort: 5432, Protocol: corev1.ProtocolTCP}}
				its.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
				added := *its.Spec.VolumeClaimTemplates[0].DeepCopy()
				added.Name = "logs"
				its.Spec.VolumeClaimTemplates = append(its.Spec.VolumeClaimTemplates, added)
			})
			reconcileSetStop(t, cli, legacy, 4)
			service := &corev1.Service{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-headless"}, service); err != nil {
				t.Fatal(err)
			}
			if len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != 5432 {
				t.Fatal("stopped assistant Service did not receive latest desired ports")
			}
			frozen := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), frozen); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(claims.Items, frozen.Items) {
				t.Fatal("stopped PVCs changed")
			}
			if !legacy {
				for _, child := range listSetStopChildren(t, cli) {
					if !ptr.Deref(child.Spec.Stop, false) || child.Spec.Template.Spec.Containers[0].Image != "db:v2" || len(child.Spec.VolumeClaimTemplates) != 2 {
						t.Fatal("stopped child did not receive latest payload")
					}
				}
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = nil })
			reconcileSetStop(t, cli, legacy, 6)
			pods := listSetStopPods(t, cli)
			if len(pods) != 2 {
				t.Fatalf("resume created %d Pods", len(pods))
			}
			for _, pod := range pods {
				if pod.Spec.Containers[0].Image != "db:v2" {
					t.Fatal("resume used obsolete image")
				}
			}
			claims = &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), claims); err != nil {
				t.Fatal(err)
			}
			if len(claims.Items) != 4 {
				t.Fatal("resume did not create newly desired claims")
			}
			for _, claim := range claims.Items {
				if claim.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
					t.Fatal("resume ignored current storage target")
				}
			}
		})
	}
}

func TestInstanceSetStopScaleRetirement(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		for _, policy := range []apps.PersistentVolumeClaimRetentionPolicyType{apps.DeletePersistentVolumeClaimRetentionPolicyType, apps.RetainPersistentVolumeClaimRetentionPolicyType} {
			t.Run(fmt.Sprintf("legacy=%v/policy=%s", legacy, policy), func(t *testing.T) {
				its := setStopFixture()
				its.Spec.EnableInstanceAPI = ptr.To(!legacy)
				its.Spec.PersistentVolumeClaimRetentionPolicy = &apps.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy, WhenDeleted: apps.DeletePersistentVolumeClaimRetentionPolicyType}
				cli := newSetStopClient(t, its)
				reconcileSetStop(t, cli, legacy, 5)
				mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true); its.Spec.Replicas = ptr.To(int32(0)) })
				reconcileSetStop(t, cli, legacy, 12)
				if len(listSetStopPods(t, cli)) != 0 {
					t.Fatal("simultaneous Stop and scale zero did not drain")
				}
				claims := &corev1.PersistentVolumeClaimList{}
				if err := cli.List(context.Background(), claims); err != nil {
					t.Fatal(err)
				}
				if len(claims.Items) != 2 {
					t.Fatal("Stop retired storage")
				}
				mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
				reconcileSetStop(t, cli, legacy, 8)
				if len(listSetStopChildren(t, cli)) != 0 {
					t.Fatal("absent retirement was blocked by availability")
				}
				claims = &corev1.PersistentVolumeClaimList{}
				if err := cli.List(context.Background(), claims); err != nil {
					t.Fatal(err)
				}
				expected := 0
				if policy == apps.RetainPersistentVolumeClaimRetentionPolicyType {
					expected = 2
				}
				if len(claims.Items) != expected {
					t.Fatalf("retired claim count %d, want %d", len(claims.Items), expected)
				}
			})
		}
	}
}

func TestInstanceSetStopOrderedWaitsForHeldRuntime(t *testing.T) {
	its := setStopFixture()
	its.Spec.EnableInstanceAPI = ptr.To(true)
	cli := newSetStopClient(t, its)
	reconcileSetStop(t, cli, false, 5)
	held := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-1"}, held); err != nil {
		t.Fatal(err)
	}
	held.Finalizers = []string{"test/hold"}
	if err := cli.Update(context.Background(), held); err != nil {
		t.Fatal(err)
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
		its.Spec.Stop = ptr.To(true)
	})
	reconcileSetStop(t, cli, false, 5)
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) != (child.Name == "demo-1") {
			t.Fatal("ordered Stop passed a held higher ordinal")
		}
	}
	result, err := reconcileSetStopParent(cli, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("pending held Stop has no bounded retry: %#v", result)
	}
	if readSetStop(t, cli).Status.Replicas != 2 {
		t.Fatal("terminating runtime was lost from replica count")
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(false)
		its.Spec.Template.Spec.Containers[0].Image = "db:v3"
	})
	reconcileSetStop(t, cli, false, 3)
	for _, child := range listSetStopChildren(t, cli) {
		if child.Name == "demo-1" && !ptr.Deref(child.Spec.Stop, false) {
			t.Fatal("early resume released a terminating child")
		}
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(held), held); err != nil {
		t.Fatal(err)
	}
	held.Finalizers = nil
	if err := cli.Update(context.Background(), held); err != nil {
		t.Fatal(err)
	}
	reconcileSetStop(t, cli, false, 4)
	for _, child := range listSetStopChildren(t, cli) {
		if child.Name == "demo-1" && !ptr.Deref(child.Spec.Stop, false) {
			t.Fatal("ordered resume passed unavailable predecessor")
		}
	}
}

func TestInstanceSetStopParallelPartialWriteKeepsBudget(t *testing.T) {
	its := setStopFixture()
	its.Spec.Replicas = ptr.To(int32(3))
	its.Spec.EnableInstanceAPI = ptr.To(true)
	cli := newSetStopClient(t, its)
	reconcileSetStop(t, cli, false, 5)
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(true)
		its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(2))
	})
	wrapped := cli.(*setStopClient)
	wrapped.childWrites = 0
	wrapped.failChildWriteAt = 2
	if _, err := reconcileSetStopParent(cli, false); err == nil {
		t.Fatal("injected failure was not reached")
	}
	wrapped.failChildWriteAt = 0
	if _, err := reconcileSetStopParent(cli, false); err != nil {
		t.Fatal(err)
	}
	stopped := 0
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) {
			stopped++
			if child.Generation == child.Status.ObservedGeneration {
				t.Fatal("child spec write did not invalidate stale observations")
			}
		}
	}
	if stopped != 2 {
		t.Fatalf("partial Stop handoff admitted %d children into a 2-slot budget", stopped)
	}
	if _, err := reconcileSetStopParent(cli, false); err != nil {
		t.Fatal(err)
	}
	stopped = 0
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) {
			stopped++
		}
	}
	if stopped != 2 {
		t.Fatal("unobserved Stop requests released slots")
	}
	reconcileSetStop(t, cli, false, 8)
	if len(listSetStopPods(t, cli)) != 0 {
		t.Fatal("partial Stop did not recover")
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(false)
		its.Spec.Template.Spec.Containers[0].Image = "db:latest"
	})
	wrapped.resumeWrites = 0
	wrapped.failResumeWriteAt = 2
	if _, err := reconcileSetStopParent(cli, false); err == nil {
		t.Fatal("partial resume failure was not reached")
	}
	wrapped.failResumeWriteAt = 0
	if _, err := reconcileSetStopParent(cli, false); err != nil {
		t.Fatal(err)
	}
	resumed := 0
	for _, child := range listSetStopChildren(t, cli) {
		if !ptr.Deref(child.Spec.Stop, false) {
			resumed++
			if child.Spec.Template.Spec.Containers[0].Image != "db:latest" {
				t.Fatal("resume did not atomically deliver latest spec")
			}
		}
	}
	if resumed < 1 || resumed > 2 {
		t.Fatalf("partial resume admitted %d children into a 2-slot budget", resumed)
	}
	for _, child := range listSetStopChildren(t, cli) {
		if !ptr.Deref(child.Spec.Stop, false) {
			continue
		}
		for i := 0; i < 2; i++ {
			_, err := (&InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&child)})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := reconcileSetStopParent(cli, false); err != nil {
		t.Fatal(err)
	}
	resumed = 0
	for _, child := range listSetStopChildren(t, cli) {
		if !ptr.Deref(child.Spec.Stop, false) {
			resumed++
		}
	}
	if resumed != 2 {
		t.Fatalf("resume admitted %d children after fresh dormant observations, want 2", resumed)
	}
	if _, err := reconcileSetStopParent(cli, false); err != nil {
		t.Fatal(err)
	}
	resumed = 0
	for _, child := range listSetStopChildren(t, cli) {
		if !ptr.Deref(child.Spec.Stop, false) {
			resumed++
		}
	}
	if resumed != 2 {
		t.Fatal("stale resumed availability released slots")
	}
	reconcileSetStop(t, cli, false, 3)
	for _, pod := range listSetStopPods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:latest", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	reconcileSetStop(t, cli, false, 5)
	pods := listSetStopPods(t, cli)
	if len(pods) != 3 || readSetStop(t, cli).Status.Replicas != 3 {
		t.Fatal("partial resume did not converge after admitted runtimes became available")
	}
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) || child.Spec.Template.Spec.Containers[0].Image != "db:latest" {
			t.Fatal("partial resume left a stopped or obsolete child")
		}
	}
	for _, pod := range pods {
		if pod.Spec.Containers[0].Image != "db:latest" {
			t.Fatal("partial resume recreated an obsolete runtime")
		}
	}
}

func TestInstanceSetStopDeletionHandsOffLatestPolicy(t *testing.T) {
	its := setStopFixture()
	its.Spec.EnableInstanceAPI = ptr.To(true)
	its.Spec.PersistentVolumeClaimRetentionPolicy = &apps.PersistentVolumeClaimRetentionPolicy{WhenDeleted: apps.RetainPersistentVolumeClaimRetentionPolicyType}
	cli := newSetStopClient(t, its)
	reconcileSetStop(t, cli, false, 5)
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	reconcileSetStop(t, cli, false, 8)
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = apps.DeletePersistentVolumeClaimRetentionPolicyType
	})
	its = readSetStop(t, cli)
	if err := cli.Delete(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	result, err := reconcileSetStopParent(cli, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("policy handoff has no bounded retry: %#v", result)
	}
	for _, child := range listSetStopChildren(t, cli) {
		if !child.DeletionTimestamp.IsZero() || child.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted != apps.DeletePersistentVolumeClaimRetentionPolicyType {
			t.Fatal("parent deletion did not persist policy before deleting child")
		}
	}
	reconcileSetStop(t, cli, false, 8)
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Items) != 0 {
		t.Fatal("actual parent deletion used obsolete retention policy")
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(its), &workloads.InstanceSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("parent deletion did not complete: %v", err)
	}
}

func newSetStopClient(t *testing.T, its *workloads.InstanceSet) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(workloads.AddToScheme)
	return &setStopClient{Client: fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()}
}

func setStopFixture() *workloads.InstanceSet {
	inst := stopTestInstance()
	return &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "set-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
		Replicas: ptr.To(int32(2)), Selector: inst.Spec.Selector, Template: inst.Spec.Template, VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: inst.Spec.VolumeClaimTemplates[0].ObjectMeta, Spec: inst.Spec.VolumeClaimTemplates[0].Spec}},
		PodManagementPolicy: appsv1.ParallelPodManagement,
	}}
}

func reconcileSetStop(t *testing.T, cli client.Client, legacy bool, count int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		_, err := reconcileSetStopParent(cli, legacy)
		if err != nil {
			t.Fatal(err)
		}
		if !legacy {
			list := &workloads.InstanceList{}
			if err := cli.List(ctx, list); err != nil {
				t.Fatal(err)
			}
			for _, inst := range list.Items {
				_, err = (&InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&inst)})
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestInstanceSetStopInitiallyStopped(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(map[bool]string{true: "legacy", false: "instances"}[legacy], func(t *testing.T) {
			its := setStopFixture()
			its.Spec.Stop = ptr.To(true)
			its.Spec.EnableInstanceAPI = ptr.To(!legacy)
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, legacy, 5)
			pods := &corev1.PodList{}
			pvcs := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), pods); err != nil {
				t.Fatal(err)
			}
			if err := cli.List(context.Background(), pvcs); err != nil {
				t.Fatal(err)
			}
			if len(pods.Items) != 0 || len(pvcs.Items) != 0 {
				t.Fatalf("initial Stop provisioned %d Pods and %d PVCs", len(pods.Items), len(pvcs.Items))
			}
		})
	}
}
