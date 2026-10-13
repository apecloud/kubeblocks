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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func retentionFixture(t *testing.T, replicas int32) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "set-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
		EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(replicas), PodManagementPolicy: appsv1.ParallelPodManagement,
		Selector:                             &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
		Template:                             corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
		VolumeClaimTemplates:                 []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
		PersistentVolumeClaimRetentionPolicy: &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	runRetentionControllers(t, cli, 5)
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != int(replicas) {
		t.Fatalf("fixture has %d Pods, want %d", len(pods.Items), replicas)
	}
	for _, pod := range pods.Items {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Items) != int(replicas) {
		t.Fatalf("fixture has %d PVCs, want %d", len(claims.Items), replicas)
	}
	for _, claim := range claims.Items {
		claim.Status.Phase = corev1.ClaimBound
		claim.Status.Capacity = claim.Spec.Resources.Requests.DeepCopy()
		if err := cli.Status().Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	runRetentionControllers(t, cli, 3)
	return cli
}

func reconcileRetentionSet(cli client.Client) (ctrl.Result, error) {
	return (&InstanceSetReconciler2{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo"}})
}

func retentionSet(t *testing.T, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func changeRetentionSet(t *testing.T, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := retentionSet(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func retentionInstances(t *testing.T, cli client.Client) []workloads.Instance {
	t.Helper()
	list := &workloads.InstanceList{}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func runRetentionControllers(t *testing.T, cli client.Client, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		if _, err := reconcileRetentionSet(cli); err != nil {
			t.Fatal(err)
		}
		for _, inst := range retentionInstances(t, cli) {
			if _, err := (&InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&inst)}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func checkRetentionDeleted(t *testing.T, cli client.Client, wantClaims int) {
	t.Helper()
	runRetentionControllers(t, cli, 12)
	pods := &corev1.PodList{}
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 0 || len(retentionInstances(t, cli)) != 0 {
		t.Fatal("Instance and Pod deletion did not converge")
	}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Items) != wantClaims {
		t.Fatalf("PVCs after deletion = %d, want %d", len(claims.Items), wantClaims)
	}
}

func TestInstanceRetentionPolicyChange(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		for _, retain := range []bool{false, true} {
			t.Run(fmt.Sprintf("InstanceSetDeletion=%t/Retain=%t", deleting, retain), func(t *testing.T) {
				cli := retentionFixture(t, 2)
				old, current := kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType
				want := 0
				if retain {
					old, current, want = current, old, 2
				}
				changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: old, WhenDeleted: old}
				})
				runRetentionControllers(t, cli, 4)
				changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
					if deleting {
						its.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = current
					} else {
						its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = current
						its.Spec.Replicas = ptr.To(int32(0))
					}
				})
				if deleting {
					if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
						t.Fatal(err)
					}
				}
				checkRetentionDeleted(t, cli, want)
			})
		}
	}
}

func TestInstanceRetentionScaleReversal(t *testing.T) {
	cli := retentionFixture(t, 2)
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(1)) })
	if _, err := reconcileRetentionSet(cli); err != nil {
		t.Fatal(err)
	}
	inst := &workloads.Instance{}
	key := client.ObjectKey{Namespace: "default", Name: "demo-1"}
	if err := cli.Get(context.Background(), key, inst); err != nil {
		t.Fatal(err)
	}
	if !ptr.Deref(inst.Spec.ScaledDown, false) || !inst.DeletionTimestamp.IsZero() {
		t.Fatal("scale-down did not persist the intermediate spec.scaledDown state")
	}
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(2)) })
	runRetentionControllers(t, cli, 3)
	if err := cli.Get(context.Background(), key, inst); err != nil {
		t.Fatal(err)
	}
	if inst.Status.CurrentState != workloads.InstanceCurrentStatePresent {
		t.Fatal("Instance did not return to Present")
	}
	if ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Error("desired Instance still has spec.scaledDown=true")
	}
	if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
		t.Fatal(err)
	}
	checkRetentionDeleted(t, cli, 0)
}

type retentionWriteFailureClient struct {
	client.Client
	writes  int
	failure error
}

func (c *retentionWriteFailureClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if inst, ok := obj.(*workloads.Instance); ok {
		old := &workloads.Instance{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(inst), old); err != nil {
			return err
		}
		if !reflect.DeepEqual(old.Spec, inst.Spec) {
			c.writes++
			if c.writes == 2 {
				if apierrors.IsConflict(c.failure) {
					if old.Annotations == nil {
						old.Annotations = map[string]string{}
					}
					old.Annotations["test/concurrent-update"] = "preserved"
					if err := c.Client.Update(ctx, old); err != nil {
						return err
					}
				}
				return c.failure
			}
		}
	}
	return c.Client.Update(ctx, obj, opts...)
}

