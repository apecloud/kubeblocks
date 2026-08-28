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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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

func TestStoppedInstanceProgress(t *testing.T) {
	const instanceName = "cluster-mysql-0"
	replicas := int32(1)
	opsRes := &OpsResource{
		Cluster:    &appsv1.Cluster{},
		OpsRequest: &opsv1alpha1.OpsRequest{},
		Recorder:   record.NewFakeRecorder(10),
	}
	pgRes := &progressResource{
		opsMessageKey:     "stop",
		fullComponentName: "mysql",
		clusterComponent:  &appsv1.ClusterComponentSpec{Name: "mysql", Replicas: replicas},
	}
	compStatus := &opsv1alpha1.OpsRequestComponentStatus{}
	its := &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{Replicas: &replicas},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
			PodName:      instanceName,
			CurrentState: workloads.InstanceCurrentStatePresent,
		}}},
	}

	expected, completed := handleStoppedInstanceProgress(opsRes, pgRes, compStatus, its)
	if expected != 1 || completed != 0 {
		t.Fatalf("progress=%d/%d, want 0/1 while the instance is present", completed, expected)
	}
	its.Status.InstanceStatus[0].CurrentState = workloads.InstanceCurrentStateAbsent
	expected, completed = handleStoppedInstanceProgress(opsRes, pgRes, compStatus, its)
	if expected != 1 || completed != 1 {
		t.Fatalf("progress=%d/%d, want 1/1", completed, expected)
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
