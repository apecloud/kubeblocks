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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func stopTestInstance() *workloads.Instance {
	return &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", UID: "instance-uid", Generation: 1}, Spec: workloads.InstanceSpec{
		InstanceSetName: "demo", Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}}, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
	}}
}
func stopTestClient(t *testing.T, inst *workloads.Instance) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(workloads.AddToScheme)
	return fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&workloads.Instance{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(inst).Build()
}
func reconcileStopEntry(t *testing.T, cli client.Client, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		reconciler := &InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
		if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo-0"}}); err != nil {
			t.Fatal(err)
		}
	}
}
func readStopInstance(t *testing.T, cli client.Client) *workloads.Instance {
	t.Helper()
	inst := &workloads.Instance{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-0"}, inst); err != nil {
		t.Fatal(err)
	}
	return inst
}
func readStopPod(t *testing.T, cli client.Client) *corev1.Pod {
	t.Helper()
	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-0"}, pod); err != nil {
		t.Fatal(err)
	}
	return pod
}
func readStopPVC(t *testing.T, cli client.Client) *corev1.PersistentVolumeClaim {
	t.Helper()
	pvc := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "data-demo-0"}, pvc); err != nil {
		t.Fatal(err)
	}
	return pvc
}
func bindStopPVC(t *testing.T, cli client.Client) *corev1.PersistentVolumeClaim {
	t.Helper()
	pvc := readStopPVC(t, cli)
	pvc.UID = "pvc-uid"
	pvc.Spec.VolumeName = "pv-data"
	if err := cli.Update(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}
	if err := cli.Status().Update(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	return readStopPVC(t, cli)
}
func assertStopPodAbsent(t *testing.T, cli client.Client) {
	t.Helper()
	err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-0"}, &corev1.Pod{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Pod should be absent, got %v", err)
	}
}

func TestInstanceStopInitiallyStopped(t *testing.T) {
	inst := stopTestInstance()
	inst.Spec.Stop = ptr.To(true)
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 4)
	assertStopPodAbsent(t, cli)
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), pvcs); err != nil {
		t.Fatal(err)
	}
	if len(pvcs.Items) != 0 {
		t.Fatal("initial stop provisioned PVCs")
	}
	got := readStopInstance(t, cli)
	if got.Status.CurrentState != workloads.InstanceCurrentStateAbsent || got.Status.Pod != nil || got.Status.UpToDate || ptr.Deref(got.Spec.ScaledDown, false) {
		t.Fatalf("incorrect stopped status: %#v", got.Status)
	}
}

func TestInstanceStopSuspendsPVCsAndResumesLatestTemplate(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	pvc := bindStopPVC(t, cli)
	pod := readStopPod(t, cli)
	pod.UID = "pod-old"
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Spec.MinReadySeconds = 3600
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 1)
	assertStopPodAbsent(t, cli)
	observed := readStopInstance(t, cli)
	if observed.Status.CurrentState != workloads.InstanceCurrentStatePresent || observed.Status.Pod.UID != "pod-old" || observed.Status.UpToDate {
		t.Fatalf("stop guessed absent before observation: %#v", observed.Status)
	}
	reconcileStopEntry(t, cli, 2)
	before := readStopPVC(t, cli)
	got = readStopInstance(t, cli)
	got.Spec.Template.Spec.Containers[0].Image = "db:v2"
	got.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	got.Spec.VolumeClaimTemplates = append(got.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaimTemplate{ObjectMeta: metav1.ObjectMeta{Name: "logs"}, Spec: *got.Spec.VolumeClaimTemplates[0].Spec.DeepCopy()})
	got.Spec.InstanceAssistantObjects = []workloads.InstanceAssistantObject{{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo-env", Namespace: "default"}, Data: map[string]string{"version": "v2"}}}}
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 3)
	after := readStopPVC(t, cli)
	if after.UID != pvc.UID || after.ResourceVersion != before.ResourceVersion || len(after.OwnerReferences) != 1 || after.OwnerReferences[0].UID != inst.UID || after.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("1Gi")) != 0 {
		t.Fatalf("stop mutated PVC: %#v", after)
	}
	err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "logs-demo-0"}, &corev1.PersistentVolumeClaim{})
	if !apierrors.IsNotFound(err) {
		t.Fatal("stop created new claim")
	}
	cm := &corev1.ConfigMap{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-env"}, cm); err != nil {
		t.Fatal(err)
	}
	if cm.Data["version"] != "v2" {
		t.Fatal("assistant stopped reconciling")
	}
	got = readStopInstance(t, cli)
	got.Spec.VolumeClaimTemplates = nil
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	retained := readStopPVC(t, cli)
	if retained.UID != pvc.UID || retained.ResourceVersion != before.ResourceVersion {
		t.Fatal("removed template mutated retained PVC")
	}
	got = readStopInstance(t, cli)
	if got.Status.Storage == nil || len(got.Status.Storage.Volumes) != 1 || got.Status.Storage.Volumes[0].Claim.UID != pvc.UID || !got.Status.Storage.Complete {
		t.Fatalf("actual stopped storage missing: %#v", got.Status.Storage)
	}
	got.Spec.Stop = ptr.To(false)
	got.Spec.MinReadySeconds = 0
	got.Spec.VolumeClaimTemplates = inst.Spec.VolumeClaimTemplates
	got.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	resumed := readStopPod(t, cli)
	if resumed.Spec.Containers[0].Image != "db:v2" {
		t.Fatal("resume used old template")
	}
	if readStopPVC(t, cli).UID != pvc.UID || readStopPVC(t, cli).Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("resume replaced storage or ignored latest capacity")
	}
}

