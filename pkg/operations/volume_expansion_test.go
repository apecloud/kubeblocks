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

package operations

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("VolumeExpansion", func() {
	const (
		clusterName       = "volume-cluster"
		compDefName       = "volume-def"
		consensusCompName = "consensus"
		vctName           = "data"
		storageClassName  = "volume-sc"
	)
	clean := func() {
		testapps.ClearClusterResources(&testCtx)
		testapps.ClearComponentResourcesWithRemoveFinalizerOption(&testCtx)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, client.HasLabels{testCtx.TestObjLabelKey})
	}
	BeforeEach(clean)
	AfterEach(clean)
	It("preserves payload updates and control fields in the existing API", func() {
		ops := testops.NewOpsRequestObj("volumeexpansion-update-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace, clusterName, opsv1alpha1.VolumeExpansionType)
		ops.Spec.VolumeExpansionList = []opsv1alpha1.VolumeExpansion{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: consensusCompName}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: vctName, Storage: resource.MustParse("5Gi")}}}}
		ops = testops.CreateOpsRequest(ctx, testCtx, ops)
		current := ops.DeepCopy()
		current.Spec.VolumeExpansionList[0].VolumeClaimTemplates[0].Storage = resource.MustParse("6Gi")
		Expect(k8sClient.Update(ctx, current)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), current)).To(Succeed())
		current.Spec.VolumeExpansionList = nil
		Expect(k8sClient.Update(ctx, current)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), current)).To(Succeed())
		current.Spec.VolumeExpansionList = ops.Spec.VolumeExpansionList
		current.Spec.Cancel = true
		Expect(k8sClient.Update(ctx, current)).To(Succeed())
	})

	It("rejects an explicit unsupported storage class before writing the Cluster", func() {
		testapps.CreateStorageClass(&testCtx, storageClassName, false)
		_, clusterObject := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
		Expect(testapps.ChangeObj(&testCtx, clusterObject, func(c *appsv1.Cluster) {
			c.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.StorageClassName = ptr.To(storageClassName)
		})).To(Succeed())
		before := clusterObject.Spec.DeepCopy()
		for _, phase := range []opsv1alpha1.OpsPhase{opsv1alpha1.OpsPendingPhase, opsv1alpha1.OpsCreatingPhase} {
			ops := testops.NewOpsRequestObj("volumeexpansion-sc-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace, clusterName, opsv1alpha1.VolumeExpansionType)
			ops.Spec.VolumeExpansionList = []opsv1alpha1.VolumeExpansion{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: consensusCompName}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: vctName, Storage: resource.MustParse("5Gi")}}}}
			ops = testops.CreateOpsRequest(ctx, testCtx, ops)
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() { ops.Status.Phase = phase })).To(Succeed())
			opsRes := &OpsResource{Cluster: clusterObject, OpsRequest: ops, Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller")}
			_, err := GetOpsManager().Do(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, opsRes)
			Expect(err).NotTo(HaveOccurred())
			persistedOps := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), persistedOps)).To(Succeed())
			Expect(persistedOps.Status.Phase).To(Equal(opsv1alpha1.OpsFailedPhase))
			persistedCluster := &appsv1.Cluster{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObject), persistedCluster)).To(Succeed())
			Expect(persistedCluster.Spec).To(Equal(*before))
		}
	})

	It("persists instance progress before completing through OpsManager", func() {
		_, cluster := testapps.InitConsensusMysql(&testCtx, clusterName, compDefName, consensusCompName)
		testapps.MockInstanceSetComponent(&testCtx, clusterName, consensusCompName)
		ops := testops.NewOpsRequestObj("volume-online", testCtx.DefaultNamespace, clusterName, opsv1alpha1.VolumeExpansionType)
		ops.Spec.VolumeExpansionList = []opsv1alpha1.VolumeExpansion{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: consensusCompName}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: vctName, Storage: resource.MustParse("5Gi")}}}}
		ops = testops.CreateOpsRequest(ctx, testCtx, ops)
		Expect(testapps.ChangeObjStatus(&testCtx, ops, func() { ops.Status.Phase = opsv1alpha1.OpsPendingPhase })).To(Succeed())
		res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller")}
		req := intctrlutil.RequestCtx{Ctx: ctx}
		_, err := GetOpsManager().Do(req, k8sClient, res)
		Expect(err).NotTo(HaveOccurred())
		_, err = GetOpsManager().Do(req, k8sClient, res)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
		Expect(testapps.ChangeObjStatus(&testCtx, ops, func() { ops.Status.Phase = opsv1alpha1.OpsRunningPhase; ops.Status.StartTimestamp = metav1.Now() })).To(Succeed())
		res.OpsRequest = ops
		delay, err := GetOpsManager().Reconcile(req, k8sClient, res)
		Expect(err).NotTo(HaveOccurred())
		Expect(delay).To(Equal(time.Minute))
		Expect(res.OpsRequest.Status.Phase).To(Equal(opsv1alpha1.OpsRunningPhase))
		Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
			cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{consensusCompName: {ObservedGeneration: cluster.Generation, UpToDate: true, Phase: appsv1.FailedComponentPhase}}
		})).To(Succeed())
		its := &workloads.InstanceSet{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-" + consensusCompName}, its)).To(Succeed())
		Expect(testapps.ChangeObj(&testCtx, its, func(i *workloads.InstanceSet) {
			i.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: vctName}, Spec: *cluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.DeepCopy()}}
		})).To(Succeed())
		Expect(testapps.ChangeObjStatus(&testCtx, its, func() {
			its.Status.ObservedGeneration = its.Generation
			its.Status.Replicas = cluster.Spec.ComponentSpecs[0].Replicas
			its.Status.InstanceStatus = nil
			for n := int32(0); n < cluster.Spec.ComponentSpecs[0].Replicas; n++ {
				its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: fmt.Sprintf("%s-chosen-%d", its.Name, n), TemplateName: ptr.To(""), DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent, UpToDate: true})
			}
		})).To(Succeed())
		// Reload persisted objects as a controller retry does.
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), ops)).To(Succeed())
		res.OpsRequest = ops
		_, err = GetOpsManager().Reconcile(req, k8sClient, res)
		Expect(err).NotTo(HaveOccurred())
		persisted := &opsv1alpha1.OpsRequest{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), persisted)).To(Succeed())
		Expect(persisted.Status.Phase).To(Equal(opsv1alpha1.OpsSucceedPhase))
		Expect(persisted.Status.Progress).To(Equal("3/3"))
		Expect(persisted.Status.Components[consensusCompName].ProgressDetails).To(HaveLen(3))
		for _, detail := range persisted.Status.Components[consensusCompName].ProgressDetails {
			Expect(detail.Status).To(Equal(opsv1alpha1.SucceedProgressStatus))
			Expect(detail.EndTime.IsZero()).To(BeFalse())
		}
	})
})

