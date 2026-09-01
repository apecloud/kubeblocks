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

func TestRestartTargetsExist(t *testing.T) {
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{
		ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql"}},
		Shardings:      []appsv1.ClusterSharding{{Name: "shard"}},
	}}
	newOpsResource := func(targets ...string) *OpsResource {
		restartList := make([]opsv1alpha1.ComponentOps, len(targets))
		for i := range targets {
			restartList[i].ComponentName = targets[i]
		}
		return &OpsResource{Cluster: cluster, OpsRequest: &opsv1alpha1.OpsRequest{
			Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{RestartList: restartList}},
		}}
	}

	handler := restartOpsHandler{}
	if !handler.targetsExist(newOpsResource("mysql", "shard")) {
		t.Fatal("existing targets were rejected")
	}
	if handler.targetsExist(newOpsResource("missing")) {
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
		wantPhase        opsv1alpha1.OpsPhase
		wantDetailStatus opsv1alpha1.ProgressStatus
	}{
		{
			name:             "instance failure remains progress while component is updating",
			componentPhase:   appsv1.UpdatingComponentPhase,
			upToDate:         true,
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
					Failed:       true,
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