func TestInstanceStopWaitsForTerminatingPodOnEarlyResume(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	pod := readStopPod(t, cli)
	pod.UID = "pod-held"
	pod.Finalizers = []string{"test/hold"}
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 3)
	terminating := readStopPod(t, cli)
	if terminating.DeletionTimestamp.IsZero() {
		t.Fatal("stop did not request deletion")
	}
	got = readStopInstance(t, cli)
	if got.Status.CurrentState != workloads.InstanceCurrentStateTerminating || got.Status.Pod.UID != "pod-held" {
		t.Fatal("terminating status discarded actual identity")
	}
	got.Spec.Stop = ptr.To(false)
	got.Spec.Template.Spec.Containers[0].Image = "db:v3"
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	if readStopPod(t, cli).UID != "pod-held" || readStopPod(t, cli).Spec.Containers[0].Image != "db:v1" {
		t.Fatal("early resume replaced or modified terminating Pod")
	}
	terminating = readStopPod(t, cli)
	terminating.Finalizers = nil
	if err := cli.Update(context.Background(), terminating); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	if readStopPod(t, cli).Spec.Containers[0].Image != "db:v3" {
		t.Fatal("resume did not use current template")
	}
}

type failStopDeleteClient struct {
	client.Client
	remaining int
	deleted   int
}

