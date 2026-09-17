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
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
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

var _ = Describe("VerticalScaling OpsRequest", func() {

	var (
		randomStr   = testCtx.GetRandomStr()
		compDefName = "test-compdef-" + randomStr
		clusterName = "test-cluster-" + randomStr
		reqCtx      intctrlutil.RequestCtx
	)

	cleanEnv := func() {
		reqCtx = intctrlutil.RequestCtx{Ctx: ctx}
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
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {

		newResources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("400m"),
				corev1.ResourceMemory: resource.MustParse("300Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("400m"),
				corev1.ResourceMemory: resource.MustParse("300Mi"),
			},
		}

		testVerticalScaling := func(verticalScaling []opsv1alpha1.VerticalScaling, instances []appsv1.InstanceTemplate) *OpsResource {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			if len(instances) > 0 {
				Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(cluster *appsv1.Cluster) {
					cluster.Spec.ComponentSpecs[0].Instances = instances
				})).Should(Succeed())
			}
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			if len(instances) > 0 {
				its := &workloads.InstanceSet{}
				key := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
					Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, defaultCompName)}
				Expect(k8sClient.Get(ctx, key, its)).Should(Succeed())
				its.Spec.Instances = make([]workloads.InstanceTemplate, 0, len(instances))
				for i := range instances {
					its.Spec.Instances = append(its.Spec.Instances, workloads.InstanceTemplate{
						Name: instances[i].Name, Replicas: instances[i].Replicas, Resources: instances[i].Resources,
					})
				}
				Expect(k8sClient.Update(ctx, its)).Should(Succeed())
			}
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			By("create VerticalScaling ops")
			ops := testops.NewOpsRequestObj("vertical-scaling-ops-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.VerticalScalingType)

			ops.Spec.VerticalScalingList = verticalScaling
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("test save last configuration and OpsRequest phase is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("test vertical scale action function")
			vsHandler := verticalScalingHandler{}
			Expect(vsHandler.Action(reqCtx, k8sClient, opsRes)).Should(Succeed())
			_, _, err = vsHandler.ReconcileAction(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			return opsRes
		}

		setInstanceProgress := func(opsRes *OpsResource, readyNames ...string) []workloads.InstanceStatus {
			ready := make(map[string]struct{}, len(readyNames))
			for _, name := range readyNames {
				ready[name] = struct{}{}
			}
			its := &workloads.InstanceSet{}
			key := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
				Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, defaultCompName)}
			Expect(k8sClient.Get(ctx, key, its)).Should(Succeed())
			for i := range its.Status.InstanceStatus {
				status := &its.Status.InstanceStatus[i]
				_, done := ready[status.PodName]
				status.CurrentState = workloads.InstanceCurrentStatePresent
				status.UpToDate = done
				status.Ready = done
				status.Available = done
			}
			its.Status.ObservedGeneration = its.Generation
			Expect(k8sClient.Status().Update(ctx, its)).Should(Succeed())
			return its.Status.InstanceStatus
		}

		It("vertical scaling by resource", func() {
			verticalScaling := []opsv1alpha1.VerticalScaling{
				{
					ComponentOps:         opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					ResourceRequirements: newResources,
				},
			}
			testVerticalScaling(verticalScaling, nil)
		})

		It("preserves payload updates and cancellation in the existing API", func() {
			ops := testops.NewOpsRequestObj("vertical-update-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.VerticalScalingType)
			ops.Spec.VerticalScalingList = []opsv1alpha1.VerticalScaling{{
				ComponentOps:         opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ResourceRequirements: newResources,
			}}
			Expect(k8sClient.Create(ctx, ops)).To(Succeed())
			current := ops.DeepCopy()
			current.Spec.VerticalScalingList[0].Requests[corev1.ResourceCPU] = resource.MustParse("800m")
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), current)).To(Succeed())
			current.Spec.VerticalScalingList = nil
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), current)).To(Succeed())
			current.Spec.VerticalScalingList = ops.Spec.VerticalScalingList
			current.Spec.Cancel = true
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
		})

		It("cancels through the operation entry path using current instance status", func() {
			verticalScaling := []opsv1alpha1.VerticalScaling{{
				ComponentOps:         opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ResourceRequirements: newResources,
			}}
			opsRes := testVerticalScaling(verticalScaling, nil)
			statuses := setInstanceProgress(opsRes)
			Expect(statuses).ShouldNot(BeEmpty())
			setInstanceProgress(opsRes, statuses[0].PodName)
			_, _, err := (verticalScalingHandler{}).ReconcileAction(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			cancelOpsRequest(reqCtx, opsRes, time.Now())
			readyNames := make([]string, 0, len(statuses))
			for i := range statuses {
				readyNames = append(readyNames, statuses[i].PodName)
			}
			setInstanceProgress(opsRes, readyNames...)
			mockRollingTargetStatus(opsRes.Cluster, appsv1.RunningComponentPhase, defaultCompName)

			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCancelledPhase))
			details := opsRes.OpsRequest.Status.Components[defaultCompName].ProgressDetails
			Expect(details).ShouldNot(BeEmpty())
			for _, detail := range details {
				Expect(detail.Message).Should(ContainSubstring("rollback"))
			}
		})

		It("vertical scaling the component which existing instance template", func() {
			templateName := "foo"
			verticalScaling := []opsv1alpha1.VerticalScaling{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					Instances: []opsv1alpha1.InstanceResourceTemplate{
						{Name: templateName, ResourceRequirements: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("300Mi"),
							},
						}},
					},
					ResourceRequirements: newResources,
				},
			}
			opsRes := testVerticalScaling(verticalScaling, []appsv1.InstanceTemplate{{Name: templateName, Replicas: pointer.Int32(1)}})
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("0/3"))
		})

		It("vertical scaling the component which existing instance template with ordinal ranges", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			templateName := "foo-ranges"
			verticalScaling := []opsv1alpha1.VerticalScaling{
				{
					ComponentOps:         opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					ResourceRequirements: newResources,
				},
			}
			opsRes := testVerticalScaling(verticalScaling, []appsv1.InstanceTemplate{{Name: templateName, Replicas: pointer.Int32(1), Ordinals: appsv1.Ordinals{Ranges: []appsv1.Range{{Start: 300, End: 600}}}}})
			podNames := []string{
				fmt.Sprintf("%s-%s-0", clusterName, defaultCompName),
				fmt.Sprintf("%s-%s-1", clusterName, defaultCompName),
				fmt.Sprintf("%s-%s-%s-300", clusterName, defaultCompName, templateName),
			}
			By("mock ops running")
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName)
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.OpsRequest, func() {
				opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
				opsRes.OpsRequest.Status.ClusterGeneration = opsRes.Cluster.Generation
			})).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("0/3"))

			By("publishing one applied instance")
			setInstanceProgress(opsRes, podNames[0])

			By("reconcile opsRequest status")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/3"))

			By("publishing the remaining applied instances")
			setInstanceProgress(opsRes, podNames...)

			By("mock cluster running")
			mockRollingTargetStatus(opsRes.Cluster, appsv1.RunningComponentPhase, defaultCompName)

			By("reconcile opsRequest status")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("3/3"))
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		})

		It("vertical scaling the replicas which instance template is empty", func() {
			templateName := "foo"
			verticalScaling := []opsv1alpha1.VerticalScaling{
				{
					ComponentOps:         opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					ResourceRequirements: newResources,
				},
			}
			opsRes := testVerticalScaling(verticalScaling, []appsv1.InstanceTemplate{{Name: templateName, Replicas: pointer.Int32(1), Resources: &newResources}})
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("0/2"))
		})

		It("vertical scaling the instance template", func() {
			templateName := "foo"
			verticalScaling := []opsv1alpha1.VerticalScaling{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					Instances: []opsv1alpha1.InstanceResourceTemplate{
						{Name: templateName, ResourceRequirements: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("300Mi"),
							},
						}},
					},
				},
			}
			opsRes := testVerticalScaling(verticalScaling, []appsv1.InstanceTemplate{{Name: templateName, Replicas: pointer.Int32(1)}})
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("0/1"))
		})

		It("force run vertical scaling opsRequests", func() {
			By("create the first vertical scaling")
			verticalScaling1 := []opsv1alpha1.VerticalScaling{
				{
					ComponentOps:         opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					ResourceRequirements: newResources,
				},
			}
			opsRes := testVerticalScaling(verticalScaling1, nil)
			firstOpsRequest := opsRes.OpsRequest.DeepCopy()
			Expect(testapps.ChangeObjStatus(&testCtx, firstOpsRequest, func() {
				firstOpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
			})).Should(Succeed())

			By("create the second opsRequest with force flag")
			ops := testops.NewOpsRequestObj("vertical-scaling-ops-1-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.VerticalScalingType)
			ops.Spec.Force = true
			ops.Spec.VerticalScalingList = []opsv1alpha1.VerticalScaling{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: newResources.Requests,
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
					},
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock the first reconcile and expect the opsPhase to Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("mock the next reconcile")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() {
				ops.Status.Phase = opsv1alpha1.OpsRunningPhase
			})).Should(Succeed())

			By("the first operations request is expected to be aborted.")
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(firstOpsRequest))).Should(Equal(opsv1alpha1.OpsAbortedPhase))
			opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster)
			Expect(len(opsRequestSlice)).Should(Equal(1))
		})
	})
})

