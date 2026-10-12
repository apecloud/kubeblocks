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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func pvcLifecycleFixture(t *testing.T, replicas int32) client.WithWatch {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(workloads.AddToScheme)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-lifecycle", Namespace: "default", UID: "its-uid", Generation: 1,
			Annotations: map[string]string{constant.CRDAPIVersionAnnotationKey: workloads.GroupVersion.String()}},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas), PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "pvc-lifecycle"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "pvc-lifecycle"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	pvcLifecycleReconcile(t, cli, 4)
	if len(pvcLifecyclePods(t, cli)) != int(replicas) {
		t.Fatal("fixture did not provision running identities")
	}
	for _, claim := range pvcLifecycleClaims(t, cli) {
		claim.UID = types.UID(claim.Name + "-uid")
		claim.Spec.VolumeName = claim.Name + "-volume"
		if err := cli.Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	return cli
}

func pvcLifecycleReconcile(t *testing.T, cli client.Client, count int) ctrl.Result {
	t.Helper()
	var result ctrl.Result
	for i := 0; i < count; i++ {
		reconciler := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
		var err error
		result, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func pvcLifecycleSet(t *testing.T, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func pvcLifecycleChange(t *testing.T, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := pvcLifecycleSet(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func pvcLifecyclePods(t *testing.T, cli client.Client) []corev1.Pod {
	t.Helper()
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	return pods.Items
}

func pvcLifecycleClaims(t *testing.T, cli client.Client) []corev1.PersistentVolumeClaim {
	t.Helper()
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	return claims.Items
}

func pvcLifecycleReady(t *testing.T, cli client.Client) {
	t.Helper()
	for _, claim := range pvcLifecycleClaims(t, cli) {
		claim.Status.Capacity = claim.Spec.Resources.Requests.DeepCopy()
		if err := cli.Status().Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	for _, pod := range pvcLifecyclePods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.PodIP = "127.0.0.1"
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	pvcLifecycleReconcile(t, cli, 2)
}

func pvcLifecycleHold(t *testing.T, cli client.Client) {
	t.Helper()
	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle-0"}, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = append(pod.Finalizers, "test/hold")
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceSetRetainedClaimsAfterRecreation(t *testing.T) {
	cli := pvcLifecycleFixture(t, 2)
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{
			WhenDeleted: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
		}
	})
	extra := pvcLifecycleClaims(t, cli)[0].DeepCopy()
	extra.Name = "retained-" + extra.Name
	extra.ResourceVersion = ""
	if err := cli.Create(context.Background(), extra); err != nil {
		t.Fatal(err)
	}
	original := pvcLifecycleSet(t, cli)
	if err := cli.Delete(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 6)
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(original), &workloads.InstanceSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("original InstanceSet was not deleted: %v", err)
	}
	claims := pvcLifecycleClaims(t, cli)
	if len(claims) != 3 {
		t.Fatal("root deletion did not retain all claims")
	}
	for _, claim := range claims {
		if len(claim.OwnerReferences) != 0 {
			t.Fatal("retained claim was not detached from the deleted root")
		}
	}
	replacement := original.DeepCopy()
	replacement.ResourceVersion = ""
	replacement.UID = "replacement-root"
	replacement.Generation = 1
	replacement.Finalizers = nil
	replacement.Status = workloads.InstanceSetStatus{}
	replacement.Spec.Replicas = ptr.To(int32(0))
	if err := cli.Create(context.Background(), replacement); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 4)
	if !reflect.DeepEqual(claims, pvcLifecycleClaims(t, cli)) {
		t.Fatal("same-name recreation changed unadopted retained claims")
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(1)) })
	pvcLifecycleReconcile(t, cli, 4)
	for _, claim := range pvcLifecycleClaims(t, cli) {
		switch claim.Name {
		case "data-pvc-lifecycle-0":
			if !metav1.IsControlledBy(&claim, replacement) || !reflect.DeepEqual(claim.Spec, claims[0].Spec) {
				t.Fatal("active claim was not adopted with its storage preserved")
			}
		case extra.Name:
			if !reflect.DeepEqual(claim, claims[2]) {
				t.Fatal("adoption changed the detached sibling claim")
			}
		default:
			if !reflect.DeepEqual(claim, claims[1]) {
				t.Fatal("adoption changed the unused retained claim")
			}
		}
	}
	pods := pvcLifecyclePods(t, cli)
	if len(pods) != 1 {
		t.Fatal("replacement did not create its active Pod")
	}
	if err := cli.Delete(context.Background(), &pods[0]); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
	pvcLifecycleReconcile(t, cli, 4)
	if remaining := pvcLifecycleClaims(t, cli); !reflect.DeepEqual(remaining, claims[1:]) {
		t.Fatal("retirement must remove only the adopted claim and preserve unowned storage")
	}
}

func TestInstanceSetPVCAllocationAndRetirement(t *testing.T) {
	cases := []struct {
		policy   kbappsv1.PersistentVolumeClaimRetentionPolicyType
		replicas int32
	}{
		{kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, 0},
		{kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, 1},
	}
	for _, tc := range cases {
		policy, replicas := tc.policy, tc.replicas
		t.Run(fmt.Sprintf("%s/replicas=%d", policy, replicas), func(t *testing.T) {
			cli := pvcLifecycleFixture(t, 2)
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.Replicas = ptr.To(replicas)
				its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
			})
			pvcLifecycleReconcile(t, cli, 6)
			claims := pvcLifecycleClaims(t, cli)
			if len(pvcLifecyclePods(t, cli)) != 0 || len(claims) != 2 {
				t.Fatal("simultaneous Stop and scale-down did not retain both claims while draining")
			}
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
			pvcLifecycleReconcile(t, cli, 3)
			if len(pvcLifecyclePods(t, cli)) != int(replicas) {
				t.Fatal("resume did not restore exactly the desired identities")
			}
			if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
				if !reflect.DeepEqual(claims, pvcLifecycleClaims(t, cli)) {
					t.Fatal("WhenScaled=Retain changed stopped claims")
				}
			} else {
				for _, claim := range claims {
					current := &corev1.PersistentVolumeClaim{}
					err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current)
					if !apierrors.IsNotFound(err) {
						t.Fatalf("WhenScaled=Delete retained absent retired identity claim: %v", err)
					}
				}
			}
		})
	}
	t.Run("Offline", func(t *testing.T) {
		cli := pvcLifecycleFixture(t, 3)
		pvcLifecycleReady(t, cli)
		pvcLifecycleHold(t, cli)
		pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
		pvcLifecycleReconcile(t, cli, 6)
		pods := pvcLifecyclePods(t, cli)
		if len(pods) != 1 || pods[0].Name != "pvc-lifecycle-0" || pods[0].DeletionTimestamp.IsZero() || len(pvcLifecycleClaims(t, cli)) != 3 {
			t.Fatal("Stop did not leave the held Pod while retaining all claims")
		}
		held := pods[0]
		offline := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "data-pvc-lifecycle-0"}, offline); err != nil {
			t.Fatal(err)
		}
		pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
			its.Spec.OfflineInstances = []string{"pvc-lifecycle-0"}
			its.Spec.Replicas = ptr.To(int32(1))
			its.Spec.Stop = ptr.To(false)
		})
		pvcLifecycleReconcile(t, cli, 1)
		pods = pvcLifecyclePods(t, cli)
		if len(pods) != 2 {
			t.Fatalf("early resume did not create the independent Active identity: %#v", pods)
		}
		for _, pod := range pods {
			switch {
			case pod.Name == held.Name:
				if !reflect.DeepEqual(held, pod) {
					t.Fatal("early resume changed the held Offline Pod")
				}
			case pod.Name != "pvc-lifecycle-1" || !pod.DeletionTimestamp.IsZero():
				t.Fatalf("early resume created an unexpected Active identity: %#v", pod)
			default:
				pod.Status = held.Status
				if err := cli.Status().Update(context.Background(), &pod); err != nil {
					t.Fatal(err)
				}
			}
		}
		pvcLifecycleReconcile(t, cli, 2)
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(offline), current); err != nil || !reflect.DeepEqual(offline, current) {
			t.Fatalf("resume changed Offline storage: %v", err)
		}
		if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "data-pvc-lifecycle-2"}, current); !apierrors.IsNotFound(err) {
			t.Fatalf("resume retained a retired identity's claim: %v", err)
		}
		if len(pvcLifecycleClaims(t, cli)) != 2 {
			t.Fatal("resume did not retain exactly the Active and Offline claims")
		}
		held.Finalizers = nil
		if err := cli.Update(context.Background(), &held); err != nil {
			t.Fatal(err)
		}
		pvcLifecycleReconcile(t, cli, 3)
		if pods := pvcLifecyclePods(t, cli); len(pods) != 1 || pods[0].Name != "pvc-lifecycle-1" {
			t.Fatalf("resume did not preserve the Active identity after Offline termination: %#v", pods)
		}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(offline), current); err != nil || !reflect.DeepEqual(offline, current) {
			t.Fatalf("absent Offline storage changed: %v", err)
		}
		pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
			its.Spec.OfflineInstances = nil
			its.Spec.Replicas = ptr.To(int32(0))
		})
		pvcLifecycleReconcile(t, cli, 3)
		if len(pvcLifecyclePods(t, cli)) != 0 || len(pvcLifecycleClaims(t, cli)) != 0 {
			t.Fatal("removing the Offline allocation did not retire its claim")
		}
	})
	t.Run("LiveOffline", func(t *testing.T) {
		cli := pvcLifecycleFixture(t, 1)
		pvcLifecycleReady(t, cli)
		offline := pvcLifecycleClaims(t, cli)[0]
		ready := pvcLifecyclePods(t, cli)[0].Status
		pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
			its.Spec.OfflineInstances = []string{"pvc-lifecycle-0"}
		})
		pvcLifecycleReconcile(t, cli, 1)
		active := &corev1.Pod{}
		if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle-1"}, active); err != nil {
			t.Fatal(err)
		}
		active.Status = ready
		if err := cli.Status().Update(context.Background(), active); err != nil {
			t.Fatal(err)
		}
		pvcLifecycleReconcile(t, cli, 2)
		if pods := pvcLifecyclePods(t, cli); len(pods) != 1 || pods[0].Name != "pvc-lifecycle-1" {
			t.Fatalf("live Offline transition did not replace the Pod: %#v", pods)
		}
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&offline), current); err != nil || !reflect.DeepEqual(offline, *current) {
			t.Fatalf("live Offline retirement changed allocated storage: %v", err)
		}
	})
}

