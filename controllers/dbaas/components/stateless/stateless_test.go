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

package stateless

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Stateful Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "nginx-definition-" + randomStr
		clusterVersionName = "nginx-cluster-version-" + randomStr
		clusterName        = "nginx-" + randomStr
		timeout            = 10 * time.Second
		interval           = time.Second
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey},
			client.GracePeriodSeconds(0))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	Context("Stateless Component test", func() {
		It("Stateless Component test", func() {
			By(" init cluster, deployment")
			cluster := testdbaas.CreateStatelessCluster(testCtx, clusterDefName, clusterVersionName, clusterName)
			deploy := testdbaas.MockStatelessComponentDeploy(testCtx, clusterName)
			statelessComponent := NewStateless(ctx, k8sClient, cluster)

			By("test pods are not ready")
			deploy.Status.AvailableReplicas = *deploy.Spec.Replicas - 1
			podsReady, _ := statelessComponent.PodsReady(deploy)
			Expect(podsReady == false).Should(BeTrue())
			componentName := "nginx"
			if testCtx.UsingExistingCluster() {
				Eventually(func() bool {
					phase, _ := statelessComponent.CalculatePhaseWhenPodsNotReady(componentName)
					return phase == ""
				}, timeout*5, interval).Should(BeTrue())
			} else {
				phase, _ := statelessComponent.CalculatePhaseWhenPodsNotReady(componentName)
				Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
			}

			By("test pods are ready")
			deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
			deploy.Status.ObservedGeneration = deploy.Generation
			deploy.Status.Replicas = *deploy.Spec.Replicas
			podsReady, _ = statelessComponent.PodsReady(deploy)
			Expect(podsReady == true).Should(BeTrue())

			By("test component is running")
			isRunning, _ := statelessComponent.IsRunning(deploy)
			Expect(isRunning == true).Should(BeTrue())

			By("test handle probe timed out")
			requeue, _ := statelessComponent.HandleProbeTimeoutWhenPodsReady()
			Expect(requeue == false).Should(BeTrue())

		})
	})

})