func TestVerticalScalingResultAndProgressConvergeIndependently(t *testing.T) {
	target := verticalScalingTestResources("2")
	component := appsv1.ClusterComponentSpec{
		Name: "db", Replicas: 2, Resources: target,
		Instances: []appsv1.InstanceTemplate{{Name: "reader", Replicas: ptr.To[int32](1)}},
	}
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, ResourceRequirements: target,
	}
	ready := []workloads.InstanceStatus{
		verticalScalingTestInstance("demo-db-0", "", true),
		verticalScalingTestInstance("demo-db-reader-0", "reader", true),
	}

	for _, tc := range []struct {
		name       string
		phase      appsv1.ComponentPhase
		generation int64
		upToDate   bool
		instances  []workloads.InstanceStatus
		staleITS   bool
		wantPhase  opsv1alpha1.OpsPhase
		progress   string
	}{
		{name: "same target written at a later generation", phase: appsv1.RunningComponentPhase, generation: 7, upToDate: true,
			instances: ready, wantPhase: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "apps result waits", phase: appsv1.UpdatingComponentPhase, generation: 7, upToDate: true,
			instances: ready, wantPhase: opsv1alpha1.OpsRunningPhase, progress: "2/2"},
		{name: "apps observation is stale", phase: appsv1.RunningComponentPhase, generation: 6, upToDate: true,
			instances: ready, wantPhase: opsv1alpha1.OpsRunningPhase, progress: "2/2"},
		{name: "apps target is not up to date", phase: appsv1.RunningComponentPhase, generation: 7, upToDate: false,
			instances: ready, wantPhase: opsv1alpha1.OpsRunningPhase, progress: "2/2"},
		{name: "apps result failed", phase: appsv1.FailedComponentPhase, generation: 7, upToDate: true,
			instances: ready, wantPhase: opsv1alpha1.OpsFailedPhase, progress: "2/2"},
		{name: "workload observation is stale", phase: appsv1.RunningComponentPhase, generation: 7, upToDate: true,
			instances: ready, staleITS: true, wantPhase: opsv1alpha1.OpsRunningPhase, progress: "2/2"},
		{name: "current progress is incomplete", phase: appsv1.RunningComponentPhase, generation: 7, upToDate: true,
			instances: ready[:1], wantPhase: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cluster := verticalScalingTestCluster(component, tc.phase, tc.generation, tc.upToDate)
			ops := verticalScalingTestOps(request)
			ops.Status.ClusterGeneration = 6
			its := verticalScalingTestInstanceSet(component, tc.instances...)
			if tc.staleITS {
				its.Generation = 2
				its.Status.ObservedGeneration = 1
			}
			cli := verticalScalingTestClient(t, cluster, ops, its)
			phase, _, err := (verticalScalingHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
			if err != nil {
				t.Fatal(err)
			}
			if phase != tc.wantPhase || ops.Status.Progress != tc.progress {
				t.Fatalf("got phase=%s progress=%s, want phase=%s progress=%s", phase, ops.Status.Progress, tc.wantPhase, tc.progress)
			}
		})
	}
}

