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
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
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
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initClusterForOps := func(opsRes *OpsResource) {
		Expect(opsutil.PatchClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		opsRes.Cluster.Status.Phase = appsv1alpha1.RunningPhase
	}

	Context("Test OpsRequest", func() {
		It("Test HorizontalScaling OpsRequest", func() {
			By("init operations resources ")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("Test HorizontalScaling with scale down replicas")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, 1)
			initClusterForOps(opsRes)

			By("mock HorizontalScaling OpsRequest phase is running and do action")
			Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.RunningPhase))

			By("Test OpsManager.Reconcile function when horizontal scaling OpsRequest is Running")
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningPhase
			_, err := GetOpsManager().Reconcile(opsRes)
			Expect(err == nil).Should(BeTrue())

			By("test GetOpsRequestAnnotation function")
			patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
			opsAnnotationString := fmt.Sprintf(`[{"name":"%s","clusterPhase":"HorizontalScaling"},{"name":"test-not-exists-ops","clusterPhase":"VolumeExpanding"}]`,
				opsRes.OpsRequest.Name)
			opsRes.Cluster.Annotations = map[string]string{
				constant.OpsRequestAnnotationKey: opsAnnotationString,
			}
			Expect(k8sClient.Patch(ctx, opsRes.Cluster, patch)).Should(Succeed())
			Expect(GetOpsManager().Do(opsRes)).Should(Succeed())

			By("Test OpsManager.Reconcile when opsRequest is succeed")
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.SucceedPhase
			opsRes.Cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
				consensusComp: {
					Phase: appsv1alpha1.RunningPhase,
				},
			}
			_, err = GetOpsManager().Reconcile(opsRes)
			Expect(err == nil).Should(BeTrue())

			By("Test HorizontalScaling with scale up replica")
			initClusterForOps(opsRes)
			expectClusterComponentReplicas := int32(2)
			opsRes.Cluster.Spec.ComponentSpecs[1].Replicas = expectClusterComponentReplicas
			opsRes.OpsRequest = createHorizontalScaling(clusterName, 3)
			Expect(GetOpsManager().Do(opsRes)).Should(Succeed())

			_, err = GetOpsManager().Reconcile(opsRes)
			Expect(err == nil).Should(BeTrue())
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
