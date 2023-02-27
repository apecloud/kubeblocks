/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operations

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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
		// must wait until resources deleted and no longer exist before the testcases start,
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
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {
		It("Test upgrade OpsRequest", func() {
			By("init operations resources ")
			opsRes, _, clusterObject := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("create Upgrade Ops")
			newClusterVersionName := "clusterversion-upgrade-" + randomStr
			_ = testapps.NewClusterVersionFactory(newClusterVersionName, clusterDefinitionName).
				AddComponent(statelessComp).AddContainerShort(testapps.DefaultNginxContainerName, "nginx:1.14.2").
				AddComponent(consensusComp).AddContainerShort(testapps.DefaultMySQLContainerName, mysqlImageForUpdate).
				AddComponent(statefulComp).AddContainerShort(testapps.DefaultMySQLContainerName, mysqlImageForUpdate).
				Create(&testCtx).GetObject()
			ops := testapps.NewOpsRequestObj("upgrade-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterObject.Name, appsv1alpha1.UpgradeType)
			ops.Spec.Upgrade = &appsv1alpha1.Upgrade{ClusterVersionRef: newClusterVersionName}
			opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)

			By("mock upgrade OpsRequest phase is Running")
			Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.RunningPhase))

			By("Test OpsManager.MainEnter function ")
			_, err := GetOpsManager().Reconcile(opsRes)
			Expect(err).Should(Succeed())
		})
	})
})