func TestVerticalScalingProgressUsesOnlySelectedTemplate(t *testing.T) {
	target := verticalScalingTestResources("2")
	other := verticalScalingTestResources("4")
	component := appsv1.ClusterComponentSpec{
		Name: "db", Replicas: 2,
		Instances: []appsv1.InstanceTemplate{
			{Name: "selected", Replicas: ptr.To[int32](1), Resources: &target},
			{Name: "other", Replicas: ptr.To[int32](1), Resources: &other},
		},
	}
	request := opsv1alpha1.VerticalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
		Instances: []opsv1alpha1.InstanceResourceTemplate{{Name: "selected", ResourceRequirements: target}}}
	cluster := verticalScalingTestCluster(component, appsv1.RunningComponentPhase, 7, true)
	ops := verticalScalingTestOps(request)
	its := verticalScalingTestInstanceSet(component,
		verticalScalingTestInstance("demo-db-selected-0", "selected", true),
		verticalScalingTestInstance("demo-db-other-0", "other", false),
	)
	cli := verticalScalingTestClient(t, cluster, ops, its)

	phase, _, err := (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsSucceedPhase || ops.Status.Progress != "1/1" {
		t.Fatalf("got phase=%s progress=%s", phase, ops.Status.Progress)
	}
	details := ops.Status.Components["db"].ProgressDetails
	if len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-selected-0" {
		t.Fatalf("unexpected progress details: %#v", details)
	}
}

