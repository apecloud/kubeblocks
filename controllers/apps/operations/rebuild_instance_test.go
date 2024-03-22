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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test Rebuild-Instance opsRequest", func() {
		createRebuildInstanceOps := func(backupName string, instanceNames ...string) *appsv1alpha1.OpsRequest {
			opsName := "rebuild-instance-" + testCtx.GetRandomStr()
			ops := testapps.NewOpsRequestObj(opsName, testCtx.DefaultNamespace,
				clusterName, appsv1alpha1.RebuildInstanceType)
			ops.Spec.RebuildFrom = []appsv1alpha1.RebuildInstance{
				{
					ComponentOps:  appsv1alpha1.ComponentOps{ComponentName: consensusComp},
					InstanceNames: instanceNames,
					BackupName:    backupName,
				},
			}
			opsRequest := testapps.CreateOpsRequest(ctx, testCtx, ops)
			opsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
			return opsRequest
		}

		prepareOpsRes := func() *OpsResource {
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			podList := initConsensusPods(ctx, k8sClient, opsRes, clusterName)

			By("Test the functions in ops_util.go")
			opsRes.OpsRequest = createRebuildInstanceOps("", podList[0].Name, podList[1].Name)
			return opsRes
		}

		It("test rebuild instance when cluster/component are mismatched", func() {
			By("init operations resources ")
			opsRes := prepareOpsRes()
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("mock cluster phase is Abnormal and component phase is Running")
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1alpha1.AbnormalClusterPhase
			})).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsCreatingPhase

			By("expect for opsRequest phase is Failed if the phase of component is not matched")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsFailedPhase))
			Expect(opsRes.OpsRequest.Status.Conditions[0].Message).Should(ContainSubstring(fmt.Sprintf(`the phase of component "%s" can not be %s`, consensusComp, appsv1alpha1.RunningClusterCompPhase)))

			By("mock component phase to Abnormal")
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsCreatingPhase
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				compStatus := opsRes.Cluster.Status.Components[consensusComp]
				compStatus.Phase = appsv1alpha1.AbnormalClusterCompPhase
				opsRes.Cluster.Status.Components[consensusComp] = compStatus
			})).Should(Succeed())

			By("expect for opsRequest phase is Running")
			_, _ = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsCreatingPhase))
		})

		FIt("test rebuild instance with no backup", func() {
			By("init operations resources ")
			opsRes := prepareOpsRes()
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsRunningPhase
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("expect for the tmp pods and pvcs are created ")
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)

		})

	})
})
