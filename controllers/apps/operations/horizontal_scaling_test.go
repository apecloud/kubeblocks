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
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningPhase
		})).ShouldNot(HaveOccurred())
	}

	Context("Test OpsRequest", func() {
		It("Test HorizontalScaling OpsRequest", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("Test HorizontalScaling with scale down replicas")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, 1)
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.VerticalScalingPhase, consensusComp)
			initClusterForOps(opsRes)

			By("mock HorizontalScaling OpsRequest phase is Creating and do action")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("Test OpsManager.Reconcile function when horizontal scaling OpsRequest is Running")
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningPhase
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("test GetOpsRequestAnnotation function")
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func() {
				opsAnnotationString := fmt.Sprintf(`[{"name":"%s","clusterPhase":"HorizontalScaling"},{"name":"test-not-exists-ops","clusterPhase":"VolumeExpanding"}]`,
					opsRes.OpsRequest.Name)
				opsRes.Cluster.Annotations = map[string]string{
					constant.OpsRequestAnnotationKey: opsAnnotationString,
				}
			})).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err.Error()).Should(ContainSubstring("Existing OpsRequest:"))

			// reset cluster annotation
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Annotations = map[string]string{}
			})).ShouldNot(HaveOccurred())

			By("Test HorizontalScaling with scale up replicax")
			initClusterForOps(opsRes)
			expectClusterComponentReplicas := int32(2)
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Spec.ComponentSpecs[1].Replicas = expectClusterComponentReplicas
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