func TestVerticalScalingDistinguishesZeroWorkFromMissingObservation(t *testing.T) {
	target := verticalScalingTestResources("2")
	for _, replicas := range []int32{0, 1} {
		t.Run(resource.NewQuantity(int64(replicas), resource.DecimalSI).String(), func(t *testing.T) {
			component := appsv1.ClusterComponentSpec{Name: "db", Replicas: replicas,
				Instances: []appsv1.InstanceTemplate{{Name: "selected", Replicas: ptr.To(replicas), Resources: &target}}}
			request := opsv1alpha1.VerticalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
				Instances: []opsv1alpha1.InstanceResourceTemplate{{Name: "selected", ResourceRequirements: target}}}
			cluster := verticalScalingTestCluster(component, appsv1.RunningComponentPhase, 7, true)
			ops := verticalScalingTestOps(request)
			cli := verticalScalingTestClient(t, cluster, ops)
			phase, _, err := (verticalScalingHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
			if err != nil {
				t.Fatal(err)
			}
			wantPhase, wantProgress := opsv1alpha1.OpsRunningPhase, "0/1"
			if replicas == 0 {
				wantPhase, wantProgress = opsv1alpha1.OpsSucceedPhase, "0/0"
			}
			if phase != wantPhase || ops.Status.Progress != wantProgress {
				t.Fatalf("got phase=%s progress=%s, want phase=%s progress=%s", phase, ops.Status.Progress, wantPhase, wantProgress)
			}
		})
	}
}

