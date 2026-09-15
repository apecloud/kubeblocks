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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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

type failNextStatusPatchClient struct {
	client.Client
	failNext  bool
	failPhase opsv1alpha1.OpsPhase
}

func (c *failNextStatusPatchClient) Status() client.StatusWriter {
	return &failNextStatusWriter{StatusWriter: c.Client.Status(), failNext: &c.failNext, failPhase: c.failPhase}
}

type failNextStatusWriter struct {
	client.StatusWriter
	failNext  *bool
	failPhase opsv1alpha1.OpsPhase
}

func (w *failNextStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch,
	opts ...client.SubResourcePatchOption) error {
	opsRequest, isOpsRequest := obj.(*opsv1alpha1.OpsRequest)
	if *w.failNext && isOpsRequest && opsRequest.Status.Phase == w.failPhase {
		*w.failNext = false
		return errors.New("injected status patch failure")
	}
	return w.StatusWriter.Patch(ctx, obj, patch, opts...)
}

func TestStopPersistsParticipantsBeforeActionAndReusesThemAfterPatchFailure(t *testing.T) {
	const (
		namespace = "default"
		cluster   = "cluster"
		component = "mysql"
		active0   = "cluster-mysql-0"
		active1   = "cluster-mysql-1"
		offline   = "cluster-mysql-offline"
	)
	replicas := int32(2)
	scheme := runtime.NewScheme()
	for name, addToScheme := range map[string]func(*runtime.Scheme) error{
		"apps": appsv1.AddToScheme, "operations": opsv1alpha1.AddToScheme, "workloads": workloads.AddToScheme,
	} {
		if err := addToScheme(scheme); err != nil {
			t.Fatalf("add %s scheme: %v", name, err)
		}
	}
	clusterObj := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace, Name: cluster, Generation: 1,
			Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "data"},
		},
		Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
			Name: component, Replicas: replicas,
		}}},
		Status: appsv1.ClusterStatus{
			Phase:              appsv1.RunningClusterPhase,
			ObservedGeneration: 1,
			Components: map[string]appsv1.ClusterComponentStatus{
				component: {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 1, UpToDate: true},
			},
		},
	}
	ops := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "stop"},
		Spec: opsv1alpha1.OpsRequestSpec{
			ClusterName: cluster,
			Type:        opsv1alpha1.StopType,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{StopList: []opsv1alpha1.ComponentOps{{
				ComponentName: component,
			}}},
		},
		Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsPendingPhase, Progress: "-/-"},
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace, Name: constant.GenerateClusterComponentName(cluster, component), Generation: 1,
		},
		Spec: workloads.InstanceSetSpec{Replicas: &replicas, OfflineInstances: []string{offline}},
		Status: workloads.InstanceSetStatus{
			Replicas: 99,
			InstanceStatus: []workloads.InstanceStatus{
				{PodName: active0, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent},
				{PodName: offline, DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&appsv1.Cluster{}, &opsv1alpha1.OpsRequest{}).
		WithObjects(clusterObj, ops, its).Build()
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}
	load := func() *OpsResource {
		gotCluster := &appsv1.Cluster{}
		gotOps := &opsv1alpha1.OpsRequest{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(clusterObj), gotCluster); err != nil {
			t.Fatal(err)
		}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(ops), gotOps); err != nil {
			t.Fatal(err)
		}
		return &OpsResource{Cluster: gotCluster, OpsRequest: gotOps, Recorder: record.NewFakeRecorder(20)}
	}

	var (
		res     *OpsResource
		waitErr error
	)
	for range 2 {
		res = load()
		_, waitErr = GetOpsManager().Do(reqCtx, cli, res)
		if waitErr != nil {
			break
		}
	}
	if !intctrlutil.IsTargetError(waitErr, intctrlutil.ErrorTypeNeedWaiting) {
		t.Fatalf("incomplete participant observation error=%v, want NeedWaiting", waitErr)
	}
	res = load()
	if res.OpsRequest.Status.Phase != opsv1alpha1.OpsPendingPhase ||
		len(res.OpsRequest.Status.Components[component].ProgressDetails) != 0 {
		t.Fatalf("incomplete participant baseline was persisted: %#v", res.OpsRequest.Status)
	}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(its), its); err != nil {
		t.Fatal(err)
	}
	its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
		PodName: active1, DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStateAbsent,
	})
	if err := cli.Update(reqCtx.Ctx, its); err != nil {
		t.Fatal(err)
	}

	creatingFailure := &failNextStatusPatchClient{
		Client: cli, failNext: true, failPhase: opsv1alpha1.OpsCreatingPhase,
	}
	var creatingErr error
	for range 4 {
		res = load()
		if _, creatingErr = GetOpsManager().Do(reqCtx, creatingFailure, res); creatingErr != nil {
			break
		}
	}
	if creatingErr == nil || creatingFailure.failNext {
		t.Fatalf("creating status patch error=%v armed=%t, want the injected failure", creatingErr, creatingFailure.failNext)
	}
	res = load()
	if res.OpsRequest.Status.Phase != opsv1alpha1.OpsPendingPhase ||
		len(res.OpsRequest.Status.Components[component].ProgressDetails) != 0 {
		t.Fatalf("failed Creating patch persisted the participant baseline: %#v", res.OpsRequest.Status)
	}
	if ptr.Deref(res.Cluster.Spec.ComponentSpecs[0].Stop, false) {
		t.Fatal("Stop mutated the Cluster before the participant baseline was persisted")
	}

	for range 4 {
		res = load()
		if res.OpsRequest.Status.Phase == opsv1alpha1.OpsCreatingPhase {
			break
		}
		if _, err := GetOpsManager().Do(reqCtx, cli, res); err != nil {
			t.Fatalf("prepare Stop through manager: %v", err)
		}
	}
	res = load()
	if res.OpsRequest.Status.Phase != opsv1alpha1.OpsCreatingPhase {
		t.Fatalf("phase=%s, want Creating after participant persistence", res.OpsRequest.Status.Phase)
	}
	details := res.OpsRequest.Status.Components[component].ProgressDetails
	if len(details) != 2 || details[0].ObjectKey != "Pod/"+active0 || details[1].ObjectKey != "Pod/"+active1 ||
		details[0].Group != component || details[1].Group != component {
		t.Fatalf("persisted participants=%v, want the two initially active identities", details)
	}

	if _, err := GetOpsManager().Do(reqCtx, cli, res); err != nil {
		t.Fatalf("apply Stop through manager: %v", err)
	}
	res = load()
	res.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
	res.OpsRequest.Status.ClusterGeneration = res.Cluster.Generation
	if err := cli.Status().Update(reqCtx.Ctx, res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	stopping := true
	its.Spec.Stop = &stopping
	its.Generation = 2
	its.Status.ObservedGeneration = 2
	its.Status.InstanceStatus = []workloads.InstanceStatus{{
		PodName: active0, DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent,
	}}
	if err := cli.Update(reqCtx.Ctx, its); err != nil {
		t.Fatal(err)
	}

	res = load()
	failing := &failNextStatusPatchClient{
		Client: cli, failNext: true, failPhase: opsv1alpha1.OpsRunningPhase,
	}
	if _, err := GetOpsManager().Reconcile(reqCtx, failing, res); err == nil {
		t.Fatal("expected the injected first progress patch to fail")
	}
	res = load()
	if len(res.OpsRequest.Status.Components[component].ProgressDetails) != 2 {
		t.Fatalf("participant baseline was lost after patch failure: %v", res.OpsRequest.Status.Components)
	}
	requeueAfter, err := GetOpsManager().Reconcile(reqCtx, cli, res)
	if err != nil {
		t.Fatalf("retry Stop reconciliation: %v", err)
	}
	if requeueAfter <= 0 {
		t.Fatal("missing participant observation did not request another reconciliation")
	}
	res = load()
	if res.OpsRequest.Status.Progress != "1/2" {
		t.Fatalf("progress=%s, want 1/2 while one persisted participant has no observation", res.OpsRequest.Status.Progress)
	}

	res.Cluster.Status.Components[component] = appsv1.ClusterComponentStatus{
		Phase: appsv1.StoppedComponentPhase, ObservedGeneration: res.Cluster.Generation, UpToDate: true,
	}
	if err := cli.Status().Update(reqCtx.Ctx, res.Cluster); err != nil {
		t.Fatal(err)
	}
	res = load()
	if _, err := GetOpsManager().Reconcile(reqCtx, cli, res); err != nil {
		t.Fatalf("complete Stop from apps status: %v", err)
	}
	res = load()
	if res.OpsRequest.Status.Phase != opsv1alpha1.OpsSucceedPhase || res.OpsRequest.Status.Progress != "1/2" {
		t.Fatalf("phase=%s progress=%s, want Succeed with independently observed progress 1/2", res.OpsRequest.Status.Phase, res.OpsRequest.Status.Progress)
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
				Template: appsv1.ClusterComponentSpec{Replicas: replicas, Stop: &stopped},
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
		WithObjects(opsRequest, shardComponent, newInstanceSet(component), newInstanceSet(physicalComponent)).Build()
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
	if err := (StopOpsHandler{}).SaveLastConfiguration(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes); err != nil {
		t.Fatalf("save Stop participants: %v", err)
	}

	phase, _, err := (StopOpsHandler{}).ReconcileAction(
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

		It("Test abort other running opsRequests", func() {
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

			By("expect the 'Restart' opsRequest with intersection component to be Aborted")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(opsv1alpha1.OpsAbortedPhase))

			By("expect the 'Restart' opsRequest with non-intersection component  to be Creating")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("expect the 'Start' opsRequest to be Aborted")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops1))).Should(Equal(opsv1alpha1.OpsAbortedPhase))
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