func TestInstanceSetPVCRetirementAfterPartialCommit(t *testing.T) {
	cli := pvcLifecycleFixture(t, 4)
	for _, pod := range pvcLifecyclePods(t, cli) {
		if err := cli.Delete(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	allocated := []corev1.PersistentVolumeClaim{}
	for _, name := range []string{"data-pvc-lifecycle-0", "data-pvc-lifecycle-3"} {
		claim := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, claim); err != nil {
			t.Fatal(err)
		}
		allocated = append(allocated, *claim)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.OfflineInstances = []string{"pvc-lifecycle-3"}
		its.Spec.Replicas = ptr.To(int32(1))
	})
	failure := errors.New("injected retired PVC delete failure")
	fired, attempts, deleted := false, 0, 0
	faulty := interceptor.NewClient(cli, interceptor.Funcs{
		Delete: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				attempts++
				if !fired && attempts == 2 {
					fired = true
					return failure
				}
				err := delegate.Delete(ctx, obj, opts...)
				if err == nil {
					deleted++
				}
				return err
			}
			return delegate.Delete(ctx, obj, opts...)
		},
	})
	controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	_, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}})
	if !errors.Is(err, failure) || !fired || attempts != 2 || deleted != 1 || len(pvcLifecycleClaims(t, cli)) != 3 || len(pvcLifecyclePods(t, cli)) != 0 {
		t.Fatalf("expected one persisted retired-claim deletion before failure, got %v", err)
	}
	for _, claim := range allocated {
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current); err != nil || !reflect.DeepEqual(claim, *current) {
			t.Fatal("partial retirement changed Active or Offline storage")
		}
	}
	pvcLifecycleReconcile(t, faulty, 5)
	if deleted != 2 || len(pvcLifecycleClaims(t, cli)) != 2 || len(pvcLifecyclePods(t, cli)) != 1 {
		t.Fatal("fresh retry did not complete retirement and resume")
	}
	for _, claim := range allocated {
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current); err != nil || !reflect.DeepEqual(claim, *current) {
			t.Fatal("retirement retry changed Active or Offline storage")
		}
	}
}