func TestVerticalScalingCancellationUsesLastConfigurationAndRefreshesProgress(t *testing.T) {
	target := verticalScalingTestResources("2")
	previous := verticalScalingTestResources("1")
	component := appsv1.ClusterComponentSpec{Name: "db", Replicas: 1, Resources: previous}
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, ResourceRequirements: target,
	}
	cluster := verticalScalingTestCluster(component, appsv1.RunningComponentPhase, 7, true)
	ops := verticalScalingTestOps(request)
	ops.Spec.Cancel = true
	ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
	ops.Status.LastConfiguration.Components = map[string]opsv1alpha1.LastComponentConfiguration{
		"db": {ResourceRequirements: previous},
	}
	instance := verticalScalingTestInstance("demo-db-0", "", false)
	instance.Failed = true
	its := verticalScalingTestInstanceSet(component, instance)
	cli := verticalScalingTestClient(t, cluster, ops, its)
	handler := verticalScalingHandler{}
	reconcile := func(wantPhase opsv1alpha1.OpsPhase, wantProgress opsv1alpha1.ProgressStatus) {
		t.Helper()
		phase, _, err := handler.ReconcileAction(
			intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
		if err != nil {
			t.Fatal(err)
		}
		detail := ops.Status.Components["db"].ProgressDetails[0]
		if phase != wantPhase || detail.Status != wantProgress {
			t.Fatalf("got phase=%s detail=%s, want phase=%s detail=%s", phase, detail.Status, wantPhase, wantProgress)
		}
		if wantProgress == opsv1alpha1.ProcessingProgressStatus && !detail.EndTime.IsZero() {
			t.Fatalf("processing detail retained terminal end time: %#v", detail)
		}
	}

	reconcile(opsv1alpha1.OpsRunningPhase, opsv1alpha1.FailedProgressStatus)
	its.Status.InstanceStatus[0].Failed = false
	its.Status.InstanceStatus[0].UpToDate = false
	if err := cli.Status().Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	reconcile(opsv1alpha1.OpsRunningPhase, opsv1alpha1.ProcessingProgressStatus)
	its.Status.InstanceStatus[0].UpToDate = true
	its.Status.InstanceStatus[0].Ready = true
	its.Status.InstanceStatus[0].Available = true
	if err := cli.Status().Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	reconcile(opsv1alpha1.OpsSucceedPhase, opsv1alpha1.SucceedProgressStatus)
	if message := ops.Status.Components["db"].ProgressDetails[0].Message; message == "" ||
		!containsAll(message, "rollback", "demo-db-0") {
		t.Fatalf("unexpected cancellation progress message %q", message)
	}
}

func TestVerticalScalingReplacedTarget(t *testing.T) {
	for _, template := range []bool{false, true} {
		for _, tc := range []struct {
			name, current string
			cancel, stale bool
			want          opsv1alpha1.OpsPhase
		}{
			{name: "same target", current: "2", want: opsv1alpha1.OpsSucceedPhase},
			{name: "replaced target", current: "3", want: opsv1alpha1.OpsAbortedPhase},
			{name: "wait for action generation", current: "3", stale: true, want: opsv1alpha1.OpsRunningPhase},
			{name: "cancel reaches original target", current: "1", cancel: true, want: opsv1alpha1.OpsCancelledPhase},
			{name: "cancel target replaced", current: "3", cancel: true, want: opsv1alpha1.OpsAbortedPhase},
		} {
			t.Run(fmt.Sprintf("%s/template=%t", tc.name, template), func(t *testing.T) {
				target, previous, current := verticalScalingTestResources("2"), verticalScalingTestResources("1"), verticalScalingTestResources(tc.current)
				component := appsv1.ClusterComponentSpec{Name: "db", Replicas: 1, Resources: current}
				request := opsv1alpha1.VerticalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, ResourceRequirements: target}
				last := opsv1alpha1.LastComponentConfiguration{ResourceRequirements: previous}
				instance := verticalScalingTestInstance("demo-db-0", "", true)
				if template {
					component.Instances = []appsv1.InstanceTemplate{{Name: "reader", Replicas: ptr.To[int32](1), Resources: &current}}
					request.ResourceRequirements = corev1.ResourceRequirements{}
					request.Instances = []opsv1alpha1.InstanceResourceTemplate{{Name: "reader", ResourceRequirements: target}}
					last.InstanceTemplates = []appsv1.InstanceTemplate{{Name: "reader", Resources: &previous}}
					instance = verticalScalingTestInstance("demo-db-reader-0", "reader", true)
				}
				cluster := verticalScalingTestCluster(component, appsv1.RunningComponentPhase, 7, true)
				ops := verticalScalingTestOps(request)
				ops.Spec.Type, ops.Spec.ClusterName = opsv1alpha1.VerticalScalingType, cluster.Name
				ops.Status.ClusterGeneration = 6
				if tc.stale {
					ops.Status.ClusterGeneration = 8
				}
				if tc.cancel {
					ops.Spec.Cancel, ops.Status.Phase = true, opsv1alpha1.OpsCancellingPhase
					ops.Status.LastConfiguration.Components["db"] = last
				}
				its := verticalScalingTestInstanceSet(component, instance)
				cli := verticalScalingTestClient(t, cluster, ops, its)
				if _, err := GetOpsManager().Reconcile(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops)); err != nil {
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
}

func TestVerticalScalingShardingProgressWaitsForEveryCurrentShard(t *testing.T) {
	target := verticalScalingTestResources("2")
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"}, ResourceRequirements: target,
	}
	for _, childCount := range []int{1, 2} {
		t.Run(resource.NewQuantity(int64(childCount), resource.DecimalSI).String(), func(t *testing.T) {
			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 7},
				Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
					Name: "shard", Shards: 2, Template: appsv1.ClusterComponentSpec{Replicas: 1, Resources: target},
				}}},
				Status: appsv1.ClusterStatus{Shardings: map[string]appsv1.ClusterShardingStatus{"shard": {
					Phase: appsv1.RunningComponentPhase, ObservedGeneration: 7, UpToDate: true,
				}}},
			}
			ops := verticalScalingTestOps(request)
			objects := []client.Object{cluster, ops}
			for i := 0; i < childCount; i++ {
				name := fmt.Sprintf("shard-%d", i)
				labels := constant.GetClusterLabels("demo", map[string]string{
					constant.KBAppShardingNameLabelKey: "shard",
				})
				labels[constant.KBAppComponentLabelKey] = name
				objects = append(objects,
					&appsv1.Component{ObjectMeta: metav1.ObjectMeta{
						Name: "demo-" + name, Namespace: "default", Labels: labels,
					}, Spec: appsv1.ComponentSpec{Replicas: 1}},
					&workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name, Namespace: "default"},
						Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](1)},
						Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
							verticalScalingTestInstance("demo-"+name+"-0", "", true),
						}}},
				)
			}
			cli := verticalScalingTestClient(t, objects...)
			phase, _, err := (verticalScalingHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
			if err != nil {
				t.Fatal(err)
			}
			wantPhase := opsv1alpha1.OpsRunningPhase
			if childCount == 2 {
				wantPhase = opsv1alpha1.OpsSucceedPhase
			}
			if phase != wantPhase || ops.Status.Progress != fmt.Sprintf("%d/2", childCount) {
				t.Fatalf("got phase=%s progress=%s, want phase=%s progress=%d/2", phase, ops.Status.Progress, wantPhase, childCount)
			}
		})
	}
}