type volumeExpansionFixture struct {
	cli      client.WithWatch
	res      *OpsResource
	req      intctrlutil.RequestCtx
	its      *workloads.InstanceSet
	recorder *record.FakeRecorder
}

func newVolumeExpansionFixture(t *testing.T) *volumeExpansionFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	vct := appsv1.PersistentVolumeClaimTemplate{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}}}}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 11}, Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "db", Replicas: 2, VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{vct}}}}, Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase, Components: map[string]appsv1.ClusterComponentStatus{"db": {ObservedGeneration: 11, UpToDate: true}}}}
	ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "expand", Namespace: "default", UID: "expand-uid"}, Spec: opsv1alpha1.OpsRequestSpec{ClusterName: cluster.Name, Type: opsv1alpha1.VolumeExpansionType, SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{VolumeExpansionList: []opsv1alpha1.VolumeExpansion{{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("5Gi")}}}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default", Generation: 7}, Spec: workloads.InstanceSetSpec{Replicas: ptr.To(int32(2)), VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: vct.Name}, Spec: *vct.Spec.DeepCopy()}}}, Status: workloads.InstanceSetStatus{ObservedGeneration: 7}}
	for _, name := range []string{"owner-chosen-a", "owner-chosen-b"} {
		its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: name, TemplateName: ptr.To(""), DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent})
	}
	rejectRead := func(obj interface{}) error {
		switch obj.(type) {
		case *corev1.Pod, *corev1.PodList, *corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaimList, *corev1.PersistentVolume, *corev1.PersistentVolumeList, *corev1.Service, *corev1.ServiceList, *corev1.ConfigMap, *corev1.ConfigMapList:
			return fmt.Errorf("forbidden data read %T", obj)
		}
		return nil
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, its).WithObjects(cluster, ops, its).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if err := rejectRead(obj); err != nil {
				return err
			}
			return cli.Get(ctx, key, obj, opts...)
		},
		List: func(ctx context.Context, cli client.WithWatch, obj client.ObjectList, opts ...client.ListOption) error {
			if err := rejectRead(obj); err != nil {
				return err
			}
			return cli.List(ctx, obj, opts...)
		},
	}).Build()
	recorder := record.NewFakeRecorder(100)
	f := &volumeExpansionFixture{cli: cli, res: &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: recorder}, req: intctrlutil.RequestCtx{Ctx: context.Background()}, its: its, recorder: recorder}
	for n := 0; n < 2; n++ {
		if _, err := GetOpsManager().Do(f.req, cli, f.res); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.Get(f.req.Ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
		t.Fatal(err)
	}
	ops.Status.Phase = opsv1alpha1.OpsRunningPhase
	ops.Status.StartTimestamp = metav1.Now()
	if err := cli.Status().Update(f.req.Ctx, ops); err != nil {
		t.Fatal(err)
	}
	f.reload(t)
	return f
}

