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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

func TestStartTargetsStarted(t *testing.T) {
	stopped := true
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{
		ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql"}, {Name: "proxy", Stop: &stopped}},
		Shardings:      []appsv1.ClusterSharding{{Name: "shard"}},
	}}
	newOpsResource := func(targets ...string) *OpsResource {
		startList := make([]opsv1alpha1.ComponentOps, len(targets))
		for i := range targets {
			startList[i].ComponentName = targets[i]
		}
		return &OpsResource{Cluster: cluster, OpsRequest: &opsv1alpha1.OpsRequest{
			Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{StartList: startList}},
		}}
	}

	handler := StartOpsHandler{}
	if !handler.targetsStarted(newOpsResource("mysql")) {
		t.Fatal("started target was rejected")
	}
	if handler.targetsStarted(newOpsResource("proxy")) {
		t.Fatal("stopped target was accepted")
	}
	if handler.targetsStarted(newOpsResource("missing")) {
		t.Fatal("missing target was accepted")
	}
	if handler.targetsStarted(newOpsResource()) {
		t.Fatal("start-all accepted while a component remained stopped")
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
		t.Fatalf("phase=%s err=%v, want Aborted after the start target is overwritten", phase, err)
	}
}

func TestStartAllTargetsComplete(t *testing.T) {
	const (
		namespace         = "default"
		clusterName       = "cluster"
		component         = "mysql"
		shardingName      = "shard"
		physicalComponent = "shard-0"
	)
	replicas := int32(1)
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName, Generation: 8},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: component, Replicas: replicas}},
			Shardings: []appsv1.ClusterSharding{{
				Name: shardingName, Shards: 1,
				Template: appsv1.ClusterComponentSpec{Replicas: replicas},
			}},
		},
		Status: appsv1.ClusterStatus{
			Components: map[string]appsv1.ClusterComponentStatus{
				component: {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 8, UpToDate: true},
			},
			Shardings: map[string]appsv1.ClusterShardingStatus{
				shardingName: {Phase: appsv1.RunningComponentPhase, ObservedGeneration: 8, UpToDate: true},
			},
		},
	}
	opsRequest := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "start-all"},
		Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
			StartList: nil,
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
				DesiredState: workloads.InstanceDesiredStateActive,
				CurrentState: workloads.InstanceCurrentStatePresent,
				UpToDate:     true,
				Ready:        true,
				Available:    true,
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
	var err error
	opsRes.Runtimes, err = buildOpsRuntimes(context.Background(), cli, opsRes)
	if err != nil {
		t.Fatalf("build runtimes: %v", err)
	}

	phase, _, err := (StartOpsHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes)
	if err != nil {
		t.Fatalf("reconcile start-all: %v", err)
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

var _ = Describe("Start OpsRequest", func() {
	var (
		randomStr      = testCtx.GetRandomStr()
		compDefName    = "test-compdef-" + randomStr
		clusterName    = "test-luster-" + randomStr
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
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {
		createStartOpsRequest := func(opsRes *OpsResource, startCompNames ...string) *opsv1alpha1.OpsRequest {
			By("create Stop opsRequest")
			ops := testops.NewOpsRequestObj("start-ops-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.StartType)
			var startList []opsv1alpha1.ComponentOps
			for _, startCompName := range startCompNames {
				startList = append(startList, opsv1alpha1.ComponentOps{
					ComponentName: startCompName,
				})
			}
			ops.Spec.StartList = startList
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			return ops
		}

		It("Test start OpsRequest", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			By("create 'Start' opsRequest")
			createStartOpsRequest(opsRes)

			By("test start action and reconcile function")
			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
			// mock cluster phase to stopped
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1.StoppedClusterPhase
			})).ShouldNot(HaveOccurred())

			// set ops phase to Pending
			runAction(reqCtx, opsRes, opsv1alpha1.OpsCreatingPhase)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			// do start action
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
				Expect(v.Stop).Should(BeNil())
			}
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).Should(BeNil())
		})

		It("Test start specific components OpsRequest", func() {
			By("init operations resources with topology")
			opsRes, _, _ := initOperationsResourcesWithTopology(clusterDefName, compDefName, clusterName)
			// mock components is stopped
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(pobj *appsv1.Cluster) {
				for i := range pobj.Spec.ComponentSpecs {
					pobj.Spec.ComponentSpecs[i].Stop = pointer.Bool(true)
				}
			})).Should(Succeed())

			By("create 'Start' opsRequest for specific components")
			createStartOpsRequest(opsRes, defaultCompName)

			By("mock 'Start' OpsRequest to Creating phase")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			runAction(reqCtx, opsRes, opsv1alpha1.OpsCreatingPhase)

			By("test start action")
			startHandler := StartOpsHandler{}
			err := startHandler.Action(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("verify components are being started")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, pobj *appsv1.Cluster) {
				for _, v := range pobj.Spec.ComponentSpecs {
					if v.Name == defaultCompName {
						Expect(v.Stop).Should(BeNil())
					} else {
						Expect(v.Stop).ShouldNot(BeNil())
						Expect(*v.Stop).Should(BeTrue())
					}
				}
			})).Should(Succeed())

			By("mock components start successfully")
			testapps.MockInstanceSetPods(&testCtx, nil, opsRes.Cluster, defaultCompName)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			mockRollingTargetStatus(opsRes.Cluster, appsv1.RunningComponentPhase, defaultCompName)

			By("test reconcile")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("verify ops request completed")
			Eventually(testops.GetOpsRequestPhase(&testCtx,
				client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		})

		It("Test abort running 'Stop' opsRequest", func() {
			By("init operations resources with topology")
			opsRes, _, _ := initOperationsResourcesWithTopology(clusterDefName, compDefName, clusterName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}

			By("create 'Stop' opsRequest for all components")
			stopOps := createStopOpsRequest(opsRes, defaultCompName)
			runAction(reqCtx, opsRes, opsv1alpha1.OpsCreatingPhase)

			By("create a start opsRequest")
			createStartOpsRequest(opsRes, defaultCompName)
			startHandler := StartOpsHandler{}
			err := startHandler.Action(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("expect the 'Stop' OpsRequest to be Aborted")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(stopOps))).Should(Equal(opsv1alpha1.OpsAbortedPhase))
		})
	})
})
