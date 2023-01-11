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

package consensusset

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Consensus Component", func() {
	var (
		randomStr                    = testCtx.GetRandomStr()
		clusterDefName               = "mysql-clusterdef-" + randomStr
		clusterVersionName           = "mysql-clusterversion-" + randomStr
		clusterName                  = "mysql-" + randomStr
		timeout                      = 10 * time.Second
		interval                     = time.Second
		consensusCompName            = "consensus"
		defaultMinReadySeconds int32 = 10
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

	mockClusterStatusProbeTimeout := func(cluster *dbaasv1alpha1.Cluster) {
		// mock pods ready in status component and probe timed out
		clusterPatch := client.MergeFrom(cluster.DeepCopy())
		podsReady := true
		cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
			consensusCompName: {
				PodsReady:     &podsReady,
				PodsReadyTime: &metav1.Time{Time: time.Now().Add(-2 * time.Minute)},
			},
		}
		Expect(k8sClient.Status().Patch(ctx, cluster, clusterPatch)).Should(Succeed())
		Eventually(func() bool {
			tmpCluster := &dbaasv1alpha1.Cluster{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, tmpCluster)
			return tmpCluster.Status.Components != nil
		}, timeout, interval).Should(BeTrue())
	}

	validateComponentStatus := func() {
		Eventually(func() bool {
			tmpCluster := &dbaasv1alpha1.Cluster{}
			_ = k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, tmpCluster)
			return tmpCluster.Status.Components[consensusCompName].Phase == dbaasv1alpha1.FailedPhase
		}, timeout, interval).Should(BeTrue())
	}

	Context("Consensus Component test", func() {
		It("Consensus Component test", func() {
			By(" init cluster, statefulSet, pods")
			clusterDef, _, cluster := testdbaas.InitConsensusMysql(ctx, testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompName)

			sts := testdbaas.MockConsensusComponentStatefulSet(ctx, testCtx, clusterName, consensusCompName)
			componentName := consensusCompName
			typeName := util.GetComponentTypeName(*cluster, componentName)
			componentDef := util.GetComponentDefFromClusterDefinition(clusterDef, typeName)
			component := util.GetComponentByName(cluster, componentName)

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
			if testCtx.UsingExistingCluster() {
				Eventually(func() bool {
					phase, _ := consensusComponent.GetPhaseWhenPodsNotReady(consensusCompName)
					return phase == ""
				}, timeout*5, interval).Should(BeTrue())

				By("test handle probe timed out")
				mockClusterStatusProbeTimeout(cluster)
				requeue, _ := consensusComponent.HandleProbeTimeoutWhenPodsReady()
				Expect(requeue == false).Should(BeTrue())
				validateComponentStatus()
			} else {
				podList := testdbaas.MockConsensusComponentPods(ctx, testCtx, clusterName, consensusCompName)
				By("test pod is not available")
				Expect(consensusComponent.PodIsAvailable(podList[0], defaultMinReadySeconds)).Should(BeTrue())

				By("test handle probe timed out")
				mockClusterStatusProbeTimeout(cluster)
				// mock leader pod is not ready
				testk8s.UpdatePodStatusNotReady(ctx, testCtx, podName)
				testk8s.DeletePodLabelKey(ctx, testCtx, podName, intctrlutil.ConsensusSetRoleLabelKey)
				requeue, _ := consensusComponent.HandleProbeTimeoutWhenPodsReady()
				Expect(requeue == false).Should(BeTrue())
				validateComponentStatus()

				By("test component is running")
				isRunning, _ := consensusComponent.IsRunning(sts)
				Expect(isRunning == false).Should(BeTrue())

				By("test component phase when pods not ready")
				phase, _ := consensusComponent.GetPhaseWhenPodsNotReady(consensusCompName)
				Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
			}
		})
	})

})