func (f *volumeExpansionFixture) reload(t *testing.T) {
	t.Helper()
	ops := &opsv1alpha1.OpsRequest{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.OpsRequest), ops); err != nil {
		t.Fatal(err)
	}
	f.res.OpsRequest = ops
}

func (f *volumeExpansionFixture) publish(t *testing.T) {
	t.Helper()
	status := f.its.Status.DeepCopy()
	if err := f.cli.Update(f.req.Ctx, f.its); err != nil {
		t.Fatal(err)
	}
	f.its.Status = *status
	if err := f.cli.Status().Update(f.req.Ctx, f.its); err != nil {
		t.Fatal(err)
	}
}

func (f *volumeExpansionFixture) forward(t *testing.T) {
	t.Helper()
	f.its.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
	f.publish(t)
}

func (f *volumeExpansionFixture) reconcile(t *testing.T, phase opsv1alpha1.OpsPhase, progress string) {
	t.Helper()
	delay, err := GetOpsManager().Reconcile(f.req, f.cli, f.res)
	if err != nil {
		t.Fatal(err)
	}
	f.reload(t)
	if got := f.res.OpsRequest.Status; got.Phase != phase || got.Progress != progress {
		t.Fatalf("phase=%s progress=%s, want %s %s", got.Phase, got.Progress, phase, progress)
	}
	if phase == opsv1alpha1.OpsRunningPhase && delay != time.Minute {
		t.Fatalf("requeue=%s, want 1 minute", delay)
	}
}

