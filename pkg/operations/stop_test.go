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
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

func TestStopTargetsStopped(t *testing.T) {
	stopped := true
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{
		ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql", Stop: &stopped}, {Name: "proxy"}},
		Shardings:      []appsv1.ClusterSharding{{Name: "shard", Template: appsv1.ClusterComponentSpec{Stop: &stopped}}},
	}}
	newOpsResource := func(targets ...string) *OpsResource {
		stopList := make([]opsv1alpha1.ComponentOps, len(targets))
		for i := range targets {
			stopList[i].ComponentName = targets[i]
		}
		return &OpsResource{Cluster: cluster, OpsRequest: &opsv1alpha1.OpsRequest{
			Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{StopList: stopList}},
		}}
	}

	handler := StopOpsHandler{}
	if !handler.targetsStopped(newOpsResource("mysql", "shard")) {
		t.Fatal("stopped targets were rejected")
	}
	if handler.targetsStopped(newOpsResource("proxy")) {
		t.Fatal("running target was accepted")
	}
	if handler.targetsStopped(newOpsResource("missing")) {
		t.Fatal("missing target was accepted")
	}
	if handler.targetsStopped(newOpsResource()) {
		t.Fatal("stop-all accepted while a component remained running")
	}
	opsRes := newOpsResource("proxy")
	opsRes.Cluster.Generation = 7
	opsRes.OpsRequest.Status.ClusterGeneration = 8
	phase, _, err := handler.ReconcileAction(intctrlutil.RequestCtx{}, nil, opsRes)
	if err != nil || phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("phase=%s err=%v, want Running before the action generation is observed", phase, err)
	}
	opsRes.Cluster.Generation = 8
	phase, _, err = handler.ReconcileAction(intctrlutil.RequestCtx{}, nil, opsRes)
	if err != nil || phase != opsv1alpha1.OpsAbortedPhase {
		t.Fatalf("phase=%s err=%v, want Aborted after the stop target is overwritten", phase, err)
	}
}

func TestStopParticipantStatuses(t *testing.T) {
	const (
		active0  = "cluster-mysql-0"
		active1  = "cluster-mysql-1"
		active2  = "cluster-mysql-2"
		offline0 = "cluster-mysql-offline-0"
		offline1 = "cluster-mysql-offline-1"
		offline2 = "cluster-mysql-offline-2"
		released = "cluster-mysql-released"
	)
	replicas := int32(3)
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Replicas:         &replicas,
			OfflineInstances: []string{offline0, offline1, offline2},
		},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
			{PodName: active0, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent},
			{PodName: active1, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStateTerminating},
			{PodName: active2, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStateAbsent},
			{PodName: offline0, DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent},
			{PodName: offline1, DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent},
			{PodName: released, DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateAbsent},
		}},
	}

	participants := stopParticipantStatuses(its)
	if len(participants) != 3 || participants[0].PodName != active0 || participants[1].PodName != active1 || participants[2].PodName != active2 {
		t.Fatalf("participants=%v, want all active assignments independent of observed runtime state", participants)
	}

	stopped := true
	its.Spec.Stop = &stopped
	for i := range its.Status.InstanceStatus[:3] {
		its.Status.InstanceStatus[i].DesiredState = workloads.InstanceDesiredStateOffline
	}
	participants = stopParticipantStatuses(its)
	if len(participants) != 3 || participants[0].PodName != active0 || participants[1].PodName != active1 || participants[2].PodName != active2 {
		t.Fatalf("stopping participants=%v, want retained Stop identities without preexisting offline assignments", participants)
	}
}

func TestStopReconcilesCurrentInstanceStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default", Generation: 1,
			Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "data"}},
		Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql", Replicas: 2}}},
		Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase, ObservedGeneration: 1,
			Components: map[string]appsv1.ClusterComponentStatus{"mysql": {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 1, UpToDate: true}}},
	}
	ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "stop", Namespace: "default"},
		Spec:   opsv1alpha1.OpsRequestSpec{ClusterName: "cluster", Type: opsv1alpha1.StopType},
		Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase}}
	failProgressPatch := false
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, ops).
		WithStatusSubresource(cluster, ops).WithInterceptorFuncs(interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, c client.Client, subresource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if request, ok := obj.(*opsv1alpha1.OpsRequest); ok && request.Status.Phase == opsv1alpha1.OpsRunningPhase && failProgressPatch {
				failProgressPatch = false
				return errors.New("injected progress patch failure")
			}
			return c.SubResource(subresource).Patch(ctx, obj, patch, opts...)
		},
	}).Build()
	req := intctrlutil.RequestCtx{Ctx: context.Background()}
	load := func() *OpsResource {
		c, o := &appsv1.Cluster{}, &opsv1alpha1.OpsRequest{}
		if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(cluster), c); err != nil {
			t.Fatal(err)
		}
		if err := cli.Get(req.Ctx, client.ObjectKeyFromObject(ops), o); err != nil {
			t.Fatal(err)
		}
		return &OpsResource{Cluster: c, OpsRequest: o, Recorder: record.NewFakeRecorder(32)}
	}
	for range 4 {
		res := load()
		if res.OpsRequest.Status.Phase == opsv1alpha1.OpsCreatingPhase {
			break
		}
		if _, err := GetOpsManager().Do(req, cli, res); err != nil {
			t.Fatal(err)
		}
	}
	res := load()
	if res.OpsRequest.Status.Phase != opsv1alpha1.OpsCreatingPhase {
		t.Fatal("Stop did not reach Creating without InstanceSet status")
	}
	for range 2 {
		if _, err := GetOpsManager().Do(req, cli, load()); err != nil {
			t.Fatal(err)
		}
	}
	res = load()
	if !ptr.Deref(res.Cluster.Spec.ComponentSpecs[0].Stop, false) {
		t.Fatal("Stop was blocked by missing progress observations")
	}
	res.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
	res.OpsRequest.Status.ClusterGeneration = res.Cluster.Generation
	res.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{"mysql": {
		ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{ObjectKey: "Pod/old-name", Status: opsv1alpha1.FailedProgressStatus}},
	}}
	if err := cli.Status().Update(req.Ctx, res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	check := func(progress string, rows int, phase opsv1alpha1.OpsPhase) {
		t.Helper()
		delay, err := GetOpsManager().Reconcile(req, cli, load())
		if err != nil {
			t.Fatal(err)
		}
		got := load()
		if got.OpsRequest.Status.Progress != progress || len(got.OpsRequest.Status.Components["mysql"].ProgressDetails) != rows || got.OpsRequest.Status.Phase != phase {
			t.Fatalf("unexpected current progress: %+v", got.OpsRequest.Status)
		}
		if phase == opsv1alpha1.OpsRunningPhase && delay <= 0 {
			t.Fatal("missing reconciliation retry")
		}
	}
	check("0/2", 0, opsv1alpha1.OpsRunningPhase)
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "cluster-mysql", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{Replicas: ptr.To(int32(2)), Stop: ptr.To(true)},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
			{PodName: "current-name", DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent},
		}}}
	if err := cli.Create(req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	failProgressPatch = true
	if _, err := GetOpsManager().Reconcile(req, cli, load()); err == nil {
		t.Fatal("expected failed progress patch")
	}
	check("1/2", 1, opsv1alpha1.OpsRunningPhase)
	its.Status.InstanceStatus[0].CurrentState = workloads.InstanceCurrentStatePresent
	if err := cli.Update(req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	check("0/2", 1, opsv1alpha1.OpsRunningPhase)
	if !load().OpsRequest.Status.Components["mysql"].ProgressDetails[0].EndTime.IsZero() {
		t.Fatal("current waiting observation retained an old completion time")
	}
	its.Status.InstanceStatus[0].CurrentState = workloads.InstanceCurrentStateAbsent
	if err := cli.Update(req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	res = load()
	res.Cluster.Status.Components["mysql"] = appsv1.ClusterComponentStatus{Phase: appsv1.StoppedComponentPhase, ObservedGeneration: res.Cluster.Generation, UpToDate: true}
	if err := cli.Status().Update(req.Ctx, res.Cluster); err != nil {
		t.Fatal(err)
	}
	check("1/2", 1, opsv1alpha1.OpsRunningPhase)
	its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
		PodName: "second-name", DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateTerminating,
	})
	if err := cli.Update(req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	check("1/2", 2, opsv1alpha1.OpsRunningPhase)
	its.Status.InstanceStatus[1].CurrentState = workloads.InstanceCurrentStateAbsent
	its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
		PodName: "old-allocation", DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent,
	})
	if err := cli.Update(req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	check("3/3", 3, opsv1alpha1.OpsRunningPhase)
	its.Status.InstanceStatus = its.Status.InstanceStatus[:2]
	if err := cli.Update(req.Ctx, its); err != nil {
		t.Fatal(err)
	}
	failProgressPatch = true
	if _, err := GetOpsManager().Reconcile(req, cli, load()); err == nil {
		t.Fatal("expected failed final progress patch")
	}
	if load().OpsRequest.Status.Phase != opsv1alpha1.OpsRunningPhase {
		t.Fatal("Stop completed before final progress was persisted")
	}
	check("2/2", 2, opsv1alpha1.OpsSucceedPhase)
}

