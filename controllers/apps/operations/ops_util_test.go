/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("OpsUtil functions", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test ops_util functions", func() {
		It("Test ops_util functions", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("Test the functions in ops_util.go")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, 1)
			Expect(patchValidateErrorCondition(ctx, k8sClient, opsRes, "validate error")).Should(Succeed())
			Expect(PatchOpsHandlerNotSupported(ctx, k8sClient, opsRes)).Should(Succeed())
			Expect(isOpsRequestFailedPhase(appsv1alpha1.OpsFailedPhase)).Should(BeTrue())
			Expect(PatchClusterNotFound(ctx, k8sClient, opsRes)).Should(Succeed())
		})

		It("Test opsRequest failed cases", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("Test the functions in ops_util.go")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, 1)
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsRunningPhase

			By("mock component failed")
			clusterComp := opsRes.Cluster.Status.Components[consensusComp]
			clusterComp.Phase = appsv1alpha1.FailedClusterCompPhase
			opsRes.Cluster.Status.SetComponentStatus(consensusComp, clusterComp)

			By("expect for opsRequest is running")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsPhase, _, err := reconcileActionWithComponentOps(reqCtx, k8sClient, opsRes, "test", handleComponentStatusProgress)
			Expect(err).Should(BeNil())
			Expect(opsPhase).Should(Equal(appsv1alpha1.OpsRunningPhase))

			By("mock component failed time reaches the threshold, expect for opsRequest is Failed")
			compStatus := opsRes.OpsRequest.Status.Components[consensusComp]
			compStatus.LastFailedTime = metav1.Time{Time: compStatus.LastFailedTime.Add(-1 * componentFailedTimeout).Add(-1 * time.Second)}
			opsRes.OpsRequest.Status.Components[consensusComp] = compStatus
			opsPhase, _, err = reconcileActionWithComponentOps(reqCtx, k8sClient, opsRes, "test", handleComponentStatusProgress)
			Expect(err).Should(BeNil())
			Expect(opsPhase).Should(Equal(appsv1alpha1.OpsFailedPhase))

		})

		It("Test opsRequest Queue functions", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			runHscaleOps := func(expectPhase appsv1alpha1.OpsPhase) *appsv1alpha1.OpsRequest {
				ops := createHorizontalScaling(clusterName, 1)
				opsRes.OpsRequest = ops
				_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(expectPhase))
				return ops
			}

			By("run first h-scale ops, expect phase to Creating")
			ops1 := runHscaleOps(appsv1alpha1.OpsCreatingPhase)

			By("run next h-scale ops, expect phase to Pending")
			ops2 := runHscaleOps(appsv1alpha1.OpsPendingPhase)

			By("check opsRequest annotation in cluster")
			cluster := &appsv1alpha1.Cluster{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), cluster)).Should(Succeed())
			opsSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
			Expect(len(opsSlice)).Should(Equal(2))
			Expect(opsSlice[0].InQueue).Should(BeFalse())
			Expect(opsSlice[1].InQueue).Should(BeTrue())

			By("test enqueueOpsRequestToClusterAnnotation function with Reentry")
			opsBehaviour := opsManager.OpsMap[ops2.Spec.Type]
			opsSlice, _ = enqueueOpsRequestToClusterAnnotation(ctx, k8sClient, opsRes, opsBehaviour)
			Expect(len(opsSlice)).Should(Equal(2))

			By("test DequeueOpsRequestInClusterAnnotation function when first opsRequest is Failed")
			// mock ops1 is Failed
			ops1.Status.Phase = appsv1alpha1.OpsFailedPhase
			opsRes.OpsRequest = ops1
			Expect(DequeueOpsRequestInClusterAnnotation(ctx, k8sClient, opsRes)).Should(Succeed())
			testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(ops2), func(g Gomega, ops *appsv1alpha1.OpsRequest) {
				// expect ops2 is Cancelled
				g.Expect(ops.Status.Phase).Should(Equal(appsv1alpha1.OpsCancelledPhase))
			})

			testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, cluster *appsv1alpha1.Cluster) {
				opsSlice, _ = opsutil.GetOpsRequestSliceFromCluster(cluster)
				// expect cluster's opsRequest queue is empty
				g.Expect(opsSlice).Should(BeEmpty())
			})
		})
	})
})
