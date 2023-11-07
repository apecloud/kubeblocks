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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("Restore OpsRequest", func() {
	var (
		randomStr   = testCtx.GetRandomStr()
		clusterName = "cluster-for-ops-" + randomStr
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

	Context("Test OpsRequest for Restore", func() {
		var (
			opsRes *OpsResource
			reqCtx intctrlutil.RequestCtx
			backup *dpv1alpha1.Backup
		)
		BeforeEach(func() {
			By("init operations resources ")
			backup = testdp.NewFakeBackup(&testCtx, nil)
			reqCtx = intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
		})

		It("", func() {
			By("create Restore OpsRequest")
			opsRes.OpsRequest = createRestoreOpsObj(clusterName, "restore-ops-"+randomStr, backup.Name)

			By("mock restore OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("test restore action and reconcile function")
			testapps.MockConsensusComponentStatefulSet(&testCtx, clusterName, consensusComp)
			testapps.MockStatelessComponentDeploy(&testCtx, clusterName, statelessComp)
			restoreHandler := RestoreOpsHandler{}
			Expect(restoreHandler.Action(reqCtx, k8sClient, opsRes)).Should(Succeed())

			By("test restore reconcile function")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

func createRestoreOpsObj(clusterName, restoreOpsName, backupName string) *appsv1alpha1.OpsRequest {
	ops := &appsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreOpsName,
			Namespace: testCtx.DefaultNamespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.OpsRequestTypeLabelKey: string(appsv1alpha1.RestoreType),
			},
		},
		Spec: appsv1alpha1.OpsRequestSpec{
			ClusterRef: clusterName,
			Type:       appsv1alpha1.RestoreType,
			RestoreSpec: &appsv1alpha1.RestoreSpec{
				BackupName: backupName,
			},
		},
	}
	return testapps.CreateOpsRequest(ctx, testCtx, ops)
}