func TestStopWaitsForAppsResult(t *testing.T) {
	for _, replicas := range []int32{0, 1} {
		t.Run(fmt.Sprintf("replicas=%d", replicas), func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default", Generation: 1},
				Spec:       appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql", Replicas: replicas, Stop: ptr.To(true)}}},
				Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
					"mysql": {Phase: appsv1.StoppedComponentPhase, ObservedGeneration: 1, UpToDate: true},
				}},
			}
			ops := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "stop", Namespace: "default"},
				Spec:       opsv1alpha1.OpsRequestSpec{ClusterName: "cluster", Type: opsv1alpha1.StopType},
				Status:     opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, ClusterGeneration: 1},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, ops).
				WithStatusSubresource(cluster, ops).Build()
			ctx := context.Background()
			check := func(wantPhase opsv1alpha1.OpsPhase, wantProgress string) {
				t.Helper()
				currentCluster, currentOps := &appsv1.Cluster{}, &opsv1alpha1.OpsRequest{}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(cluster), currentCluster); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), currentOps); err != nil {
					t.Fatal(err)
				}
				res := &OpsResource{Cluster: currentCluster, OpsRequest: currentOps, Recorder: record.NewFakeRecorder(16)}
				delay, err := GetOpsManager().Reconcile(intctrlutil.RequestCtx{Ctx: ctx}, cli, res)
				if err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), currentOps); err != nil {
					t.Fatal(err)
				}
				if currentOps.Status.Phase != wantPhase || currentOps.Status.Progress != wantProgress {
					t.Fatalf("phase=%s progress=%s, want %s %s", currentOps.Status.Phase, currentOps.Status.Progress, wantPhase, wantProgress)
				}
				if wantPhase == opsv1alpha1.OpsRunningPhase && delay <= 0 {
					t.Fatal("Stop has no retry while observations disagree")
				}
			}
			check(opsv1alpha1.OpsRunningPhase, fmt.Sprintf("0/%d", replicas))
			its := &workloads.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster-mysql", Namespace: "default"},
				Spec:       workloads.InstanceSetSpec{Replicas: &replicas},
			}
			if replicas > 0 {
				its.Status.InstanceStatus = []workloads.InstanceStatus{{PodName: "mysql-0", DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStateAbsent}}
			}
			if err := cli.Create(ctx, its); err != nil {
				t.Fatal(err)
			}
			check(opsv1alpha1.OpsRunningPhase, fmt.Sprintf("%d/%d", replicas, replicas))
			cluster.Status.Components["mysql"] = appsv1.ClusterComponentStatus{Phase: appsv1.StoppingComponentPhase, ObservedGeneration: 1, UpToDate: true}
			if err := cli.Status().Update(ctx, cluster); err != nil {
				t.Fatal(err)
			}
			its.Spec.Stop = ptr.To(true)
			for i := range its.Status.InstanceStatus {
				its.Status.InstanceStatus[i].DesiredState = workloads.InstanceDesiredStateOffline
			}
			if err := cli.Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			check(opsv1alpha1.OpsRunningPhase, fmt.Sprintf("%d/%d", replicas, replicas))
			cluster.Status.Components["mysql"] = appsv1.ClusterComponentStatus{Phase: appsv1.StoppedComponentPhase, ObservedGeneration: 1, UpToDate: true}
			if err := cli.Status().Update(ctx, cluster); err != nil {
				t.Fatal(err)
			}
			check(opsv1alpha1.OpsSucceedPhase, fmt.Sprintf("%d/%d", replicas, replicas))
		})
	}
}

