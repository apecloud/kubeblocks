/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package components

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("StatefulSet Controller", func() {

	var (
		randomStr          = testCtx.GetRandomStr()
		clusterName        = "mysql-" + randomStr
		clusterDefName     = "cluster-definition-consensus-" + randomStr
		clusterVersionName = "cluster-version-operations-" + randomStr
		opsRequestName     = "wesql-restart-test-" + randomStr
	)

	const (
		revisionID        = "6fdd48d9cd"
		consensusCompName = "consensus"
		consensusCompType = "consensus"
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	testUpdateStrategy := func(updateStrategy appsv1alpha1.UpdateStrategy, componentName string, index int) {
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKey{Name: clusterDefName},
			func(clusterDef *appsv1alpha1.ClusterDefinition) {
				clusterDef.Spec.ComponentDefs[0].ConsensusSpec.UpdateStrategy = appsv1alpha1.SerialStrategy
			})()).Should(Succeed())

		// mock consensus component is not ready
		objectKey := client.ObjectKey{Name: clusterName + "-" + componentName, Namespace: testCtx.DefaultNamespace}
		Expect(testapps.GetAndChangeObjStatus(&testCtx, objectKey, func(newSts *appsv1.StatefulSet) {
			newSts.Status.UpdateRevision = fmt.Sprintf("%s-%s-%dfdd48d8cd", clusterName, componentName, index)
			newSts.Status.ObservedGeneration = newSts.Generation - 1
		})()).Should(Succeed())
	}

	testUsingEnvTest := func(sts *appsv1.StatefulSet) []*corev1.Pod {
		By("mock statefulset update completed")
		updateRevision := fmt.Sprintf("%s-%s-%s", clusterName, consensusCompName, revisionID)
		Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
			sts.Status.UpdateRevision = updateRevision
			testk8s.MockStatefulSetReady(sts)
			sts.Status.ObservedGeneration = 2
		})).Should(Succeed())

		By("create pods of statefulset")
		pods := testapps.MockConsensusComponentPods(&testCtx, sts, clusterName, consensusCompName)

		By("Mock a pod without role label and it will wait for HandleProbeTimeoutWhenPodsReady")
		leaderPod := pods[0]
		Expect(testapps.ChangeObj(&testCtx, leaderPod, func(lpod *corev1.Pod) {
			delete(lpod.Labels, constant.RoleLabelKey)
		})).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(leaderPod), func(g Gomega, pod *corev1.Pod) {
			g.Expect(pod.Labels[constant.RoleLabelKey] == "").Should(BeTrue())
		})).Should(Succeed())

		By("mock restart component to trigger reconcile of StatefulSet controller")
		Expect(testapps.ChangeObj(&testCtx, sts, func(lsts *appsv1.StatefulSet) {
			lsts.Spec.Template.Annotations = map[string]string{
				constant.RestartAnnotationKey: time.Now().Format(time.RFC3339),
			}
		})).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sts),
			func(g Gomega, fetched *appsv1.StatefulSet) {
				g.Expect(fetched.Status.UpdateRevision).To(Equal(updateRevision))
			})).Should(Succeed())

		By("wait for component podsReady to be true and phase to be 'Rebooting'")
		clusterKey := client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
			compStatus := cluster.Status.Components[consensusCompName]
			g.Expect(compStatus.Phase).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase)) // original expecting value RebootingPhase
			g.Expect(compStatus.PodsReady).ShouldNot(BeNil())
			g.Expect(*compStatus.PodsReady).Should(BeTrue())
			// REVIEW/TODO: ought add extra condition check for RebootingPhase
		})).Should(Succeed())

		By("add leader role label for leaderPod to mock consensus component to be Running")
		Expect(testapps.ChangeObj(&testCtx, leaderPod, func(lpod *corev1.Pod) {
			lpod.Labels[constant.RoleLabelKey] = "leader"
		})).Should(Succeed())
		return pods
	}

	Context("test controller", func() {
		It("test statefulSet controller", func() {
			By("mock cluster object")
			_, _, cluster := testapps.InitConsensusMysql(&testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompType, consensusCompName)

			clusterKey := client.ObjectKeyFromObject(cluster)
			// REVIEW/TODO: "Rebooting" got refactored
			By("mock cluster phase is 'Rebooting' and restart operation is running on cluster")
			Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
				cluster.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase
				cluster.Status.ObservedGeneration = 1
				cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
					consensusCompName: {
						Phase: appsv1alpha1.SpecReconcilingClusterCompPhase,
					},
				}
			})).Should(Succeed())
			_ = testapps.CreateRestartOpsRequest(&testCtx, clusterName, opsRequestName, []string{consensusCompName})
			Expect(testapps.ChangeObj(&testCtx, cluster, func(lcluster *appsv1alpha1.Cluster) {
				lcluster.Annotations = map[string]string{
					constant.OpsRequestAnnotationKey: fmt.Sprintf(`[{"name":"%s","clusterPhase":"Updating"}]`, opsRequestName),
				}
			})).Should(Succeed())

			// trigger statefulset controller Reconcile
			sts := testapps.MockConsensusComponentStatefulSet(&testCtx, clusterName, consensusCompName)

			By("mock the StatefulSet and pods are ready")
			// mock statefulSet available and consensusSet component is running
			pods := testUsingEnvTest(sts)

			By("check the component phase becomes Running")
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, consensusCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))

			By("mock component of cluster is stopping")
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Status.Phase = appsv1alpha1.SpecReconcilingClusterPhase
				tmpCluster.Status.SetComponentStatus(consensusCompName, appsv1alpha1.ClusterComponentStatus{
					Phase: appsv1alpha1.SpecReconcilingClusterCompPhase,
				})
			})()).Should(Succeed())

			By("mock stop operation and processed successfully")
			Expect(testapps.ChangeObj(&testCtx, cluster, func(lcluster *appsv1alpha1.Cluster) {
				lcluster.Spec.ComponentSpecs[0].Replicas = 0
			})).Should(Succeed())
			Expect(testapps.ChangeObj(&testCtx, sts, func(lsts *appsv1.StatefulSet) {
				replicas := int32(0)
				lsts.Spec.Replicas = &replicas
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				testk8s.MockStatefulSetReady(sts)
			})).Should(Succeed())
			// delete all pods of components
			for _, v := range pods {
				testapps.DeleteObject(&testCtx, client.ObjectKeyFromObject(v), v)
			}

			By("check the component phase becomes Stopped")
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, consensusCompName)).Should(Equal(appsv1alpha1.StoppedClusterCompPhase))

			By("test updateStrategy with Serial")
			testUpdateStrategy(appsv1alpha1.SerialStrategy, consensusCompName, 1)

			By("test updateStrategy with Parallel")
			testUpdateStrategy(appsv1alpha1.ParallelStrategy, consensusCompName, 2)
		})
	})
})
