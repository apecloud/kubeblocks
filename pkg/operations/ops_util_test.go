/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("OpsUtil functions", func() {

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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ConfigMapSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test ops_util functions", func() {
		It("Test ops_util functions", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)

			By("Test the functions in ops_util.go")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{
						ReplicaChanges: pointer.Int32(2),
					},
				},
			})
			Expect(patchValidateErrorCondition(ctx, k8sClient, opsRes, "validate error")).Should(Succeed())
			Expect(PatchOpsHandlerNotSupported(ctx, k8sClient, opsRes)).Should(Succeed())
			Expect(isOpsRequestFailedPhase(opsv1alpha1.OpsFailedPhase)).Should(BeTrue())
			Expect(PatchClusterNotFound(ctx, k8sClient, opsRes)).Should(Succeed())
		})

		It("Test opsRequest failed cases", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			pods := testapps.MockInstanceSetPods(&testCtx, nil, opsRes.Cluster, defaultCompName)
			time.Sleep(time.Second)
			By("Test the functions in ops_util.go")
			ops := testops.NewOpsRequestObj("restart-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.RestartType)
			ops.Spec.RestartList = []opsv1alpha1.ComponentOps{{ComponentName: defaultCompName}}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
			opsRes.OpsRequest.Status.StartTimestamp = metav1.Now()

			By("mock component failed")
			clusterComp := opsRes.Cluster.Status.Components[defaultCompName]
			clusterComp.Phase = appsv1.FailedClusterCompPhase
			opsRes.Cluster.Status.SetComponentStatus(defaultCompName, clusterComp)

			By("expect for opsRequest is running")
			handleRestartProgress := func(reqCtx intctrlutil.RequestCtx,
				cli client.Client,
				opsRes *OpsResource,
				pgRes *progressResource,
				compStatus *opsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, completedCount int32, err error) {
				return handleComponentStatusProgress(reqCtx, cli, opsRes, pgRes, compStatus,
					func(ops *opsv1alpha1.OpsRequest, pod *corev1.Pod, pgRes *progressResource) bool {
						return !pod.CreationTimestamp.Before(&ops.Status.StartTimestamp)
					})
			}

			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.RestartList)

			opsPhase, _, err := compOpsHelper.reconcileActionWithComponentOps(reqCtx, k8sClient, opsRes,
				"test", handleRestartProgress)
			Expect(err).Should(BeNil())
			Expect(opsPhase).Should(Equal(opsv1alpha1.OpsRunningPhase))

			By("mock one pod recreates failed, expect for opsRequest is Failed")
			testk8s.MockPodIsTerminating(ctx, testCtx, pods[2])
			testk8s.RemovePodFinalizer(ctx, testCtx, pods[2])
			// recreate it
			pod := testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName, pods[2].Name, "follower", "Readonly")
			// mock pod is failed
			testk8s.MockPodIsFailed(ctx, testCtx, pod)
			opsPhase, _, err = compOpsHelper.reconcileActionWithComponentOps(reqCtx, k8sClient, opsRes, "test", handleRestartProgress)
			Expect(err).Should(BeNil())
			Expect(opsPhase).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("Test opsRequest with disable ha", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)

			By("Test the functions in ops_util.go")
			ops := testops.NewOpsRequestObj("restart-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.RestartType)
			ops.Spec.RestartList = []opsv1alpha1.ComponentOps{{ComponentName: defaultCompName}}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.OpsRequest, func() {
				opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsCreatingPhase
				opsRes.OpsRequest.Status.StartTimestamp = metav1.Time{Time: time.Now()}
			})).Should(Succeed())
			By("create ha configmap and do horizontalScaling with disable ha")
			haConfigName := "ha-config"
			haConfig := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      haConfigName,
					Namespace: testCtx.DefaultNamespace,
					Annotations: map[string]string{
						"enable": "true",
					},
				},
			}
			Expect(k8sClient.Create(ctx, haConfig)).Should(Succeed())
			opsRes.OpsRequest.Annotations = map[string]string{
				constant.DisableHAAnnotationKey: haConfigName,
			}

			By("mock instance set")
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)

			By("expect to disable ha")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(haConfig), func(g Gomega, cm *corev1.ConfigMap) {
				cm.Annotations["enable"] = "false"
			})).Should(Succeed())

			By("mock restart ops to succeed and expect to enable ha")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase
			_ = testapps.MockInstanceSetPods(&testCtx, its, opsRes.Cluster, defaultCompName)
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsSucceedPhase))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(haConfig), func(g Gomega, cm *corev1.ConfigMap) {
				cm.Annotations["enable"] = "true"
			})).Should(Succeed())
		})

		It("Test opsRequest Queue functions", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)

			runHscaleOps := func(expectPhase opsv1alpha1.OpsPhase) *opsv1alpha1.OpsRequest {
				ops := createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					ScaleIn: &opsv1alpha1.ScaleIn{
						ReplicaChanger: opsv1alpha1.ReplicaChanger{
							ReplicaChanges: pointer.Int32(1),
						},
					},
				})
				opsRes.OpsRequest = ops
				_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(expectPhase))
				return ops
			}

			By("run first h-scale ops, expect phase to Creating")
			ops1 := runHscaleOps(opsv1alpha1.OpsCreatingPhase)

			By("run next h-scale ops, expect phase to Pending")
			ops2 := runHscaleOps(opsv1alpha1.OpsPendingPhase)

			By("check opsRequest annotation in cluster")
			cluster := &appsv1.Cluster{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), cluster)).Should(Succeed())
			opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
			Expect(len(opsSlice)).Should(Equal(2))
			Expect(opsSlice[0].InQueue).Should(BeFalse())
			Expect(opsSlice[1].InQueue).Should(BeTrue())

			By("test enqueueOpsRequestToClusterAnnotation function with Reentry")
			opsBehaviour := opsManager.OpsMap[ops2.Spec.Type]
			_, _ = enqueueOpsRequestToClusterAnnotation(ctx, k8sClient, opsRes, opsBehaviour)
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), cluster)).Should(Succeed())
			opsSlice, _ = opsutil.GetOpsRequestSliceFromCluster(cluster)
			Expect(len(opsSlice)).Should(Equal(2))

			By("test DequeueOpsRequestInClusterAnnotation function when first opsRequest is Failed")
			// mock ops1 is Failed
			ops1.Status.Phase = opsv1alpha1.OpsFailedPhase
			opsRes.OpsRequest = ops1
			Expect(DequeueOpsRequestInClusterAnnotation(ctx, k8sClient, opsRes)).Should(Succeed())
			testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops2), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				// expect ops2 is Cancelled
				g.Expect(ops.Status.Phase).Should(Equal(opsv1alpha1.OpsCancelledPhase))
			})

			testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, cluster *appsv1.Cluster) {
				opsSlice, _ = opsutil.GetOpsRequestSliceFromCluster(cluster)
				// expect cluster's opsRequest queue is empty
				g.Expect(opsSlice).Should(BeEmpty())
			})
		})

		It("Test opsRequest dependency", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)

			By("create a first horizontal opsRequest")
			ops1 := createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
				},
			})
			opsRes.OpsRequest = ops1
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("create another horizontal opsRequest with force flag and dependent the first opsRequest")
			ops2 := createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ScaleOut: &opsv1alpha1.ScaleOut{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)},
				},
			})
			ops2.Annotations = map[string]string{constant.OpsDependentOnSuccessfulOpsAnnoKey: ops1.Name}
			ops2.Spec.Force = true
			opsRes.OpsRequest = ops2
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsPendingPhase))
			// expect the dependent ops has been annotated
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops1), func(g Gomega, ops *opsv1alpha1.OpsRequest) {
				g.Expect(ops.Annotations[constant.RelatedOpsAnnotationKey]).Should(Equal(ops2.Name))
			})).Should(Succeed())

			By("expect for the ops is Creating when dependent ops is succeed")
			Expect(testapps.ChangeObjStatus(&testCtx, ops1, func() {
				ops1.Status.Phase = opsv1alpha1.OpsSucceedPhase
			})).Should(Succeed())

			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("expect for the ops is Cancelled when dependent ops is Failed")
			Expect(testapps.ChangeObjStatus(&testCtx, ops1, func() {
				ops1.Status.Phase = opsv1alpha1.OpsFailedPhase
			})).Should(Succeed())

			ops2.Annotations = map[string]string{constant.OpsDependentOnSuccessfulOpsAnnoKey: ops1.Name}
			ops2.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops2))).Should(Equal(opsv1alpha1.OpsCancelledPhase))
		})

		It("Test EnqueueOnForce=true", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)

			By("mock cluster phase to Updating")
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1.UpdatingClusterPhase
			})).Should(Succeed())

			By("expect the ops phase is failed")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{
						ReplicaChanges: pointer.Int32(1),
					},
				},
			})
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))

			By("Test EnqueueOnForce=true")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{
						ReplicaChanges: pointer.Int32(1),
					},
				},
			})
			opsRes.OpsRequest.Spec.Force = true
			opsRes.OpsRequest.Spec.EnqueueOnForce = true
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			By("expect the ops phase is Creating")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
		})
	})
})