func TestStopAllTargetsComplete(t *testing.T) {
	const (
		namespace         = "default"
		clusterName       = "cluster"
		component         = "mysql"
		shardingName      = "shard"
		physicalComponent = "shard-0"
	)
	replicas := int32(1)
	stopped := true
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 8},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: component, Replicas: replicas, Stop: &stopped}},
			Shardings: []appsv1.ClusterSharding{{
				Name: shardingName, Shards: 1,
				Template:       appsv1.ClusterComponentSpec{Replicas: 3, Stop: &stopped},
				ShardTemplates: []appsv1.ShardTemplate{{Name: "small", Shards: ptr.To(int32(1)), Replicas: &replicas}},
			}},
		},
		Status: appsv1.ClusterStatus{
			Components: map[string]appsv1.ClusterComponentStatus{
				component: {Phase: appsv1.StoppedComponentPhase, ObservedGeneration: 8, UpToDate: true},
			},
			Shardings: map[string]appsv1.ClusterShardingStatus{
				shardingName: {Phase: appsv1.StoppedComponentPhase, ObservedGeneration: 8, UpToDate: true},
			},
		},
	}
	opsRequest := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "stop-all"},
		Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
			StopList: nil,
		}},
		Status: opsv1alpha1.OpsRequestStatus{
			ClusterGeneration: 8,
			Components:        map[string]opsv1alpha1.OpsRequestComponentStatus{},
		},
	}
	shardLabels := constant.GetClusterLabels(clusterName, map[string]string{
		constant.KBAppShardingNameLabelKey: shardingName,
	})
	shardLabels[constant.KBAppComponentLabelKey] = physicalComponent
	shardComponent := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      constant.GenerateClusterComponentName(clusterName, physicalComponent),
		Labels:    shardLabels,
	}}
	newInstanceSet := func(name string) *workloads.InstanceSet {
		return &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      constant.GenerateClusterComponentName(clusterName, name),
			},
			Spec: workloads.InstanceSetSpec{Replicas: &replicas},
			Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
				PodName:      constant.GenerateClusterComponentName(clusterName, name) + "-0",
				DesiredState: workloads.InstanceDesiredStateOffline,
				CurrentState: workloads.InstanceCurrentStateAbsent,
			}}},
		}
	}
	testScheme := runtime.NewScheme()
	for name, addToScheme := range map[string]func(*runtime.Scheme) error{
		"apps":       appsv1.AddToScheme,
		"operations": opsv1alpha1.AddToScheme,
		"workloads":  workloads.AddToScheme,
	} {
		if err := addToScheme(testScheme); err != nil {
			t.Fatalf("add %s scheme: %v", name, err)
		}
	}
	cli := fake.NewClientBuilder().WithScheme(testScheme).
		WithStatusSubresource(&opsv1alpha1.OpsRequest{}).
		WithObjects(opsRequest, newInstanceSet(component), newInstanceSet(physicalComponent)).Build()
	opsRes := &OpsResource{
		Cluster:    cluster,
		OpsRequest: opsRequest,
		Recorder:   record.NewFakeRecorder(10),
	}
	for _, name := range []string{component, physicalComponent} {
		obj := &workloads.InstanceSet{}
		key := client.ObjectKey{Namespace: namespace, Name: constant.GenerateClusterComponentName(clusterName, name)}
		if err := cli.Get(context.Background(), key, obj); err != nil {
			t.Fatal(err)
		}
		obj.Spec.Stop = &stopped
		if err := cli.Update(context.Background(), obj); err != nil {
			t.Fatal(err)
		}
	}

	phase, delay, err := (StopOpsHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes)
	if err != nil || phase != opsv1alpha1.OpsRunningPhase || delay <= 0 {
		t.Fatalf("phase=%s delay=%s err=%v, want retry while the shard list is incomplete", phase, delay, err)
	}
	if err := cli.Create(context.Background(), shardComponent); err != nil {
		t.Fatal(err)
	}
	phase, _, err = (StopOpsHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes)
	if err != nil {
		t.Fatalf("reconcile stop-all: %v", err)
	}
	if phase != opsv1alpha1.OpsSucceedPhase {
		t.Fatalf("phase=%s, want Succeed", phase)
	}
	if opsRequest.Status.Progress != "2/2" {
		t.Fatalf("progress=%s, want 2/2", opsRequest.Status.Progress)
	}
	if len(opsRequest.Status.Components[component].ProgressDetails) != 1 ||
		len(opsRequest.Status.Components[shardingName].ProgressDetails) != 1 {
		t.Fatalf("components=%v, want one detail for component and sharding", opsRequest.Status.Components)
	}
}