func TestVerticalScalingShardingProgressDoesNotCrossCompensate(t *testing.T) {
	target := verticalScalingTestResources("2")
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"}, ResourceRequirements: target,
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 7},
		Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
			Name: "shard", Shards: 2, Template: appsv1.ClusterComponentSpec{Replicas: 1, Resources: target},
		}}},
		Status: appsv1.ClusterStatus{Shardings: map[string]appsv1.ClusterShardingStatus{"shard": {
			Phase: appsv1.RunningComponentPhase, ObservedGeneration: 7, UpToDate: true,
		}}},
	}
	labels := constant.GetClusterLabels("demo", map[string]string{constant.KBAppShardingNameLabelKey: "shard"})
	labels[constant.KBAppComponentLabelKey] = "shard-0"
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Name: "demo-shard-0", Namespace: "default", Labels: labels,
	}, Spec: appsv1.ComponentSpec{Replicas: 2}}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-shard-0", Namespace: "default"},
		Spec:       workloads.InstanceSetSpec{Replicas: ptr.To[int32](2)},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
			verticalScalingTestInstance("demo-shard-0-0", "", true),
			verticalScalingTestInstance("demo-shard-0-1", "", true),
		}},
	}
	ops := verticalScalingTestOps(request)
	cli := verticalScalingTestClient(t, cluster, component, its, ops)
	phase, _, err := (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != "2/3" {
		t.Fatalf("got phase=%s progress=%s, want phase=%s progress=2/3", phase, ops.Status.Progress, opsv1alpha1.OpsRunningPhase)
	}
}

