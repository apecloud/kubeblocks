package workloads

import (
	"context"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func retentionFixture(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "set-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
		EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(int32(2)), PodManagementPolicy: appsv1.ParallelPodManagement,
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
	if len(pods.Items) != 2 {
		t.Fatalf("fixture has %d Pods, want 2", len(pods.Items))
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
	if len(claims.Items) != 2 {
		t.Fatalf("fixture has %d PVCs, want 2", len(claims.Items))
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
				cli := retentionFixture(t)
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
	cli := retentionFixture(t)
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
