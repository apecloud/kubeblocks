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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
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
			cluster := testdbaas.CreateStatelessCluster(ctx, testCtx, clusterDefName, clusterVersionName, clusterName)
			deploy := testdbaas.MockStatelessComponentDeploy(ctx, testCtx, clusterName, statelessCompName)
			statelessComponent := NewStateless(ctx, k8sClient, cluster)

			By("test DeploymentSpecIsUpdated function")
			Expect(DeploymentSpecIsUpdated(deploy)).Should(BeTrue())

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
				Eventually(func(g Gomega) bool {
					tmpDeploy := &appsv1.Deployment{}
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: deploy.Name, Namespace: testCtx.DefaultNamespace}, tmpDeploy)).Should(Succeed())
					return tmpDeploy.Status.AvailableReplicas == availableReplicas
				}, timeout, interval).Should(BeTrue())
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
			pod := testdbaas.MockStatelessPod(ctx, testCtx, clusterName, statelessCompName, podName)
			lastTransTime := metav1.NewTime(time.Now().Add(-1 * (defaultMinReadySeconds + 1) * time.Second))
			testk8s.MockPodAvailable(pod, lastTransTime)
			Expect(statelessComponent.PodIsAvailable(pod, defaultMinReadySeconds)).Should(BeTrue())
		})
	})

})
