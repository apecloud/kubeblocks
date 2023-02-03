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

package components

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/stateless"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Deployment Controller", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		timeout            = time.Second * 10
		interval           = time.Second
		clusterDefName     = "stateless-definition1-" + randomStr
		clusterVersionName = "stateless-cluster-version1-" + randomStr
		clusterName        = "stateless1-" + randomStr
		namespace          = "default"
		statelessCompName  = "stateless"
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testdbaas.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("test controller", func() {
		It("", func() {
			if testCtx.UsingExistingCluster() {
				timeout = 3 * timeout
			}
			cluster := testdbaas.CreateStatelessCluster(ctx, testCtx, clusterDefName, clusterVersionName, clusterName)
			By("patch cluster to Running")
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				statelessCompName: {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), cluster, patch)).Should(Succeed())
			Eventually(testdbaas.GetClusterComponentPhase(testCtx, clusterName, statelessCompName),
				timeout, interval).Should(Equal(dbaasv1alpha1.RunningPhase))

			By(" check component is Failed/Abnormal")
			deploy := testdbaas.MockStatelessComponentDeploy(ctx, testCtx, clusterName, statelessCompName)
			Eventually(testdbaas.GetClusterComponentPhase(testCtx, clusterName, statelessCompName),
				timeout, interval).Should(Equal(dbaasv1alpha1.FailedPhase))

			By("mock deployment is ready")
			newDeployment := &appsv1.Deployment{}
			newDeploymentKey := client.ObjectKey{Name: deploy.Name, Namespace: namespace}
			Expect(k8sClient.Get(context.Background(), newDeploymentKey, newDeployment)).Should(Succeed())
			deployPatch := client.MergeFrom(newDeployment.DeepCopy())
			testk8s.MockDeploymentReady(newDeployment, stateless.NewRSAvailableReason)
			Expect(k8sClient.Status().Patch(context.Background(), newDeployment, deployPatch)).Should(Succeed())

			By("test deployment status is Running")
			Eventually(func() bool {
				deploy := &appsv1.Deployment{}
				if err := k8sClient.Get(context.Background(), newDeploymentKey, deploy); err != nil {
					return false
				}
				return deploy.Status.AvailableReplicas == newDeployment.Status.AvailableReplicas &&
					deploy.Status.ReadyReplicas == newDeployment.Status.ReadyReplicas &&
					deploy.Status.Replicas == newDeployment.Status.Replicas
			}, timeout, interval).Should(BeTrue())

			By("waiting the component is Running")
			Eventually(testdbaas.GetClusterComponentPhase(testCtx, clusterName, statelessCompName),
				timeout, interval).Should(Equal(dbaasv1alpha1.RunningPhase))
		})
	})
})
