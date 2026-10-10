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

	"github.com/golang/mock/gomock"
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
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
)

func stopTestInstance() *workloads.Instance {
	return &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", UID: "instance-uid", Generation: 1},
		Spec: workloads.InstanceSpec{
			InstanceSetName: "demo",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}},
			},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
				},
			}},
		},
	}
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
	if got.Status.CurrentState != workloads.InstanceCurrentStateAbsent || got.Status.UpToDate || ptr.Deref(got.Spec.ScaledDown, false) {
		t.Fatalf("incorrect stopped status: %#v", got.Status)
	}
	got.Spec.Stop = nil
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	readStopPod(t, cli)
	readStopPVC(t, cli)
}

func TestInstanceStopDoesNotChangePodRevision(t *testing.T) {
	var revision string
	for _, tc := range []struct {
		name string
		stop *bool
	}{{name: "unset"}, {name: "false", stop: ptr.To(false)}, {name: "true", stop: ptr.To(true)}} {
		t.Run(tc.name, func(t *testing.T) {
			inst := stopTestInstance()
			inst.Spec.Stop = tc.stop
			cli := stopTestClient(t, inst)
			reconcileStopEntry(t, cli, 3)
			actual := readStopInstance(t, cli).Status.UpdateRevision
			if actual == "" {
				t.Fatal("revision was not computed")
			}
			if revision == "" {
				revision = actual
			} else if actual != revision {
				t.Fatalf("Stop changed revision from %s to %s", revision, actual)
			}
		})
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
	if observed.Status.CurrentState != workloads.InstanceCurrentStatePresent || observed.Status.UpToDate {
		t.Fatalf("stop guessed absent before observation: %#v", observed.Status)
	}
	reconcileStopEntry(t, cli, 2)
	before := readStopPVC(t, cli)
	got = readStopInstance(t, cli)
	got.Spec.Template.Spec.Containers[0].Image = "db:v2"
	got.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	got.Spec.VolumeClaimTemplates[0].Labels = map[string]string{"changed": "true"}
	got.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{"changed": "true"}
	got.Spec.VolumeClaimTemplates = append(got.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaimTemplate{ObjectMeta: metav1.ObjectMeta{Name: "logs"}, Spec: *got.Spec.VolumeClaimTemplates[0].Spec.DeepCopy()})
	got.Spec.InstanceAssistantObjects = []workloads.InstanceAssistantObject{{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo-env", Namespace: "default"}, Data: map[string]string{"version": "v2"}}}}
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 3)
	after := readStopPVC(t, cli)
	if !reflect.DeepEqual(after, before) || after.UID != pvc.UID || len(after.OwnerReferences) != 1 || after.OwnerReferences[0].UID != inst.UID {
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
	pvc := bindStopPVC(t, cli)
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
	if got.Status.CurrentState != workloads.InstanceCurrentStateTerminating {
		t.Fatal("deleting Pod was not reported as Terminating")
	}
	got.Spec.Stop = ptr.To(false)
	got.Spec.Template.Spec.Containers[0].Image = "db:v3"
	got.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	if readStopPod(t, cli).UID != "pod-held" || readStopPod(t, cli).Spec.Containers[0].Image != "db:v1" {
		t.Fatal("early resume replaced or modified terminating Pod")
	}
	if !reflect.DeepEqual(readStopPVC(t, cli), pvc) {
		t.Fatal("early resume changed PVC before the terminating Pod disappeared")
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
	if readStopPVC(t, cli).UID != pvc.UID || readStopPVC(t, cli).Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("resume did not apply current storage template to the original PVC")
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
				got.Spec.VolumeClaimTemplates = nil
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

func TestInstanceStopInvalidConfigObservation(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	pod := readStopPod(t, cli)
	pod.Annotations = map[string]string{constant.CMInsConfigurationHashLabelKey: "invalid-json"}
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	pod.Status.Phase = corev1.PodRunning
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	controller := &InstanceReconciler{Client: cli, Recorder: record.NewFakeRecorder(1000)}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}
	if _, err := controller.Reconcile(context.Background(), req); err == nil {
		t.Fatal("running observation should reject invalid config metadata")
	}
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 3)
	assertStopPodAbsent(t, cli)
	status := readStopInstance(t, cli).Status
	if status.CurrentState != workloads.InstanceCurrentStateAbsent || status.UpToDate || status.Configs != nil {
		t.Fatalf("invalid config metadata prevented stop convergence: %#v", status)
	}
}

func TestInstanceStopSkipsUpdateActionsAndPreservesRevision(t *testing.T) {
	inst := stopTestInstance()
	inst.Spec.LifecycleActions = &workloads.LifecycleActions{Switchover: &apps.Action{Exec: &apps.ExecAction{}}}
	inst.Spec.Configs = []workloads.ConfigTemplate{{Name: "config", ConfigHash: ptr.To("old")}}
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	bindStopPVC(t, cli)
	revision := readStopInstance(t, cli).Status.UpdateRevision
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(false)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 1)
	if readStopInstance(t, cli).Status.UpdateRevision != revision {
		t.Fatal("explicit Stop=false changed the Pod revision")
	}
	pod := readStopPod(t, cli)
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true}}
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	got = readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	got.Spec.Configs[0].ConfigHash = ptr.To("new")
	got.Spec.Configs[0].Reconfigure = &apps.Action{Exec: &apps.ExecAction{}}
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	mock := kbacli.NewMockClient(gomock.NewController(t))
	kbacli.SetMockClient(mock, nil)
	t.Cleanup(kbacli.UnsetMockClient)
	reconcileStopEntry(t, cli, 1)
	assertStopPodAbsent(t, cli)
	observed := readStopInstance(t, cli).Status
	if observed.CurrentState != workloads.InstanceCurrentStatePresent || !observed.Ready || !observed.Available || observed.UpToDate {
		t.Fatalf("stop did not preserve the actual ready Pod observation: %#v", observed)
	}
	got = readStopInstance(t, cli)
	changedRevision := got.Status.UpdateRevision
	got.Spec.Stop = nil
	got.Spec.MinReadySeconds = 3600
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	if readStopInstance(t, cli).Status.UpdateRevision != changedRevision {
		t.Fatal("clearing Stop changed the Pod revision")
	}
}

