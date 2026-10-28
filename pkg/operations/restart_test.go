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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

func TestRestartTargetsMatch(t *testing.T) {
	started := metav1.NewTime(time.Date(2026, 9, 16, 1, 2, 3, 0, time.UTC))
	annotations := map[string]string{constant.RestartAnnotationKey: started.Format(time.RFC3339)}
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{
		ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql", Annotations: annotations}},
		Shardings:      []appsv1.ClusterSharding{{Name: "shard", Template: appsv1.ClusterComponentSpec{Annotations: annotations}}},
	}}
	newOpsResource := func(targets ...string) *OpsResource {
		restartList := make([]opsv1alpha1.ComponentOps, len(targets))
		for i := range targets {
			restartList[i].ComponentName = targets[i]
		}
		return &OpsResource{Cluster: cluster, OpsRequest: &opsv1alpha1.OpsRequest{
			Spec:   opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{RestartList: restartList}},
			Status: opsv1alpha1.OpsRequestStatus{StartTimestamp: started},
		}}
	}

	handler := restartOpsHandler{}
	if !handler.targetsMatch(newOpsResource("mysql", "shard")) {
		t.Fatal("existing targets were rejected")
	}
	if handler.targetsMatch(newOpsResource("missing")) {
		t.Fatal("missing target was accepted")
	}
	opsRes := newOpsResource("missing")
	opsRes.Cluster.Generation = 7
	opsRes.OpsRequest.Status.ClusterGeneration = 8
	phase, _, err := handler.ReconcileAction(intctrlutil.RequestCtx{}, nil, opsRes)
	if err != nil || phase != opsv1alpha1.OpsRunningPhase {
		t.Fatalf("phase=%s err=%v, want Running before the action generation is observed", phase, err)
	}
	opsRes.Cluster.Generation = 8
	phase, _, err = handler.ReconcileAction(intctrlutil.RequestCtx{}, nil, opsRes)
	if err != nil || phase != opsv1alpha1.OpsAbortedPhase {
		t.Fatalf("phase=%s err=%v, want Aborted after the target is removed", phase, err)
	}
}

func TestRestartReplacedTrigger(t *testing.T) {
	for _, tc := range []struct {
		name, trigger   string
		sharding, stale bool
		want            opsv1alpha1.OpsPhase
	}{
		{name: "same trigger at a later generation", trigger: "2026-09-16T01:02:03Z", want: opsv1alpha1.OpsSucceedPhase},
		{name: "replaced trigger", trigger: "2026-09-16T01:03:03Z", want: opsv1alpha1.OpsAbortedPhase},
		{name: "removed trigger", want: opsv1alpha1.OpsAbortedPhase},
		{name: "replaced shard trigger", trigger: "2026-09-16T01:03:03Z", sharding: true, want: opsv1alpha1.OpsAbortedPhase},
		{name: "wait for action generation", stale: true, want: opsv1alpha1.OpsRunningPhase},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			component := appsv1.ClusterComponentSpec{Name: "db", Replicas: 1,
				Annotations: map[string]string{constant.RestartAnnotationKey: tc.trigger}}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 8},
				Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{component}},
				Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{"db": {
					Phase: appsv1.RunningComponentPhase, ObservedGeneration: 8, UpToDate: true,
				}}}}
			if tc.sharding {
				cluster.Spec.ComponentSpecs = nil
				cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "db", Shards: 1, Template: component}}
			}
			if tc.stale {
				cluster.Generation = 6
			}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "restart", Namespace: "default"},
				Spec: opsv1alpha1.OpsRequestSpec{ClusterName: "demo", Type: opsv1alpha1.RestartType,
					SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{RestartList: []opsv1alpha1.ComponentOps{{ComponentName: "db"}}}},
				Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, ClusterGeneration: 7,
					StartTimestamp: metav1.NewTime(time.Date(2026, 9, 16, 1, 2, 3, 0, time.UTC))}}
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
				Spec: workloads.InstanceSetSpec{Replicas: &component.Replicas},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{PodName: "demo-db-0",
					DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent,
					UpToDate: true, Ready: true, Available: true}}}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops).WithObjects(cluster, ops, its).Build()
			res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(20)}
			if _, err := GetOpsManager().Reconcile(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, res); err != nil {
				t.Fatal(err)
			}
			persisted := &opsv1alpha1.OpsRequest{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(ops), persisted); err != nil {
				t.Fatal(err)
			}
			if persisted.Status.Phase != tc.want {
				t.Fatalf("phase=%s, want %s", persisted.Status.Phase, tc.want)
			}
			if tc.want == opsv1alpha1.OpsAbortedPhase && persisted.Status.CompletionTimestamp.IsZero() {
				t.Fatal("abort was not persisted as terminal")
			}
		})
	}
}