func TestVerticalScalingShardingUsesPhysicalTemplateInheritanceAndCoverage(t *testing.T) {
	target := verticalScalingTestResources("2")
	override := verticalScalingTestResources("4")
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"}, ResourceRequirements: target,
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 7},
		Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
			Name: "shard", Shards: 2, Template: appsv1.ClusterComponentSpec{
				Replicas: 1, Resources: target,
				Instances: []appsv1.InstanceTemplate{{Name: "reader", Replicas: ptr.To[int32](1), Resources: &override}},
			},
		}}},
		Status: appsv1.ClusterStatus{Shardings: map[string]appsv1.ClusterShardingStatus{"shard": {
			Phase: appsv1.RunningComponentPhase, ObservedGeneration: 7, UpToDate: true,
		}}},
	}
	labels := constant.GetClusterLabels("demo", map[string]string{constant.KBAppShardingNameLabelKey: "shard"})
	labels[constant.KBAppComponentLabelKey] = "shard-0"
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Name: "demo-shard-0", Namespace: "default", Labels: labels,
	}, Spec: appsv1.ComponentSpec{Replicas: 1, Instances: []appsv1.InstanceTemplate{{
		Name: "reader", Replicas: ptr.To[int32](1),
	}}}}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-shard-0", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](1), Instances: []workloads.InstanceTemplate{{
			Name: "reader", Replicas: ptr.To[int32](1), Resources: nil,
		}}},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
			verticalScalingTestInstance("demo-shard-0-reader-0", "reader", true),
		}},
	}
	ops := verticalScalingTestOps(request)
	cli := verticalScalingTestClient(t, cluster, component, its, ops)
	phase, _, err := (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != "1/1" {
		t.Fatalf("got phase=%s progress=%s, want phase=%s progress=1/1", phase, ops.Status.Progress, opsv1alpha1.OpsRunningPhase)
	}
	second := component.DeepCopy()
	second.Name = "demo-shard-1"
	second.ResourceVersion = ""
	second.Labels[constant.KBAppComponentLabelKey] = "shard-1"
	if err := cli.Create(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	phase, _, err = (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != "1/2" {
		t.Fatalf("missing physical workload got phase=%s progress=%s, want phase=%s progress=1/2", phase, ops.Status.Progress, opsv1alpha1.OpsRunningPhase)
	}
}

func verticalScalingTestResources(cpu string) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu)}}
}

func verticalScalingTestCluster(component appsv1.ClusterComponentSpec, phase appsv1.ComponentPhase,
	observedGeneration int64, upToDate bool) *appsv1.Cluster {
	return &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 7},
		Spec:       appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{component}},
		Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{"db": {
			Phase: phase, ObservedGeneration: observedGeneration, UpToDate: upToDate,
		}}},
	}
}

func verticalScalingTestOps(request opsv1alpha1.VerticalScaling) *opsv1alpha1.OpsRequest {
	return &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "scale", Namespace: "default"},
		Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
			VerticalScalingList: []opsv1alpha1.VerticalScaling{request},
		}},
		Status: opsv1alpha1.OpsRequestStatus{
			Phase: opsv1alpha1.OpsRunningPhase, ClusterGeneration: 7,
			LastConfiguration: opsv1alpha1.LastConfiguration{Components: map[string]opsv1alpha1.LastComponentConfiguration{}},
		},
	}
}

func verticalScalingTestInstance(name, template string, ready bool) workloads.InstanceStatus {
	return workloads.InstanceStatus{
		PodName: name, TemplateName: ptr.To(template), DesiredState: workloads.InstanceDesiredStateActive,
		CurrentState: workloads.InstanceCurrentStatePresent, UpToDate: true, Ready: ready, Available: ready,
	}
}

func verticalScalingTestInstanceSet(component appsv1.ClusterComponentSpec,
	instances ...workloads.InstanceStatus) *workloads.InstanceSet {
	templates := make([]workloads.InstanceTemplate, 0, len(component.Instances))
	for i := range component.Instances {
		template := &component.Instances[i]
		templates = append(templates, workloads.InstanceTemplate{Name: template.Name, Replicas: template.Replicas})
	}
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
		Spec:       workloads.InstanceSetSpec{Replicas: ptr.To(component.Replicas), Instances: templates},
		Status:     workloads.InstanceSetStatus{InstanceStatus: instances},
	}
}

func verticalScalingTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&opsv1alpha1.OpsRequest{}, &workloads.InstanceSet{}).
		WithObjects(objects...).Build()
}

func verticalScalingTestResourcesBundle(cluster *appsv1.Cluster, ops *opsv1alpha1.OpsRequest) *OpsResource {
	return &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(20)}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