func TestVolumeExpansionUsesOwnerObservations(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	f.res.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{"db": {ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{ObjectKey: "PVC/retired", Status: opsv1alpha1.FailedProgressStatus}}}}
	if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "0/2")
	for _, d := range f.res.OpsRequest.Status.Components["db"].ProgressDetails {
		if !strings.HasPrefix(d.ObjectKey, "Pod/owner-chosen-") || d.StartTime.IsZero() {
			t.Fatalf("unexpected current detail: %#v", d)
		}
	}
	f.forward(t)
	f.its.Status.InstanceStatus[0].UpToDate = true
	f.publish(t)
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "1/2")
	for len(f.recorder.Events) > 0 {
		<-f.recorder.Events
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "1/2")
	if len(f.recorder.Events) != 0 {
		t.Fatal("unchanged persisted progress emitted another event")
	}
	f.its.Status.InstanceStatus[1].UpToDate = true
	f.its.Status.InstanceStatus[1].Failed = true // Health does not decide storage completion.
	f.publish(t)
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase, "2/2")
	for _, d := range f.res.OpsRequest.Status.Components["db"].ProgressDetails {
		if d.Status != opsv1alpha1.SucceedProgressStatus || d.EndTime.IsZero() {
			t.Fatalf("incomplete persisted detail %#v", d)
		}
	}
}

func TestVolumeExpansionWaitsForCurrentTargetAndAllocation(t *testing.T) {
	for _, state := range []string{"old-target", "stale-apps", "stale-its", "missing-its", "missing-row", "unknown-template", "absent", "offline", "released", "stale-allocation"} {
		t.Run(state, func(t *testing.T) {
			f := newVolumeExpansionFixture(t)
			f.forward(t)
			for i := range f.its.Status.InstanceStatus {
				f.its.Status.InstanceStatus[i].UpToDate = true
			}
			switch state {
			case "old-target":
				f.its.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
			case "stale-apps":
				s := f.res.Cluster.Status.Components["db"]
				s.ObservedGeneration--
				f.res.Cluster.Status.Components["db"] = s
			case "stale-its":
				f.its.Status.ObservedGeneration--
			case "missing-its":
				if err := f.cli.Delete(f.req.Ctx, f.its); err != nil {
					t.Fatal(err)
				}
			case "missing-row":
				f.its.Status.InstanceStatus = f.its.Status.InstanceStatus[:1]
			case "unknown-template":
				f.its.Status.InstanceStatus[0].TemplateName = nil
			case "absent":
				f.its.Status.InstanceStatus[0].CurrentState = workloads.InstanceCurrentStateAbsent
			case "offline":
				f.its.Status.InstanceStatus[0].DesiredState = workloads.InstanceDesiredStateOffline
			case "released":
				f.its.Status.InstanceStatus[0].DesiredState = workloads.InstanceDesiredStateReleased
			case "stale-allocation":
				f.its.Spec.Replicas = ptr.To(int32(1))
			}
			if state != "missing-its" {
				f.publish(t)
			}
			delay, err := GetOpsManager().Reconcile(f.req, f.cli, f.res)
			if err != nil {
				t.Fatal(err)
			}
			f.reload(t)
			if f.res.OpsRequest.Status.Phase != opsv1alpha1.OpsRunningPhase || delay != time.Minute {
				t.Fatalf("phase=%s delay=%s", f.res.OpsRequest.Status.Phase, delay)
			}
		})
	}
}

func TestVolumeExpansionStopsUntilTargetForwardedAfterStart(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	f.res.Cluster.Spec.ComponentSpecs[0].Stop = ptr.To(true)
	f.its.Spec.Stop = ptr.To(true)
	// Stop can advance generation while preserving the old VCT target.
	for i := range f.its.Status.InstanceStatus {
		f.its.Status.InstanceStatus[i].DesiredState = workloads.InstanceDesiredStateOffline
		f.its.Status.InstanceStatus[i].UpToDate = true
	}
	f.publish(t)
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "0/2")
	f.res.Cluster.Spec.ComponentSpecs[0].Stop = nil
	f.its.Spec.Stop = nil
	for i := range f.its.Status.InstanceStatus {
		f.its.Status.InstanceStatus[i].DesiredState = workloads.InstanceDesiredStateActive
	}
	f.publish(t)
	// Start itself clears Stop before the next apps reconcile forwards the new target.
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "0/2")
	for _, detail := range f.res.OpsRequest.Status.Components["db"].ProgressDetails {
		if detail.Status == opsv1alpha1.SucceedProgressStatus {
			t.Fatalf("old target reported expanded: %#v", detail)
		}
	}
	f.forward(t)
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase, "2/2")
}

