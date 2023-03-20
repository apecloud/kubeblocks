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

package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("test clusterVersion controller", func() {

	var (
		randomStr          = testCtx.GetRandomStr()
		clusterVersionName = "mysql-version-" + randomStr
		clusterDefName     = "mysql-definition-" + randomStr
	)

	const statefulCompType = "stateful"

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("test clusterVersion controller", func() {
		It("test clusterVersion controller", func() {
			By("create a clusterVersion obj")
			clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(statefulCompType).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()

			By("wait for clusterVersion phase is unavailable when clusterDef is not found")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersionObj),
				func(g Gomega, tmpCV *appsv1alpha1.ClusterVersion) {
					g.Expect(tmpCV.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())

			By("create a clusterDefinition obj")
			testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, statefulCompType).
				Create(&testCtx).GetObject()

			By("wait for clusterVersion phase is available")
			Eventually(testapps.CheckObj(&testCtx,
				client.ObjectKeyFromObject(clusterVersionObj),
				func(g Gomega, tmpCV *appsv1alpha1.ClusterVersion) {
					g.Expect(tmpCV.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})

})