func TestInstanceSetPVCMixedRetirement(t *testing.T) {
	for _, tc := range []struct {
		management appsv1.PodManagementPolicyType
		policy     kbappsv1.PersistentVolumeClaimRetentionPolicyType
	}{
		{appsv1.OrderedReadyPodManagement, kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
		{appsv1.ParallelPodManagement, kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
		{appsv1.OrderedReadyPodManagement, kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType},
	} {
		t.Run(string(tc.management)+"/"+string(tc.policy), func(t *testing.T) {
			management, policy := tc.management, tc.policy
			cli := pvcLifecycleFixture(t, 4)
			pvcLifecycleReady(t, cli)
			orphan := &corev1.Pod{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle-3"}, orphan); err != nil {
				t.Fatal(err)
			}
			if err := cli.Delete(context.Background(), orphan); err != nil {
				t.Fatal(err)
			}
			claims := pvcLifecycleClaims(t, cli)
			unknown := claims[0].DeepCopy()
			unknown.Name = "unassigned-claim"
			unknown.ResourceVersion = ""
			unknown.UID = "unassigned-claim-uid"
			unknown.Labels[constant.KBAppPodNameLabelKey] = ""
			if err := cli.Create(context.Background(), unknown); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(unknown), unknown); err != nil {
				t.Fatal(err)
			}
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(1))
				its.Spec.PodManagementPolicy = management
				its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
				its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
			})
			pvcLifecycleReconcile(t, cli, 1)
			pods := pvcLifecyclePods(t, cli)
			if len(pods) != 2 {
				t.Fatalf("expected exactly one admitted Pod retirement, got %d", len(pods))
			}
			for _, pod := range pods {
				if pod.Name == "pvc-lifecycle-2" {
					t.Fatal("highest live ordinal was not retired")
				}
			}
			actual := map[string]corev1.PersistentVolumeClaim{}
			for _, claim := range pvcLifecycleClaims(t, cli) {
				actual[claim.Name] = claim
			}
			expectedClaims := 3
			if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
				expectedClaims = 5
			}
			if len(actual) != expectedClaims {
				t.Fatalf("expected %d claims after admitted retirement, got %d", expectedClaims, len(actual))
			}
			for _, claim := range claims {
				current, found := actual[claim.Name]
				retired := policy != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType && (claim.Name == "data-pvc-lifecycle-2" || claim.Name == "data-pvc-lifecycle-3")
				if retired {
					if found {
						t.Fatalf("retired claim survived: %s", claim.Name)
					}
				} else if !found || !reflect.DeepEqual(claim, current) {
					t.Fatalf("non-admitted or retained claim changed: %s", claim.Name)
				}
			}
			if current, found := actual[unknown.Name]; !found || !reflect.DeepEqual(*unknown, current) {
				t.Fatal("blank identity claim changed")
			}
		})
	}
}