func TestVolumeExpansionTimeoutOrdering(t *testing.T) {
	for _, state := range []string{"failed-30m", "aborted-generic", "completed-after-deadline"} {
		t.Run(state, func(t *testing.T) {
			f := newVolumeExpansionFixture(t)
			f.res.OpsRequest.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-VolumeExpansionTimeOut - time.Minute))
			f.res.OpsRequest.Spec.TimeoutSeconds = ptr.To(int32(1))
			want := opsv1alpha1.OpsFailedPhase
			if state == "aborted-generic" {
				f.res.OpsRequest.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
				want = opsv1alpha1.OpsAbortedPhase
			}
			if state == "completed-after-deadline" {
				f.forward(t)
				for i := range f.its.Status.InstanceStatus {
					f.its.Status.InstanceStatus[i].UpToDate = true
				}
				f.publish(t)
				want = opsv1alpha1.OpsSucceedPhase
			}
			status := f.res.OpsRequest.Status.DeepCopy()
			if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			f.res.OpsRequest.Status = *status
			if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			if _, err := GetOpsManager().Reconcile(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.reload(t)
			if f.res.OpsRequest.Status.Phase != want {
				t.Fatalf("phase=%s, want %s", f.res.OpsRequest.Status.Phase, want)
			}
		})
	}
}

func TestVolumeExpansionRetriesProgressPatchBeforeSuccess(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	f.forward(t)
	for i := range f.its.Status.InstanceStatus {
		f.its.Status.InstanceStatus[i].UpToDate = true
	}
	f.publish(t)
	fail := true
	cli := interceptor.NewClient(f.cli, interceptor.Funcs{SubResourcePatch: func(ctx context.Context, cli client.Client, name string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
		if fail && name == "status" {
			fail = false
			return fmt.Errorf("injected status patch failure")
		}
		return cli.SubResource(name).Patch(ctx, obj, patch, opts...)
	}})
	if _, err := GetOpsManager().Reconcile(f.req, cli, f.res); err == nil {
		t.Fatal("patch failure was lost")
	}
	f.reload(t)
	if f.res.OpsRequest.Status.Phase != opsv1alpha1.OpsRunningPhase || f.res.OpsRequest.Status.Progress != "" {
		t.Fatalf("persisted premature success: %#v", f.res.OpsRequest.Status)
	}
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase, "2/2")
}

