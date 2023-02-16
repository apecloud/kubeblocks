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

package dbaas

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("test clusterVersion controller", func() {

	var (
		randomStr          = testCtx.GetRandomStr()
		clusterVersionName = "mysql-version-" + randomStr
		clusterDefName     = "mysql-definition-" + randomStr
		clusterNamePrefix  = "mysql-cluster"
	)

	const statefulCompType = "stateful"

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("test clusterVersion controller", func() {
		It("test clusterVersion controller", func() {
			By("create a clusterVersion obj")
			clusterVersionObj := testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(statefulCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()

			By("wait for clusterVersion phase is unavailable when clusterDef is not found")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersionObj), func(g Gomega, tmpCV *dbaasv1alpha1.ClusterVersion) {
				g.Expect(tmpCV.Status.Phase).Should(Equal(dbaasv1alpha1.UnavailablePhase))
			})).Should(Succeed())

			By("create a clusterDefinition obj")
			testdbaas.NewClusterDefFactory(clusterDefName).
				AddComponent(testdbaas.StatefulMySQLComponent, statefulCompType).
				Create(&testCtx).GetObject()

			By("wait for clusterVersion phase is available")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersionObj), func(g Gomega, tmpCV *dbaasv1alpha1.ClusterVersion) {
				g.Expect(tmpCV.Status.Phase).Should(Equal(dbaasv1alpha1.AvailablePhase))
			})).Should(Succeed())

			By("test sync cluster.status.operations")
			cluster := testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
				clusterDefName, clusterVersionName).WithRandomName().Create(&testCtx).GetObject()

			// create a new ClusterVersion
			testdbaas.NewClusterVersionFactory(clusterVersionName+"1", clusterDefName).
				AddComponent(statefulCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()

			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
				operations := tmpCluster.Status.Operations
				g.Expect(operations != nil && operations.Upgradable).Should(BeTrue())
			})).Should(Succeed())

		})
	})

})