var _ = Describe("Stop OpsRequest", func() {
	var (
		randomStr      = testCtx.GetRandomStr()
		compDefName    = "test-compdef-" + randomStr
		clusterName    = "test-cluster-" + randomStr
		clusterDefName = "test-clusterdef-" + randomStr
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {

		It("Test 'Stop' OpsRequest", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			testapps.MockInstanceSetPods(&testCtx, nil, opsRes.Cluster, defaultCompName)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			By("create 'Stop' opsRequest")
			createStopOpsRequest(opsRes)

			By("test top action and reconcile function")
			runAction(reqCtx, opsRes, opsv1alpha1.OpsCreatingPhase)
			// do stop cluster
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
				Expect(v.Stop).ShouldNot(BeNil())
				Expect(*v.Stop).Should(BeTrue())
			}
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).Should(BeNil())
		})

		It("Test stop specific components OpsRequest", func() {
			By("init operations resources with topology")
			opsRes, _, _ := initOperationsResourcesWithTopology(clusterDefName, compDefName, clusterName)
			pods := testapps.MockInstanceSetPods(&testCtx, nil, opsRes.Cluster, defaultCompName)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)

			By("create 'Stop' opsRequest for specific components")
			createStopOpsRequest(opsRes, defaultCompName)

			By("mock 'Stop' OpsRequest to Creating phase")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			runAction(reqCtx, opsRes, opsv1alpha1.OpsCreatingPhase)

			By("test stop action")
			stopHandler := StopOpsHandler{}
			err := stopHandler.Action(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("verify components are being stopped")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, pobj *appsv1.Cluster) {
				for _, v := range pobj.Spec.ComponentSpecs {
					if v.Name == defaultCompName {
						Expect(v.Stop).ShouldNot(BeNil())
						Expect(*v.Stop).Should(BeTrue())
					} else {
						Expect(v.Stop).Should(BeNil())
					}
				}
			})).Should(Succeed())

			By("mock components stopped successfully")
			itsKey := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
				Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, defaultCompName)}
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
			})).Should(Succeed())
			for i := range pods {
				testk8s.MockPodIsTerminating(ctx, testCtx, pods[i])
				testk8s.RemovePodFinalizer(ctx, testCtx, pods[i])
			}
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			mockRollingTargetStatus(opsRes.Cluster, appsv1.StoppedComponentPhase, defaultCompName)

			By("test reconcile")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("verify ops request completed")
			Eventually(testops.GetOpsRequestPhase(&testCtx,
				client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		})

		It("Test does not abort other running opsRequests", func() {
			By("init operations resources with topology")
			opsRes, _, _ := initOperationsResourcesWithTopology(clusterDefName, compDefName, clusterName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}

			By("create a 'Restart' opsRequest with intersection component")
			ops1 := createRestartOpsObj(clusterName, "restart-ops"+randomStr, defaultCompName)
			opsRes.OpsRequest = ops1
			runAction(reqCtx, opsRes, opsv1alpha1.OpsCreatingPhase)

			By("create a 'Restart' opsRequest with non-intersection component")
			ops2 := createRestartOpsObj(clusterName, "restart-ops2"+randomStr, secondaryCompName)
			ops2.Spec.Force = true
			opsRes.OpsRequest = ops2
			runAction(reqCtx, opsRes, opsv1alpha1.OpsCreatingPhase)

			By("create a 'Start' opsRequest")
			ops3 := testops.CreateOpsRequest(ctx, testCtx, testops.NewOpsRequestObj("start-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.StartType))
			opsRes.OpsRequest = ops3
			Expect(testapps.ChangeObjStatus(&testCtx, ops3, func() {
				ops3.Status.Phase = opsv1alpha1.OpsPendingPhase
			})).Should(Succeed())
			runAction(reqCtx, opsRes, opsv1alpha1.OpsPendingPhase)

			By("create 'Stop' opsRequest for all components")
			createStopOpsRequest(opsRes, defaultCompName)
			stopHandler := StopOpsHandler{}
			err := stopHandler.Action(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("expect the running 'Restart' opsRequest to remain active")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("expect the 'Restart' opsRequest with non-intersection component  to be Creating")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("expect the pending 'Start' opsRequest to remain pending")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops3))).Should(Equal(opsv1alpha1.OpsPendingPhase))
		})
	})
})

func createStopOpsRequest(opsRes *OpsResource, stopCompNames ...string) *opsv1alpha1.OpsRequest {
	By("create Stop opsRequest")
	ops := testops.NewOpsRequestObj("stop-ops-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
		opsRes.Cluster.Name, opsv1alpha1.StopType)
	var stopList []opsv1alpha1.ComponentOps
	for _, stopCompName := range stopCompNames {
		stopList = append(stopList, opsv1alpha1.ComponentOps{
			ComponentName: stopCompName,
		})
	}
	ops.Spec.StopList = stopList
	opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
	// set ops phase to Pending
	opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
	return ops
}