func TestVolumeExpansionPreservesOverridesAndVolumeUnits(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	spec := &f.res.Cluster.Spec.ComponentSpecs[0]
	logs := spec.VolumeClaimTemplates[0].DeepCopy()
	logs.Name = "logs"
	spec.VolumeClaimTemplates = append(spec.VolumeClaimTemplates, *logs)
	f.res.OpsRequest.Spec.VolumeExpansionList[0].VolumeClaimTemplates = append(f.res.OpsRequest.Spec.VolumeExpansionList[0].VolumeClaimTemplates, opsv1alpha1.OpsRequestVolumeClaimTemplate{Name: "logs", Storage: resource.MustParse("5Gi")})
	independent := spec.VolumeClaimTemplates[0].DeepCopy()
	independent.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	other := independent.DeepCopy()
	other.Name = "other"
	spec.Replicas = 3
	spec.Instances = []appsv1.InstanceTemplate{{Name: "custom", Replicas: ptr.To(int32(1)), VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{*independent}}, {Name: "inherited", Replicas: ptr.To(int32(1)), VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{*other}}}
	// Action keeps explicit instance storage untouched and applies both logical targets.
	if err := (volumeExpansionOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if got := spec.Instances[0].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String(); got != "2Gi" {
		t.Fatalf("override=%s", got)
	}
	f.its.Spec.Replicas = ptr.To(int32(3))
	f.its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: *spec.VolumeClaimTemplates[0].Spec.DeepCopy()}, {ObjectMeta: metav1.ObjectMeta{Name: "logs"}, Spec: *spec.VolumeClaimTemplates[1].Spec.DeepCopy()}}
	f.its.Spec.Instances = []workloads.InstanceTemplate{{Name: "custom", Replicas: ptr.To(int32(1)), VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: *independent.Spec.DeepCopy()}}}, {Name: "inherited", Replicas: ptr.To(int32(1)), VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Spec: *other.Spec.DeepCopy()}}}}
	f.its.Status.InstanceStatus = nil
	for _, name := range []string{"", "custom", "inherited"} {
		f.its.Status.InstanceStatus = append(f.its.Status.InstanceStatus, workloads.InstanceStatus{PodName: "chosen-" + name, TemplateName: ptr.To(name), DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent, UpToDate: name != "inherited"})
	}
	f.publish(t)
	if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "3/5")
	f.its.Status.InstanceStatus[2].UpToDate = true
	f.its.Spec.VolumeClaimTemplates[1].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	f.publish(t)
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "2/5")
	f.its.Spec.VolumeClaimTemplates[1].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
	f.publish(t)
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase, "5/5")
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if len(details) != 3 {
		t.Fatalf("combined details=%d, want 3", len(details))
	}
	for _, d := range details {
		if d.ObjectKey == "Pod/chosen-custom" && strings.Contains(d.Message, "data") {
			t.Fatalf("override included in progress: %s", d.Message)
		}
	}
}

func TestVolumeExpansionZeroWorkRequiresObservedAllocation(t *testing.T) {
	for _, state := range []string{"empty-selection", "zero-replicas", "all-overrides", "stopped-all-overrides"} {
		t.Run(state, func(t *testing.T) {
			f := newVolumeExpansionFixture(t)
			if state == "empty-selection" || state == "stopped-all-overrides" {
				f.forward(t)
			}
			spec := &f.res.Cluster.Spec.ComponentSpecs[0]
			switch state {
			case "empty-selection":
				f.res.OpsRequest.Spec.VolumeExpansionList[0].VolumeClaimTemplates = nil
			case "zero-replicas":
				spec.Replicas = 0
				f.its.Spec.Replicas = ptr.To(int32(0))
				f.its.Status.InstanceStatus = nil
			default:
				override := spec.VolumeClaimTemplates[0].DeepCopy()
				override.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
				spec.Instances = []appsv1.InstanceTemplate{{Name: "custom", Replicas: ptr.To(int32(2)), VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{*override}}}
				f.its.Spec.Instances = []workloads.InstanceTemplate{{Name: "custom", Replicas: ptr.To(int32(2)), VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: *override.Spec.DeepCopy()}}}}
				for i := range f.its.Status.InstanceStatus {
					f.its.Status.InstanceStatus[i].TemplateName = ptr.To("custom")
				}
				if state == "stopped-all-overrides" {
					spec.Stop = ptr.To(true)
					f.its.Spec.Stop = ptr.To(true)
					for i := range f.its.Status.InstanceStatus {
						f.its.Status.InstanceStatus[i].DesiredState = workloads.InstanceDesiredStateOffline
						f.its.Status.InstanceStatus[i].CurrentState = workloads.InstanceCurrentStateAbsent
					}
				}
			}
			f.its.Status.ObservedGeneration--
			f.publish(t)
			if err := f.cli.Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			if state == "empty-selection" {
				s := f.res.Cluster.Status.Components["db"]
				s.UpToDate = false
				f.res.Cluster.Status.Components["db"] = s
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase, "0/0")
			f.its.Status.ObservedGeneration = f.its.Generation
			f.publish(t)
			s := f.res.Cluster.Status.Components["db"]
			s.UpToDate = true
			f.res.Cluster.Status.Components["db"] = s
			if state == "zero-replicas" || state == "all-overrides" {
				f.reconcile(t, opsv1alpha1.OpsRunningPhase, "0/0")
				f.forward(t)
			}
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase, "0/0")
		})
	}
}

