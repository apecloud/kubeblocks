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

package stateful

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
		clusterDefName     = "mysql1-clusterdef-" + randomStr
		clusterVersionName = "mysql1-clusterversion-" + randomStr
		clusterName        = "mysql1-" + randomStr
		consensusCompName  = "consensus"
		timeout            = 10 * time.Second
		interval           = time.Second
	)
	const defaultMinReadySeconds = 10

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
		testdbaas.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("Stateful Component test", func() {
		It("Stateful Component test", func() {
			By(" init cluster, statefulSet, pods")
			clusterDef, _, cluster := testdbaas.InitConsensusMysql(testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompName)
			_ = testdbaas.MockConsensusComponentStatefulSet(testCtx, clusterName, consensusCompName)
			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				_ = k8sClient.List(ctx, stsList, client.InNamespace(testCtx.DefaultNamespace), client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey:  clusterName,
					intctrlutil.AppComponentLabelKey: consensusCompName,
				}, client.Limit(1))
				return len(stsList.Items) > 0
			}, timeout, interval).Should(BeTrue())
			sts := &stsList.Items[0]

			By("test pods are not ready")
			clusterComponent := cluster.GetComponentByName(consensusCompName)
			componentDef := clusterDef.GetComponentDefByTypeName(clusterComponent.Type)
			stateful := NewStateful(ctx, k8sClient, cluster, clusterComponent, componentDef)
			patch := client.MergeFrom(sts.DeepCopy())
			availableReplicas := *sts.Spec.Replicas - 1
			sts.Status.AvailableReplicas = availableReplicas
			sts.Status.ReadyReplicas = availableReplicas
			sts.Status.Replicas = availableReplicas
			podsReady, _ := stateful.PodsReady(sts)
			Expect(podsReady == false).Should(BeTrue())

			if testCtx.UsingExistingCluster() {
				Eventually(func() bool {
					phase, _ := stateful.GetPhaseWhenPodsNotReady(consensusCompName)
					return phase == ""
				}, timeout*5, interval).Should(BeTrue())
			} else {
				podList := testdbaas.MockConsensusComponentPods(testCtx, sts, clusterName, consensusCompName)
				phase, _ := stateful.GetPhaseWhenPodsNotReady(consensusCompName)
				Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
				Expect(k8sClient.Status().Patch(ctx, sts, patch)).Should(Succeed())

				By("test stateful component is abnormal")
				Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(sts), func(g Gomega, tmpSts *appsv1.StatefulSet) {
					g.Expect(tmpSts.Status.AvailableReplicas == availableReplicas).Should(BeTrue())
				})).Should(Succeed())
				phase, _ = stateful.GetPhaseWhenPodsNotReady(consensusCompName)
				Expect(phase == dbaasv1alpha1.AbnormalPhase).Should(BeTrue())

				By("test pod is ready")
				lastTransTime := metav1.NewTime(time.Now().Add(-1 * (defaultMinReadySeconds + 1) * time.Second))
				testk8s.MockPodAvailable(podList[0], lastTransTime)
				Expect(stateful.PodIsAvailable(podList[0], defaultMinReadySeconds)).Should(BeTrue())
			}

			By("test pods are ready")
			// mock sts is ready
			testk8s.MockStatefulSetReady(sts)
			podsReady, _ = stateful.PodsReady(sts)
			Expect(podsReady == true).Should(BeTrue())

			By("test component.replicas is inconsistent with sts.spec.replicas")
			oldReplicas := clusterComponent.Replicas
			replicas := int32(4)
			clusterComponent.Replicas = &replicas
			isRunning, _ := stateful.IsRunning(sts)
			Expect(isRunning == false).Should(BeTrue())
			// reset replicas
			clusterComponent.Replicas = oldReplicas

			By("test component is running")
			isRunning, _ = stateful.IsRunning(sts)
			Expect(isRunning == true).Should(BeTrue())

			By("test handle probe timed out")
			requeue, _ := stateful.HandleProbeTimeoutWhenPodsReady(nil)
			Expect(requeue == false).Should(BeTrue())
		})
	})

})