type failStopStatusClient struct {
	client.Client
	remaining int
}

func (c *failStopStatusClient) Status() client.SubResourceWriter {
	return &failStopStatusWriter{SubResourceWriter: c.Client.Status(), owner: c}
}

type failStopStatusWriter struct {
	client.SubResourceWriter
	owner *failStopStatusClient
}

func (w *failStopStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if _, ok := obj.(*workloads.Instance); ok && w.owner.remaining > 0 {
		w.owner.remaining--
		return errors.New("injected Instance status failure")
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

func TestInstanceStopRetriesAfterPodDeletedBeforeStatusCommit(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	pvc := bindStopPVC(t, cli)
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	faulty := &failStopStatusClient{Client: cli, remaining: 1}
	controller := &InstanceReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	if _, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}); err == nil {
		t.Fatal("expected status commit failure after Pod deletion")
	}
	assertStopPodAbsent(t, cli)
	reconcileStopEntry(t, cli, 3)
	if readStopInstance(t, cli).Status.CurrentState != workloads.InstanceCurrentStateAbsent || !reflect.DeepEqual(readStopPVC(t, cli), pvc) {
		t.Fatal("restart did not recover the observed state after partial commit")
	}
}

type failResumePodClient struct {
	client.Client
	remaining int
}

func (c *failResumePodClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*corev1.Pod); ok && c.remaining > 0 {
		c.remaining--
		return errors.New("injected resume Pod create failure")
	}
	return c.Client.Create(ctx, obj, opts...)
}

func TestInstanceStopResumeRetriesAfterPVCCommit(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	pvc := bindStopPVC(t, cli)
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	got = readStopInstance(t, cli)
	got.Spec.Stop = nil
	got.Spec.Template.Spec.Containers[0].Image = "db:v2"
	got.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	faulty := &failResumePodClient{Client: cli, remaining: 1}
	controller := &InstanceReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	if _, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}); err == nil {
		t.Fatal("expected resume Pod create failure")
	}
	assertStopPodAbsent(t, cli)
	committed := readStopPVC(t, cli)
	if committed.UID != pvc.UID || committed.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("expected storage update to commit before Pod creation failed")
	}
	reconcileStopEntry(t, cli, 3)
	if readStopPod(t, cli).Spec.Containers[0].Image != "db:v2" || readStopPVC(t, cli).UID != pvc.UID {
		t.Fatal("resume retry did not converge with the original PVC")
	}
}