func TestInstanceRetentionPartialHandoff(t *testing.T) {
	writeFailure := errors.New("injected Instance update failure")
	conflict := apierrors.NewConflict(schema.GroupResource{Group: workloads.GroupVersion.Group, Resource: "instances"}, "demo", errors.New("concurrent update"))
	for _, deleting := range []bool{false, true} {
		for _, failure := range []error{writeFailure, conflict} {
			t.Run(fmt.Sprintf("InstanceSetDeletion=%t/Conflict=%t", deleting, apierrors.IsConflict(failure)), func(t *testing.T) {
				cli := retentionFixture(t, 4)
				changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType
				})
				runRetentionControllers(t, cli, 4)
				changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
					if !deleting {
						its.Spec.Replicas = ptr.To(int32(2))
					}
				})
				if deleting {
					if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
						t.Fatal(err)
					}
				}
				faulty := &retentionWriteFailureClient{Client: cli, failure: failure}
				result, err := reconcileRetentionSet(faulty)
				if apierrors.IsConflict(failure) {
					if err != nil || !result.Requeue {
						t.Fatalf("conflict must retry, result=%#v err=%v", result, err)
					}
				} else if !errors.Is(err, failure) {
					t.Fatalf("expected injected error, got %v", err)
				}
				updated := 0
				instances := retentionInstances(t, cli)
				if len(instances) != 4 {
					t.Fatal("partial handoff deleted an Instance")
				}
				for _, inst := range instances {
					if !inst.DeletionTimestamp.IsZero() {
						t.Fatal("partial handoff began Instance deletion")
					}
					if inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled == kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType {
						updated++
						if !deleting && !ptr.Deref(inst.Spec.ScaledDown, false) {
							t.Fatal("scale-down policy and marker were not persisted together")
						}
					}
				}
				if updated != 1 {
					t.Fatalf("partial handoff persisted %d policies, want 1", updated)
				}
				if deleting {
					result, err = reconcileRetentionSet(cli)
					if err != nil || result.RequeueAfter <= 0 {
						t.Fatalf("policy handoff must schedule another reconcile, result=%#v err=%v", result, err)
					}
					for _, inst := range retentionInstances(t, cli) {
						if !inst.DeletionTimestamp.IsZero() || inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted != kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType {
							t.Fatal("Instance deletion preceded persisted current policy")
						}
					}
					if apierrors.IsConflict(failure) {
						preserved := 0
						for _, inst := range retentionInstances(t, cli) {
							if inst.Annotations["test/concurrent-update"] == "preserved" {
								preserved++
							}
						}
						if preserved != 1 {
							t.Fatal("retry overwrote the concurrent Instance metadata update")
						}
					}
				}
				if !deleting {
					runRetentionControllers(t, cli, 8)
					if len(retentionInstances(t, cli)) != 2 {
						t.Fatal("partial scale-down did not converge")
					}
					changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
				}
				checkRetentionDeleted(t, cli, 0)
			})
		}
	}
}

func TestInstanceRetentionPartialScaleReversal(t *testing.T) {
	cli := retentionFixture(t, 4)
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(2)) })
	if _, err := reconcileRetentionSet(cli); err != nil {
		t.Fatal(err)
	}
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(4)) })
	failure := errors.New("injected scale reversal update failure")
	if _, err := reconcileRetentionSet(&retentionWriteFailureClient{Client: cli, failure: failure}); !errors.Is(err, failure) {
		t.Fatalf("expected partial reversal failure, got %v", err)
	}
	marked := 0
	for _, inst := range retentionInstances(t, cli) {
		if !inst.DeletionTimestamp.IsZero() {
			t.Fatal("scale reversal started Instance deletion")
		}
		if ptr.Deref(inst.Spec.ScaledDown, false) {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("partial scale reversal has %d marked Instances, want 1", marked)
	}
	runRetentionControllers(t, cli, 3)
	for _, inst := range retentionInstances(t, cli) {
		if ptr.Deref(inst.Spec.ScaledDown, false) {
			t.Fatal("scale reversal retry left spec.scaledDown=true")
		}
	}
	if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
		t.Fatal(err)
	}
	checkRetentionDeleted(t, cli, 0)
}

func TestInstanceRetentionDeletingInstanceKeepsPolicy(t *testing.T) {
	cli := retentionFixture(t, 2)
	pod := &corev1.Pod{}
	key := client.ObjectKey{Namespace: "default", Name: "demo-1"}
	if err := cli.Get(context.Background(), key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = append(pod.Finalizers, "test/hold")
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(1)) })
	runRetentionControllers(t, cli, 3)
	inst := &workloads.Instance{}
	if err := cli.Get(context.Background(), key, inst); err != nil {
		t.Fatal(err)
	}
	if inst.DeletionTimestamp.IsZero() || !ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Fatal("fixture did not begin scale-down deletion")
	}
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Replicas = ptr.To(int32(2))
		its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType
	})
	if _, err := reconcileRetentionSet(cli); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileRetentionSet(cli); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), key, inst); err != nil {
		t.Fatal(err)
	}
	if !ptr.Deref(inst.Spec.ScaledDown, false) || inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
		t.Fatal("changed the policy of an Instance whose deletion already began")
	}
	if err := cli.Get(context.Background(), key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = nil
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	checkRetentionDeleted(t, cli, 1)
}

func TestInstanceRetentionScaleDownAdmission(t *testing.T) {
	for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
		t.Run(string(policy), func(t *testing.T) {
			cli := retentionFixture(t, 2)
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.PodManagementPolicy = policy
				its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
				its.Spec.Replicas = ptr.To(int32(0))
			})
			if _, err := reconcileRetentionSet(cli); err != nil {
				t.Fatal(err)
			}
			for _, inst := range retentionInstances(t, cli) {
				if ptr.Deref(inst.Spec.ScaledDown, false) != (inst.Name == "demo-1") || !inst.DeletionTimestamp.IsZero() {
					t.Fatalf("unexpected scale-down admission for %s", inst.Name)
				}
			}
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType
			})
			if _, err := reconcileRetentionSet(cli); err != nil {
				t.Fatal(err)
			}
			for _, inst := range retentionInstances(t, cli) {
				if !inst.DeletionTimestamp.IsZero() {
					t.Fatal("marked Instance deleted before persisting the changed policy")
				}
				if inst.Name == "demo-1" && inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled != kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType {
					t.Fatal("marked Instance kept the old policy")
				}
				if inst.Name == "demo-0" && ptr.Deref(inst.Spec.ScaledDown, false) {
					t.Fatal("policy handoff exceeded the scale-down budget")
				}
			}
			checkRetentionDeleted(t, cli, 0)
		})
	}
}