func TestInstanceSetPVCAbsentOwnership(t *testing.T) {
	cli := pvcLifecycleFixture(t, 1)
	owned := pvcLifecycleClaims(t, cli)[0]
	claims := map[string]corev1.PersistentVolumeClaim{}
	for _, ownership := range []string{"detached", "previous-root", "noncontroller"} {
		claim := owned.DeepCopy()
		claim.Name = "foreign-" + ownership
		claim.ResourceVersion = ""
		switch ownership {
		case "detached":
			claim.OwnerReferences = nil
		case "previous-root":
			claim.OwnerReferences[0].UID = "previous-root"
		case "noncontroller":
			claim.OwnerReferences[0].Controller = ptr.To(false)
		}
		if err := cli.Create(context.Background(), claim); err != nil {
			t.Fatal(err)
		}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), claim); err != nil {
			t.Fatal(err)
		}
		claims[claim.Name] = *claim
	}
	pods := pvcLifecyclePods(t, cli)
	if err := cli.Delete(context.Background(), &pods[0]); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
	pvcLifecycleReconcile(t, cli, 4)
	actual := pvcLifecycleClaims(t, cli)
	if len(actual) != len(claims) {
		t.Fatalf("absent cleanup must remove only the current-root controlled claim, got %d claims", len(actual))
	}
	for _, claim := range actual {
		if !reflect.DeepEqual(claim, claims[claim.Name]) {
			t.Fatalf("absent cleanup changed foreign sibling claim %s", claim.Name)
		}
	}
}

func TestInstanceSetPVCIncompleteAllocation(t *testing.T) {
	cli := pvcLifecycleFixture(t, 2)
	pvcLifecycleReady(t, cli)
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.FlatInstanceOrdinal = true
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
		its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: "OnDelete"}
		its.Spec.Instances = []workloads.InstanceTemplate{
			{Name: "a", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}},
			{Name: "b", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{1}}},
		}
	})
	pvcLifecycleReconcile(t, cli, 4)
	pvcLifecycleReady(t, cli)
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Instances[0].Ordinals.Discrete = []int32{1}
		its.Spec.Instances[1].Ordinals.Discrete = []int32{0}
	})
	pvcLifecycleReconcile(t, cli, 1)
	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle-0"}, pod); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	claim := &corev1.PersistentVolumeClaim{}
	key := client.ObjectKey{Namespace: "default", Name: "data-pvc-lifecycle-0"}
	if err := cli.Get(context.Background(), key, claim); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 3)
	current := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), key, current); err != nil || !reflect.DeepEqual(claim, current) {
		t.Fatalf("allocation transfer changed absent Active storage: %v", err)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Replicas = ptr.To(int32(0))
		for i := range its.Spec.Instances {
			its.Spec.Instances[i].Replicas = ptr.To(int32(0))
		}
	})
	pvcLifecycleReconcile(t, cli, 4)
	if len(pvcLifecycleClaims(t, cli)) != 0 || len(pvcLifecyclePods(t, cli)) != 0 {
		t.Fatal("complete scale-down did not resume claim retirement")
	}
}
