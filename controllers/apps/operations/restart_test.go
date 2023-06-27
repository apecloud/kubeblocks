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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Restart OpsRequest", func() {

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

	Context("Test OpsRequest", func() {
		var (
			opsRes  *OpsResource
			cluster *appsv1alpha1.Cluster
			reqCtx  intctrlutil.RequestCtx
		)
		BeforeEach(func() {
			By("init operations resources ")
			opsRes, _, cluster = initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			reqCtx = intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
		})

		It("Test restart OpsRequest", func() {
			By("create Restart opsRequest")
			opsRes.OpsRequest = createRestartOpsObj(clusterName, "restart-ops-"+randomStr)
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.SpecReconcilingClusterCompPhase, consensusComp, statelessComp)

			By("mock restart OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("test restart action and reconcile function")
			testapps.MockConsensusComponentStatefulSet(&testCtx, clusterName, consensusComp)
			testapps.MockStatelessComponentDeploy(&testCtx, clusterName, statelessComp)
			rHandler := restartOpsHandler{}
			_ = rHandler.Action(reqCtx, k8sClient, opsRes)

			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err == nil).Should(BeTrue())

			By("test GetRealAffectedComponentMap function")
			h := restartOpsHandler{}
			Expect(len(h.GetRealAffectedComponentMap(opsRes.OpsRequest))).Should(Equal(2))
		})

		It("expect failed when cluster is stopped", func() {
			By("mock cluster is stopped")
			Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
				cluster.Status.Phase = appsv1alpha1.StoppedClusterPhase
			})).Should(Succeed())
			By("create Restart opsRequest")
			opsRes.OpsRequest = createRestartOpsObj(clusterName, "restart-ops-"+randomStr)
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("OpsRequest.spec.type=Restart is forbidden when Cluster.status.phase=Stopped"))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest),
				func(g Gomega, fetched *appsv1alpha1.OpsRequest) {
					g.Expect(fetched.Status.Phase).To(Equal(appsv1alpha1.OpsFailedPhase))
				})).Should(Succeed())
		})
	})
})

func createRestartOpsObj(clusterName, restartOpsName string) *appsv1alpha1.OpsRequest {
	ops := testapps.NewOpsRequestObj(restartOpsName, testCtx.DefaultNamespace,
		clusterName, appsv1alpha1.RestartType)
	ops.Spec.RestartList = []appsv1alpha1.ComponentOps{
		{ComponentName: consensusComp},
		{ComponentName: statelessComp},
	}
	return testapps.CreateOpsRequest(ctx, testCtx, ops)
}
