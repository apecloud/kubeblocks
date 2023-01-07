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

package stateful

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Stateful Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "mysql1-clusterdef-" + randomStr
		clusterVersionName = "mysql1-clusterversion-" + randomStr
		clusterName        = "mysql1-" + randomStr
		timeout            = 10 * time.Second
		interval           = time.Second
	)
	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
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

	Context("Stateful Component test", func() {
		It("Stateful Component test", func() {
			By(" init cluster, statefulSet, pods")
			_, _, cluster := testdbaas.InitConsensusMysql(testCtx, clusterDefName, clusterVersionName, clusterName)

			_ = testdbaas.MockConsensusComponentStatefulSet(testCtx, clusterName)
			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				_ = k8sClient.List(ctx, stsList, client.InNamespace(testCtx.DefaultNamespace), client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey:  clusterName,
					intctrlutil.AppComponentLabelKey: testdbaas.ConsensusComponentName,
				}, client.Limit(1))
				return len(stsList.Items) > 0
			}, timeout, interval).Should(BeTrue())
			sts := &stsList.Items[0]

			By("test pods are not ready")
			stateful := NewStateful(ctx, k8sClient, cluster)
			sts.Status.AvailableReplicas = *sts.Spec.Replicas - 1
			podsReady, _ := stateful.PodsReady(sts)
			Expect(podsReady == false).Should(BeTrue())

			if testCtx.UsingExistingCluster() {
				Eventually(func() bool {
					phase, _ := stateful.CalculatePhaseWhenPodsNotReady(testdbaas.ConsensusComponentName)
					return phase == ""
				}, timeout*5, interval).Should(BeTrue())
			} else {
				testdbaas.MockConsensusComponentPods(testCtx, clusterName)
				phase, _ := stateful.CalculatePhaseWhenPodsNotReady(testdbaas.ConsensusComponentName)
				Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
			}

			By("test pods are ready")
			// mock sts is ready
			testk8s.MockStatefulSetReady(sts)
			podsReady, _ = stateful.PodsReady(sts)
			Expect(podsReady == true).Should(BeTrue())

			By("test component is running")
			isRunning, _ := stateful.IsRunning(sts)
			Expect(isRunning == true).Should(BeTrue())

			By("test handle probe timed out")
			requeue, _ := stateful.HandleProbeTimeoutWhenPodsReady()
			Expect(requeue == false).Should(BeTrue())
		})
	})

})
