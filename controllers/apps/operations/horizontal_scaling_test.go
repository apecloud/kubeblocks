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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("HorizontalScaling OpsRequest", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
	)

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
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initClusterForOps := func(opsRes *OpsResource) {
		Expect(opsutil.PatchClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
		})).ShouldNot(HaveOccurred())
	}

	Context("Test OpsRequest", func() {
		It("Test HorizontalScaling OpsRequest", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("Test HorizontalScaling with scale down replicas")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, 1)
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.SpecReconcilingClusterCompPhase, consensusComp) // appsv1alpha1.VerticalScalingPhase
			initClusterForOps(opsRes)

			By("mock HorizontalScaling OpsRequest phase is Creating and do action")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("Test OpsManager.Reconcile function when horizontal scaling OpsRequest is Running")
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("test GetOpsRequestAnnotation function")
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(lcluster *appsv1alpha1.Cluster) {
				opsAnnotationString := fmt.Sprintf(`[{"name":"%s","clusterPhase":"Updating"},{"name":"test-not-exists-ops","clusterPhase":"Updating"}]`,
					opsRes.OpsRequest.Name)
				lcluster.Annotations = map[string]string{
					constant.OpsRequestAnnotationKey: opsAnnotationString,
				}
			})).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err.Error()).Should(ContainSubstring("existing OpsRequest:"))

			// reset cluster annotation
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(lcluster *appsv1alpha1.Cluster) {
				lcluster.Annotations = map[string]string{}
			})).ShouldNot(HaveOccurred())

			By("Test HorizontalScaling with scale up replicax")
			initClusterForOps(opsRes)
			expectClusterComponentReplicas := int32(2)
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(lcluster *appsv1alpha1.Cluster) {
				lcluster.Spec.ComponentSpecs[1].Replicas = expectClusterComponentReplicas
			})).ShouldNot(HaveOccurred())

			// mock pod created according to horizontalScaling replicas
			for _, v := range []int{1, 2} {
				podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, v)
				testapps.MockConsensusComponentStsPod(testCtx, nil, clusterName, consensusComp, podName, "follower", "ReadOnly")
			}

			opsRes.OpsRequest = createHorizontalScaling(clusterName, 3)
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			// do h-scale action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("test GetRealAffectedComponentMap function")
			h := horizontalScalingOpsHandler{}
			Expect(len(h.GetRealAffectedComponentMap(opsRes.OpsRequest))).Should(Equal(1))
		})

	})
})

func createHorizontalScaling(clusterName string, replicas int) *appsv1alpha1.OpsRequest {
	horizontalOpsName := "horizontalscaling-ops-" + testCtx.GetRandomStr()
	ops := testapps.NewOpsRequestObj(horizontalOpsName, testCtx.DefaultNamespace,
		clusterName, appsv1alpha1.HorizontalScalingType)
	ops.Spec.HorizontalScalingList = []appsv1alpha1.HorizontalScaling{
		{
			ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
			Replicas:     int32(replicas),
		},
	}
	return testapps.CreateOpsRequest(ctx, testCtx, ops)
}
