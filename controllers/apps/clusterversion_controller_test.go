/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

	const statefulCompDefName = "stateful"

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
				AddComponent(statefulCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()

			By("wait for clusterVersion phase is unavailable when clusterDef is not found")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersionObj),
				func(g Gomega, tmpCV *appsv1alpha1.ClusterVersion) {
					g.Expect(tmpCV.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())

			By("create a clusterDefinition obj")
			testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
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
