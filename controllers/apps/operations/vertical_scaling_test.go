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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
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
		It("Test verticalScaling OpsRequest", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("create VerticalScaling ops")
			ops := testapps.NewOpsRequestObj("verticalscaling-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterName, appsv1alpha1.VerticalScalingType)
			ops.Spec.VerticalScalingList = []appsv1alpha1.VerticalScaling{
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
			opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)
			By("test save last configuration and OpsRequest phase is Running")
			Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(ops))).Should(Equal(appsv1alpha1.RunningPhase))

			By("test vertical scale action function")
			vsHandler := verticalScalingHandler{}
			Expect(vsHandler.Action(opsRes)).Should(Succeed())
			_, _, err := vsHandler.ReconcileAction(opsRes)
			Expect(err == nil).Should(BeTrue())
		})
	})
})
