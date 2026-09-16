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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

		It("rejects vertical scaling payload updates while allowing cancellation", func() {
			ops := testops.NewOpsRequestObj("vertical-immutable-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.VerticalScalingType)
			ops.Spec.VerticalScalingList = []opsv1alpha1.VerticalScaling{{
				ComponentOps:         opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ResourceRequirements: newResources,
			}}
			Expect(k8sClient.Create(ctx, ops)).Should(Succeed())

			mutated := ops.DeepCopy()
			mutated.Spec.VerticalScalingList[0].Requests[corev1.ResourceCPU] = resource.MustParse("800m")
			err := k8sClient.Update(ctx, mutated)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("forbidden to update spec.verticalScaling"))

			current := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), current)).Should(Succeed())
			current.Spec.VerticalScalingList = nil
			err = k8sClient.Update(ctx, current)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("forbidden to add or remove spec.verticalScaling"))

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ops), current)).Should(Succeed())
			current.Spec.Cancel = true
			Expect(k8sClient.Update(ctx, current)).Should(Succeed())
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
