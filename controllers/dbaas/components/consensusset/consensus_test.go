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

package consensusset

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Consensus Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "mysql-clusterdef-" + randomStr
		clusterVersionName = "mysql-clusterversion-" + randomStr
		clusterName        = "mysql-" + randomStr
	)

	const (
		consensusCompName            = "consensus"
		defaultMinReadySeconds int32 = 10
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
		testdbaas.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	mockClusterStatusProbeTimeout := func(cluster *dbaasv1alpha1.Cluster) {
		// mock pods ready in status component and probe timed out
		Eventually(testdbaas.ChangeObjStatus(&testCtx, cluster, func() {
			podsReady := true
			cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				consensusCompName: {
					PodsReady:     &podsReady,
					PodsReadyTime: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				},
			}
		})).Should(Succeed())

		Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
			g.Expect(tmpCluster.Status.Components != nil).Should(BeTrue())
		})).Should(Succeed())
	}

	validateComponentStatus := func(cluster *dbaasv1alpha1.Cluster) {
		Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
			g.Expect(tmpCluster.Status.Components[consensusCompName].Phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
		})).Should(Succeed())
	}

	Context("Consensus Component test", func() {
		It("Consensus Component test", func() {
			By(" init cluster, statefulSet, pods")
			clusterDef, _, cluster := testdbaas.InitConsensusMysql(testCtx, clusterDefName,
				clusterVersionName, clusterName, "consensus", consensusCompName)

			sts := testdbaas.MockConsensusComponentStatefulSet(testCtx, clusterName, consensusCompName)
			componentName := consensusCompName
			typeName := cluster.GetComponentTypeName(componentName)
			componentDef := clusterDef.GetComponentDefByTypeName(typeName)
			component := cluster.GetComponentByName(componentName)

			By("test pods are not ready")
			consensusComponent := NewConsensusSet(ctx, k8sClient, cluster, component, componentDef)
			sts.Status.AvailableReplicas = *sts.Spec.Replicas - 1
			podsReady, _ := consensusComponent.PodsReady(sts)
			Expect(podsReady == false).Should(BeTrue())

			By("test pods are ready")
			// mock sts is ready
			testk8s.MockStatefulSetReady(sts)
			podsReady, _ = consensusComponent.PodsReady(sts)
			Expect(podsReady == true).Should(BeTrue())

			By("test component is running")
			isRunning, _ := consensusComponent.IsRunning(sts)
			Expect(isRunning == false).Should(BeTrue())

			podName := sts.Name + "-0"
			podList := testdbaas.MockConsensusComponentPods(testCtx, sts, clusterName, consensusCompName)
			By("expect for pod is available")
			Expect(consensusComponent.PodIsAvailable(podList[0], defaultMinReadySeconds)).Should(BeTrue())

			By("test handle probe timed out")
			mockClusterStatusProbeTimeout(cluster)
			// mock leader pod is not ready
			testk8s.UpdatePodStatusNotReady(ctx, testCtx, podName)
			testk8s.DeletePodLabelKey(ctx, testCtx, podName, intctrlutil.RoleLabelKey)
			requeue, _ := consensusComponent.HandleProbeTimeoutWhenPodsReady(nil)
			Expect(requeue == false).Should(BeTrue())
			validateComponentStatus(cluster)

			By("test component is running")
			isRunning, _ = consensusComponent.IsRunning(sts)
			Expect(isRunning == false).Should(BeTrue())

			By("test component phase when pods not ready")
			phase, _ := consensusComponent.GetPhaseWhenPodsNotReady(consensusCompName)
			Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
		})
	})
})
