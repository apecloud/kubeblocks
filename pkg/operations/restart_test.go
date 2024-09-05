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

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("Restart OpsRequest", func() {

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
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingClusterCompPhase, defaultCompName)

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

		ExpectCompRestarted := func(opsRequest *appsv1alpha1.OpsRequest, compName string, expectRestarted bool) {
			instanceSetName := constant.GenerateWorkloadNamePattern(clusterName, compName)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: instanceSetName, Namespace: testCtx.DefaultNamespace},
				func(g Gomega, pobj *workloads.InstanceSet) {
					startTimestamp := opsRes.OpsRequest.Status.StartTimestamp
					workloadRestartTimeStamp := pobj.Spec.Template.Annotations[constant.RestartAnnotationKey]
					res, _ := time.Parse(time.RFC3339, workloadRestartTimeStamp)
					g.Expect(!startTimestamp.After(res)).Should(Equal(expectRestarted))
				})).Should(Succeed())
		}

		It("Test restart OpsRequest with existing update orders", func() {
			By("init operations resources")
			opsRes, _, cluster = initOperationsResourcesWithTopology(clusterDefName, compDefName, clusterName)

			By("create Restart opsRequest")
			opsRes.OpsRequest = createRestartOpsObj(clusterName, "restart-ops-"+randomStr,
				defaultCompName, secondaryCompName, thirdCompName)
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.UpdatingClusterCompPhase,
				defaultCompName, secondaryCompName, thirdCompName)

			By("mock restart OpsRequest to Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("test restart Action")
			rHandler := restartOpsHandler{}
			_ = rHandler.Action(reqCtx, k8sClient, opsRes)
			ExpectCompRestarted(opsRes.OpsRequest, defaultCompName, false)
			ExpectCompRestarted(opsRes.OpsRequest, secondaryCompName, false)
			ExpectCompRestarted(opsRes.OpsRequest, thirdCompName, false)

			By("test reconcile Action")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			ExpectCompRestarted(opsRes.OpsRequest, defaultCompName, true)
			ExpectCompRestarted(opsRes.OpsRequest, secondaryCompName, false)
			ExpectCompRestarted(opsRes.OpsRequest, thirdCompName, false)

			By("mock restart secondary component completed")
			setCompProgress := func(compName string, status appsv1alpha1.ProgressStatus) {
				workloadName := constant.GenerateWorkloadNamePattern(clusterName, compName)
				opsRes.OpsRequest.Status.Components[compName] = appsv1alpha1.OpsRequestComponentStatus{
					ProgressDetails: []appsv1alpha1.ProgressStatusDetail{
						{ObjectKey: getProgressObjectKey(constant.PodKind, workloadName+"-0"), Status: status},
						{ObjectKey: getProgressObjectKey(constant.PodKind, workloadName+"-1"), Status: status},
						{ObjectKey: getProgressObjectKey(constant.PodKind, workloadName+"-2"), Status: status},
					},
				}
			}
			setCompProgress(defaultCompName, appsv1alpha1.SucceedProgressStatus)
			setCompProgress(secondaryCompName, appsv1alpha1.PendingProgressStatus)

			By("test reconcile Action and expect to restart third component")
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err == nil).Should(BeTrue())
			ExpectCompRestarted(opsRes.OpsRequest, defaultCompName, true)
			ExpectCompRestarted(opsRes.OpsRequest, secondaryCompName, true)
			ExpectCompRestarted(opsRes.OpsRequest, thirdCompName, false)
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
	ops := testapps.NewOpsRequestObj(restartOpsName, testCtx.DefaultNamespace,
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
