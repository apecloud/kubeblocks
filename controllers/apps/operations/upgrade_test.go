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
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Upgrade OpsRequest", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
	)
	const mysqlImageForUpdate = "docker.io/apecloud/apecloud-mysql-server:8.0.30"
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
		It("Test upgrade OpsRequest", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, clusterObject := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("create Upgrade Ops")
			newClusterVersionName := "clusterversion-upgrade-" + randomStr
			_ = testapps.NewClusterVersionFactory(newClusterVersionName, clusterDefinitionName).
				AddComponentVersion(statelessComp).AddContainerShort(testapps.DefaultNginxContainerName, "nginx:1.14.2").
				AddComponentVersion(consensusComp).AddContainerShort(testapps.DefaultMySQLContainerName, mysqlImageForUpdate).
				AddComponentVersion(statefulComp).AddContainerShort(testapps.DefaultMySQLContainerName, mysqlImageForUpdate).
				Create(&testCtx).GetObject()
			ops := testapps.NewOpsRequestObj("upgrade-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterObject.Name, appsv1alpha1.UpgradeType)
			ops.Spec.Upgrade = &appsv1alpha1.Upgrade{ClusterVersionRef: newClusterVersionName}
			opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.UpdatingClusterCompPhase,
				consensusComp, statelessComp, statefulComp) // appsv1alpha1.VerticalScalingPhase
			// TODO: add status condition for VerticalScalingPhase

			By("mock upgrade OpsRequest phase is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))
			// do upgrade
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("Test OpsManager.MainEnter function ")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
