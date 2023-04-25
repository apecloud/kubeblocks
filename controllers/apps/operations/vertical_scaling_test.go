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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("VerticalScaling OpsRequest", func() {

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

		testVerticalScaling := func(verticalScaling []appsv1alpha1.VerticalScaling) {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("create VerticalScaling ops")
			ops := testapps.NewOpsRequestObj("verticalscaling-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterName, appsv1alpha1.VerticalScalingType)

			ops.Spec.VerticalScalingList = verticalScaling
			opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)
			By("test save last configuration and OpsRequest phase is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("test vertical scale action function")
			vsHandler := verticalScalingHandler{}
			Expect(vsHandler.Action(reqCtx, k8sClient, opsRes)).Should(Succeed())
			_, _, err = vsHandler.ReconcileAction(reqCtx, k8sClient, opsRes)
			Expect(err == nil).Should(BeTrue())

			By("test GetRealAffectedComponentMap function")
			Expect(len(vsHandler.GetRealAffectedComponentMap(opsRes.OpsRequest))).Should(Equal(1))
		}

		It("vertical scaling by resource", func() {
			verticalScaling := []appsv1alpha1.VerticalScaling{
				{
					ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("400m"),
							corev1.ResourceMemory: resource.MustParse("300Mi"),
						},
					},
				},
			}
			testVerticalScaling(verticalScaling)
		})

		It("vertical scaling by class", func() {
			verticalScaling := []appsv1alpha1.VerticalScaling{
				{
					ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
					ClassDefRef: &appsv1alpha1.ClassDefRef{
						Class: testapps.Class1c1gName,
					},
				},
			}
			testVerticalScaling(verticalScaling)
		})
	})
})