func (c *failStopDeleteClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*corev1.Pod); ok && c.remaining > 0 {
		c.remaining--
		return errors.New("injected Pod delete failure")
	}
	err := c.Client.Delete(ctx, obj, opts...)
	if _, ok := obj.(*corev1.Pod); ok && err == nil {
		c.deleted++
	}
	return err
}
func TestInstanceStopRetriesAfterCommitFailure(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	bindStopPVC(t, cli)
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	got = readStopInstance(t, cli)
	got.Spec.InstanceAssistantObjects = []workloads.InstanceAssistantObject{{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo-partial", Namespace: "default"}, Data: map[string]string{"committed": "yes"}}}}
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	faulty := &failStopDeleteClient{Client: cli, remaining: 1}
	reconciler := &InstanceReconciler{Client: faulty, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo-0"}}); err == nil {
		t.Fatal("expected commit failure")
	}
	readStopPod(t, cli)
	assistant := &corev1.ConfigMap{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-partial"}, assistant); err != nil || assistant.Data["committed"] != "yes" {
		t.Fatalf("expected actual partial assistant commit: %v", err)
	}
	reconcileStopEntry(t, faulty, 4)
	assertStopPodAbsent(t, cli)
	if faulty.deleted != 1 || readStopPVC(t, cli).UID != "pvc-uid" || readStopInstance(t, cli).Status.CurrentState != workloads.InstanceCurrentStateAbsent {
		t.Fatal("retry did not converge without repeating side effects")
	}
}

func TestInstanceStopActualDeletionKeepsRetentionPolicy(t *testing.T) {
	for _, scaled := range []bool{false, true} {
		for _, retain := range []bool{false, true} {
			t.Run(fmt.Sprintf("scaled=%t/retain=%t", scaled, retain), func(t *testing.T) {
				inst := stopTestInstance()
				inst.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenDeleted: apps.DeletePersistentVolumeClaimRetentionPolicyType, WhenScaled: apps.DeletePersistentVolumeClaimRetentionPolicyType}
				if retain {
					if scaled {
						inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = apps.RetainPersistentVolumeClaimRetentionPolicyType
					} else {
						inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = apps.RetainPersistentVolumeClaimRetentionPolicyType
					}
				}
				cli := stopTestClient(t, inst)
				reconcileStopEntry(t, cli, 3)
				pvc := bindStopPVC(t, cli)
				got := readStopInstance(t, cli)
				got.Spec.Stop = ptr.To(true)
				got.Spec.ScaledDown = ptr.To(scaled)
				got.Generation++
				if err := cli.Update(context.Background(), got); err != nil {
					t.Fatal(err)
				}
				reconcileStopEntry(t, cli, 3)
				if err := cli.Delete(context.Background(), readStopInstance(t, cli)); err != nil {
					t.Fatal(err)
				}
				reconcileStopEntry(t, cli, 4)
				err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-0"}, &workloads.Instance{})
				if !apierrors.IsNotFound(err) {
					t.Fatalf("Instance deletion incomplete: %v", err)
				}
				existing := &corev1.PersistentVolumeClaim{}
				err = cli.Get(context.Background(), client.ObjectKeyFromObject(pvc), existing)
				if retain {
					if err != nil || existing.UID != pvc.UID || len(existing.OwnerReferences) != 0 {
						t.Fatalf("retention broken: %v %#v", err, existing)
					}
				} else if !apierrors.IsNotFound(err) {
					t.Fatalf("delete policy ignored: %v", err)
				}
			})
		}
	}
}

type failExternalClaimReadClient struct {
	client.Client
	fail bool
}

func (c *failExternalClaimReadClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.PersistentVolumeClaim); ok && key.Name == "external" && c.fail {
		return errors.New("external claim read unavailable")
	}
	return c.Client.Get(ctx, key, obj, opts...)
}
func TestInstanceStopCommitsBeforeExternalClaimReadRetry(t *testing.T) {
	for _, tc := range []struct {
		name    string
		stop    bool
		requeue bool
		retry   time.Duration
	}{
		{name: "stopped commits and retries observation", stop: true, retry: 5 * time.Second},
		{name: "running preserves earlier readiness retry", stop: false, requeue: true, retry: time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inst := stopTestInstance()
			inst.Spec.VolumeClaimTemplates = nil
			inst.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "external"}}}}
			cli := stopTestClient(t, inst)
			external := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "default", UID: "external-uid"}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "pv-external"}}
			if err := cli.Create(context.Background(), external); err != nil {
				t.Fatal(err)
			}
			reconcileStopEntry(t, cli, 3)
			observed := readStopInstance(t, cli)
			if !observed.Status.Storage.Complete || observed.Status.Storage.Volumes[0].Claim.UID != "external-uid" {
				t.Fatal("external claim not observed")
			}
			observed.Spec.Stop = ptr.To(tc.stop)
			observed.Spec.MinReadySeconds = 3600
			observed.Generation++
			if err := cli.Update(context.Background(), observed); err != nil {
				t.Fatal(err)
			}
			faulty := &failExternalClaimReadClient{Client: cli, fail: true}
			controller := &InstanceReconciler{Client: faulty, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
			result, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo-0"}})
			if err != nil || result.Requeue != tc.requeue || result.RequeueAfter != tc.retry {
				t.Fatalf("read did not schedule bounded retry: %v %#v", err, result)
			}
			if tc.stop {
				assertStopPodAbsent(t, cli)
			} else {
				pod := readStopPod(t, cli)
				if pod.DeletionTimestamp != nil {
					t.Fatal("running Pod was deleted")
				}
				status := readStopInstance(t, cli).Status
				if status.CurrentState != workloads.InstanceCurrentStatePresent || status.Storage == nil || status.Storage.Complete {
					t.Fatalf("read failure status was not persisted: %#v", status)
				}
			}
			unchanged := &corev1.PersistentVolumeClaim{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(external), unchanged); err != nil || unchanged.UID != external.UID || len(unchanged.OwnerReferences) != 0 {
				t.Fatal("external claim was mutated or adopted")
			}
			if tc.stop {
				reconcileStopEntry(t, cli, 2)
				if readStopInstance(t, cli).Status.CurrentState != workloads.InstanceCurrentStateAbsent {
					t.Fatal("stop did not converge despite failed observation")
				}
			}
		})
	}
}