func TestRestartUsesClusterStatusForTerminalPhase(t *testing.T) {
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

	tests := []struct {
		name             string
		componentPhase   appsv1.ComponentPhase
		upToDate         bool
		instanceFailed   bool
		wantPhase        opsv1alpha1.OpsPhase
		wantDetailStatus opsv1alpha1.ProgressStatus
	}{
		{
			name:             "instance failure remains progress while component is updating",
			componentPhase:   appsv1.UpdatingComponentPhase,
			upToDate:         true,
			instanceFailed:   true,
			wantPhase:        opsv1alpha1.OpsRunningPhase,
			wantDetailStatus: opsv1alpha1.FailedProgressStatus,
		},
		{
			name:             "running component is authoritative success",
			componentPhase:   appsv1.RunningComponentPhase,
			upToDate:         true,
			wantPhase:        opsv1alpha1.OpsSucceedPhase,
			wantDetailStatus: opsv1alpha1.SucceedProgressStatus,
		},
		{
			name:             "failed component is authoritative failure",
			componentPhase:   appsv1.FailedComponentPhase,
			upToDate:         true,
			instanceFailed:   true,
			wantPhase:        opsv1alpha1.OpsFailedPhase,
			wantDetailStatus: opsv1alpha1.FailedProgressStatus,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const (
				namespace    = "default"
				clusterName  = "cluster"
				component    = "mysql"
				instanceName = "cluster-mysql-0"
			)
			replicas := int32(1)
			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 8},
				Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
					Name: component, Replicas: replicas,
					Annotations: map[string]string{constant.RestartAnnotationKey: "2026-09-16T01:02:03Z"},
				}}},
				Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
					component: {
						Phase:              test.componentPhase,
						ObservedGeneration: 8,
						UpToDate:           test.upToDate,
					},
				}},
			}
			opsRequest := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "restart"},
				Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
					RestartList: []opsv1alpha1.ComponentOps{{ComponentName: component}},
				}},
				Status: opsv1alpha1.OpsRequestStatus{
					ClusterGeneration: 8,
					StartTimestamp:    metav1.NewTime(time.Date(2026, 9, 16, 1, 2, 3, 0, time.UTC)),
					Components:        map[string]opsv1alpha1.OpsRequestComponentStatus{},
				},
			}
			its := &workloads.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      constant.GenerateClusterComponentName(clusterName, component),
				},
				Spec: workloads.InstanceSetSpec{Replicas: &replicas},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
					PodName:      instanceName,
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					UpToDate:     true,
					Ready:        true,
					Available:    true,
					Failed:       test.instanceFailed,
				}}},
			}
			cli := fake.NewClientBuilder().WithScheme(testScheme).
				WithStatusSubresource(&opsv1alpha1.OpsRequest{}).
				WithObjects(opsRequest, its).Build()
			opsRes := &OpsResource{
				Cluster:    cluster,
				OpsRequest: opsRequest,
				Recorder:   record.NewFakeRecorder(10),
			}
			var err error
			opsRes.Runtimes, err = buildOpsRuntimes(context.Background(), cli, opsRes)
			if err != nil {
				t.Fatalf("build runtimes: %v", err)
			}

			phase, _, err := (restartOpsHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes)
			if err != nil {
				t.Fatalf("reconcile restart: %v", err)
			}
			if phase != test.wantPhase {
				t.Fatalf("phase=%s, want %s", phase, test.wantPhase)
			}
			if opsRequest.Status.Progress != "1/1" {
				t.Fatalf("progress=%s, want 1/1", opsRequest.Status.Progress)
			}
			details := opsRequest.Status.Components[component].ProgressDetails
			if len(details) != 1 || details[0].Status != test.wantDetailStatus {
				t.Fatalf("details=%v, want one %s detail", details, test.wantDetailStatus)
			}
		})
	}
}