func TestVolumeExpansionEmptySelectionStillRequiresOwnerAllocation(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	f.res.OpsRequest.Spec.VolumeExpansionList[0].VolumeClaimTemplates = nil
	if err := f.cli.Delete(f.req.Ctx, f.its); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "0/0")
}

func TestVolumeExpansionShardScopesAndOverrides(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	root := *f.res.Cluster.Spec.ComponentSpecs[0].DeepCopy()
	root.Replicas = 1
	independent := *root.VolumeClaimTemplates[0].DeepCopy()
	independent.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	f.res.Cluster.Spec.ComponentSpecs = nil
	f.res.Cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "db", Shards: 2, Template: root, ShardTemplates: []appsv1.ShardTemplate{{Name: "wide", Shards: ptr.To(int32(1)), Replicas: ptr.To(int32(2))}, {Name: "independent", Shards: ptr.To(int32(1)), Replicas: ptr.To(int32(3)), VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{independent}}}}}
	f.res.Cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{"db": {ObservedGeneration: 11, UpToDate: true}}
	makeComp := func(name, template string, replicas int32) *appsv1.Component {
		return &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name, Namespace: "default", Labels: constant.GetCompLabels("demo", name, map[string]string{constant.KBAppShardingNameLabelKey: "db", constant.KBAppShardTemplateLabelKey: template})}, Spec: appsv1.ComponentSpec{Replicas: replicas}}
	}
	if err := f.cli.Create(f.req.Ctx, makeComp("db-a", "wide", 2)); err != nil {
		t.Fatal(err)
	}
	f.its.Name = "demo-db-a"
	f.its.ResourceVersion = ""
	if err := f.cli.Create(f.req.Ctx, f.its); err != nil {
		t.Fatal(err)
	}
	f.forward(t)
	for i := range f.its.Status.InstanceStatus {
		f.its.Status.InstanceStatus[i].UpToDate = true
	}
	f.publish(t)
	// The complete included child cannot hide an unobserved physical scope.
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "2/2")
	if err := f.cli.Create(f.req.Ctx, makeComp("db-b", "independent", 3)); err != nil {
		t.Fatal(err)
	}
	// Explicit shard VCT replacement excludes that whole scope, without needing its ITS.
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase, "2/2")
	if len(f.res.OpsRequest.Status.Components["db"].ProgressDetails) != 2 {
		t.Fatal("unaffected shard included")
	}
}

func TestVolumeExpansionCannotCompensateAcrossScopes(t *testing.T) {
	f := newVolumeExpansionFixture(t)
	f.forward(t)
	for i := range f.its.Status.InstanceStatus {
		f.its.Status.InstanceStatus[i].UpToDate = true
	}
	extra := f.its.Status.InstanceStatus[0]
	extra.PodName = "extra-current-instance"
	f.its.Status.InstanceStatus = append(f.its.Status.InstanceStatus, extra)
	f.publish(t)
	spec := *f.res.Cluster.Spec.ComponentSpecs[0].DeepCopy()
	spec.Name = "other"
	f.res.Cluster.Spec.ComponentSpecs = append(f.res.Cluster.Spec.ComponentSpecs, spec)
	f.res.Cluster.Status.Components["other"] = f.res.Cluster.Status.Components["db"]
	request := f.res.OpsRequest.Spec.VolumeExpansionList[0]
	request.ComponentName = "other"
	f.res.OpsRequest.Spec.VolumeExpansionList = append(f.res.OpsRequest.Spec.VolumeExpansionList, request)
	other := f.its.DeepCopy()
	other.Name = "demo-other"
	other.ResourceVersion = ""
	other.Status.InstanceStatus = other.Status.InstanceStatus[:1]
	other.Status.InstanceStatus[0].PodName = "other-chosen"
	if err := f.cli.Create(f.req.Ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Status().Update(f.req.Ctx, other); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase, "4/4")
}
