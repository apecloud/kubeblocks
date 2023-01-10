/*
Copyright ApeCloud Inc.

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

package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("OpsRequest Controller", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-" + randomStr
		clusterVersionName    = "clusterversion-" + randomStr
		clusterName           = "cluster-" + randomStr
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	Context("Test OpsRequest", func() {
		It("Should Test all OpsRequest", func() {
			cluster := testdbaas.CreateConsensusMysqlCluster(testCtx, clusterDefinitionName, clusterVersionName, clusterName)
			By("init restart OpsRequest")
			testOpsName := "restart-" + randomStr
			ops := testdbaas.GenerateOpsRequestObj(testOpsName, clusterName, dbaasv1alpha1.RestartType)
			ops.Spec.RestartList = []dbaasv1alpha1.ComponentOps{
				{ComponentName: testdbaas.ConsensusComponentName},
			}
			ops = testdbaas.CreateOpsRequest(testCtx, ops)

			By("test PatchOpsRequestReconcileAnnotation function")
			Expect(PatchOpsRequestReconcileAnnotation(ctx, k8sClient, cluster, testOpsName)).Should(Succeed())
			opsRecordSlice := []dbaasv1alpha1.OpsRecorder{
				{
					Name:           testOpsName,
					ToClusterPhase: dbaasv1alpha1.UpdatingPhase,
				},
				{
					Name:           "not-exists-ops",
					ToClusterPhase: dbaasv1alpha1.UpdatingPhase,
				},
			}
			Expect(PatchClusterOpsAnnotations(ctx, k8sClient, cluster, opsRecordSlice)).Should(Succeed())

			By("test GetOpsRequestSliceFromCluster function")
			opsRecordSlice, _ = GetOpsRequestSliceFromCluster(cluster)
			Expect(len(opsRecordSlice) == 2 && opsRecordSlice[0].Name == testOpsName).Should(BeTrue())

			By("test MarkRunningOpsRequestAnnotation function")
			Expect(MarkRunningOpsRequestAnnotation(ctx, k8sClient, cluster)).Should(Succeed())
			opsRecordSlice, _ = GetOpsRequestSliceFromCluster(cluster)
			Expect(len(opsRecordSlice) == 1).Should(BeTrue())

			By("test no OpsRequest annotation in cluster")
			Expect(PatchClusterOpsAnnotations(ctx, k8sClient, cluster, nil)).Should(Succeed())
			opsRecordSlice, _ = GetOpsRequestSliceFromCluster(cluster)
			Expect(len(opsRecordSlice) == 0).Should(BeTrue())
		})
	})
})