var _ = Describe("Restart OpsRequest", func() {

	var (
		randomStr   = testCtx.GetRandomStr()
		compDefName = "test-compdef-" + randomStr
		clusterName = "test-cluster-" + randomStr
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
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {
		var (
			opsRes  *OpsResource
			cluster *appsv1.Cluster
			reqCtx  intctrlutil.RequestCtx
		)

		BeforeEach(func() {
			reqCtx = intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
		})

		It("Test restart OpsRequest", func() {
			By("init operations resources ")
			opsRes, _, cluster = initOperationsResources(compDefName, clusterName)
			By("create Restart opsRequest")
			opsRes.OpsRequest = createRestartOpsObj(clusterName, "restart-ops-"+randomStr)
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName)

			By("mock restart OpsRequest to Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("test restart action and reconcile function")
			rHandler := restartOpsHandler{}
			_ = rHandler.Action(reqCtx, k8sClient, opsRes)

			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("restarts distinct requests with the same persisted start second without repeating retries", func() {
			opsRes, _, cluster = initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			pods := testapps.MockInstanceSetPods(&testCtx, nil, cluster, defaultCompName)
			mockRunningInstanceStatus(cluster, defaultCompName)
			started := metav1.NewTime(time.Now().Truncate(time.Second))
			initialGeneration := cluster.Generation

			startRequest := func(name string) {
				opsRes.OpsRequest = createRestartOpsObj(clusterName, name)
				_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))
				opsRes.OpsRequest.Status.StartTimestamp = started
				Expect(k8sClient.Status().Update(ctx, opsRes.OpsRequest)).Should(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
				Expect(opsRes.OpsRequest.Status.StartTimestamp).Should(Equal(started))
				_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
				Expect(err).ShouldNot(HaveOccurred())
			}

			By("completing the first request with its observed generation")
			startRequest("restart-first-" + randomStr)
			opsRes.OpsRequest.Status.ClusterGeneration = opsRes.Cluster.Generation
			Expect(PatchOpsStatus(ctx, k8sClient, opsRes, opsv1alpha1.OpsRunningPhase)).Should(Succeed())
			first := opsRes.OpsRequest.DeepCopy()
			firstTrigger := opsRes.Cluster.Spec.ComponentSpecs[0].Annotations[constant.RestartAnnotationKey]
			Expect(opsRes.Cluster.Generation).Should(Equal(initialGeneration + 1))
			mockRollingTargetStatus(cluster, appsv1.RunningComponentPhase, defaultCompName)
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))

			By("starting a distinct request in the identical persisted second")
			startRequest("restart-second-" + randomStr)
			Expect(opsRes.OpsRequest.UID).ShouldNot(Equal(first.UID))
			Expect(opsRes.OpsRequest.Status.StartTimestamp).Should(Equal(first.Status.StartTimestamp))
			secondTrigger := opsRes.Cluster.Spec.ComponentSpecs[0].Annotations[constant.RestartAnnotationKey]
			Expect(secondTrigger).ShouldNot(Equal(firstTrigger))
			Expect(opsRes.Cluster.Generation).Should(Equal(initialGeneration + 2))

			By("retrying after the Cluster update without changing its restart trigger or generation")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.Cluster.Generation).Should(Equal(initialGeneration + 2))
			Expect(opsRes.Cluster.Spec.ComponentSpecs[0].Annotations[constant.RestartAnnotationKey]).Should(Equal(secondTrigger))
			opsRes.OpsRequest.Status.ClusterGeneration = opsRes.Cluster.Generation
			Expect(PatchOpsStatus(ctx, k8sClient, opsRes, opsv1alpha1.OpsRunningPhase)).Should(Succeed())

			By("waiting while ready Pods and component observations still describe the first restart")
			for _, pod := range pods {
				stored := pod.DeepCopy()
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), stored)).Should(Succeed())
				Expect(stored.UID).Should(Equal(pod.UID))
			}
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
		})

		It("preserves restart ordering and matches request identity for components and shardings", func() {
			started := metav1.NewTime(time.Date(2026, 9, 16, 1, 2, 3, 0, time.UTC))
			ops := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{UID: "current-request"},
				Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
					RestartList: []opsv1alpha1.ComponentOps{{ComponentName: "db"}},
				}},
				Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: started},
			}
			current := started.Format(time.RFC3339) + "/" + string(ops.UID)
			for _, sharding := range []bool{false, true} {
				for _, test := range []struct {
					trigger, want string
				}{
					{"", current},
					{"2026-09-16T01:02:02Z", current},
					{"2026-09-16T01:02:03Z", current},
					{"2026-09-16T01:02:03Z/previous-request", current},
					{current, current},
					{"2026-09-16T01:02:04Z", "2026-09-16T01:02:04Z"},
					{"2026-09-16T01:02:04Z/later-request", "2026-09-16T01:02:04Z/later-request"},
				} {
					spec := appsv1.ClusterComponentSpec{Name: "db", Annotations: map[string]string{constant.RestartAnnotationKey: test.trigger}}
					target := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{spec}}}
					if sharding {
						target.Spec.ComponentSpecs = nil
						target.Spec.Shardings = []appsv1.ClusterSharding{{Name: "db", Template: spec}}
					}
					res := &OpsResource{Cluster: target, OpsRequest: ops}
					handler := restartOpsHandler{compOpsHelper: newComponentOpsHelper(ops.Spec.RestartList)}
					handler.doRestart(res, &spec, "db")
					Expect(spec.Annotations[constant.RestartAnnotationKey]).Should(Equal(test.want))
					Expect(handler.targetsMatch(res)).Should(Equal(test.want == current))
					spec.Annotations[constant.RestartAnnotationKey] = started.Format(time.RFC3339)
					Expect(handler.targetsMatch(res)).Should(BeTrue(), "legacy running requests must survive an operator upgrade")
					spec.Annotations[constant.RestartAnnotationKey] = started.Format(time.RFC3339) + "/replacement-request"
					Expect(handler.targetsMatch(res)).Should(BeFalse(), "a replacement in the same second must not match the old request")
				}
			}
		})

		It("expect failed when cluster is stopped", func() {
			By("init operations resources ")
			opsRes, _, cluster = initOperationsResources(compDefName, clusterName)
			By("mock cluster is stopped")
			Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
				cluster.Status.Phase = appsv1.StoppedClusterPhase
			})).Should(Succeed())
			By("create Restart opsRequest")
			opsRes.OpsRequest = createRestartOpsObj(clusterName, "restart-ops-"+randomStr)
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest),
				func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
					g.Expect(fetched.Status.Phase).To(Equal(opsv1alpha1.OpsFailedPhase))
					condition := meta.FindStatusCondition(fetched.Status.Conditions, opsv1alpha1.ConditionTypeValidated)
					g.Expect(condition.Message).Should(Equal("OpsRequest.spec.type=Restart is forbidden when Cluster.status.phase=Stopped"))
				})).Should(Succeed())
		})
	})
})

func createRestartOpsObj(clusterName, restartOpsName string, compNames ...string) *opsv1alpha1.OpsRequest {
	ops := testops.NewOpsRequestObj(restartOpsName, testCtx.DefaultNamespace,
		clusterName, opsv1alpha1.RestartType)
	if len(compNames) == 0 {
		ops.Spec.RestartList = []opsv1alpha1.ComponentOps{
			{ComponentName: defaultCompName},
		}
	} else {
		for _, compName := range compNames {
			ops.Spec.RestartList = append(ops.Spec.RestartList, opsv1alpha1.ComponentOps{
				ComponentName: compName,
			})
		}
	}
	opsRequest := testops.CreateOpsRequest(ctx, testCtx, ops)
	opsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
	return opsRequest
}
