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

package stateless

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Stateful Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "stateless-definition-" + randomStr
		clusterVersionName = "stateless-cluster-version-" + randomStr
		clusterName        = "stateless-" + randomStr
		timeout            = 10 * time.Second
		interval           = time.Second
		statelessCompName  = "stateless"
	)
	const defaultMinReadySeconds = 10

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testdbaas.ClearResources(&testCtx, intctrlutil.ClusterSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("Stateless Component test", func() {
		It("Stateless Component test", func() {
			By(" init cluster, deployment")
			cluster := testdbaas.CreateStatelessCluster(testCtx, clusterDefName, clusterVersionName, clusterName)
			deploy := testdbaas.MockStatelessComponentDeploy(testCtx, clusterName, statelessCompName)
			statelessComponent := NewStateless(ctx, k8sClient, cluster)

			By("test pods are not ready")
			patch := client.MergeFrom(deploy.DeepCopy())
			availableReplicas := *deploy.Spec.Replicas - 1
			deploy.Status.AvailableReplicas = availableReplicas
			deploy.Status.ReadyReplicas = availableReplicas
			deploy.Status.Replicas = availableReplicas
			podsReady, _ := statelessComponent.PodsReady(deploy)
			Expect(podsReady == false).Should(BeTrue())
			if testCtx.UsingExistingCluster() {
				Eventually(func() bool {
					phase, _ := statelessComponent.GetPhaseWhenPodsNotReady(statelessCompName)
					return phase == ""
				}, timeout*5, interval).Should(BeTrue())
			} else {
				phase, _ := statelessComponent.GetPhaseWhenPodsNotReady(statelessCompName)
				Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
				Expect(k8sClient.Status().Patch(ctx, deploy, patch)).Should(Succeed())
				By("wait deployment ")
				Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKey{Name: deploy.Name,
					Namespace: testCtx.DefaultNamespace}, func(g Gomega, tmpDeploy *appsv1.Deployment) {
					g.Expect(tmpDeploy.Status.AvailableReplicas == availableReplicas).Should(BeTrue())
				})).Should(Succeed())
				phase, _ = statelessComponent.GetPhaseWhenPodsNotReady(statelessCompName)
				Expect(phase == dbaasv1alpha1.AbnormalPhase).Should(BeTrue())
			}

			By("test pods are ready")
			testk8s.MockDeploymentReady(deploy, NewRSAvailableReason)
			podsReady, _ = statelessComponent.PodsReady(deploy)
			Expect(podsReady == true).Should(BeTrue())

			By("test component is running")
			isRunning, _ := statelessComponent.IsRunning(deploy)
			Expect(isRunning == true).Should(BeTrue())

			By("test handle probe timed out")
			requeue, _ := statelessComponent.HandleProbeTimeoutWhenPodsReady(nil)
			Expect(requeue == false).Should(BeTrue())

			By("test pod is ready")
			podName := "nginx-" + randomStr
			pod := testdbaas.MockStatelessPod(testCtx, clusterName, statelessCompName, podName)
			lastTransTime := metav1.NewTime(time.Now().Add(-1 * (defaultMinReadySeconds + 1) * time.Second))
			testk8s.MockPodAvailable(pod, lastTransTime)
			Expect(statelessComponent.PodIsAvailable(pod, defaultMinReadySeconds)).Should(BeTrue())
		})
	})

})
