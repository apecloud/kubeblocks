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

package consensus

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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
		revisionID                   = "6fdd48d9cd"
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
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	mockClusterStatusProbeTimeout := func(cluster *appsv1alpha1.Cluster) {
		// mock pods ready in component status and probe timed out
		Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
			podsReady := true
			cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
				consensusCompName: {
					PodsReady:     &podsReady,
					PodsReadyTime: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				},
			}
		})).ShouldNot(HaveOccurred())

		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
			g.Expect(tmpCluster.Status.Components).ShouldNot(BeEmpty())
		})).Should(Succeed())
	}

	Context("Consensus Component test", func() {
		It("Consensus Component test", func() {
			By(" init cluster, statefulSet, pods")
			clusterDef, _, cluster := testapps.InitConsensusMysql(&testCtx, clusterDefName,
				clusterVersionName, clusterName, "consensus", consensusCompName)

			sts := testapps.MockConsensusComponentStatefulSet(&testCtx, clusterName, consensusCompName)
			componentName := consensusCompName
			compDefName := cluster.Spec.GetComponentDefRefName(componentName)
			componentDef := clusterDef.GetComponentDefByName(compDefName)
			component := cluster.Spec.GetComponentByName(componentName)

			By("test pods are not ready")
			consensusComponent := newConsensusSet(k8sClient, cluster, component, *componentDef)
			sts.Status.AvailableReplicas = *sts.Spec.Replicas - 1
			podsReady, _ := consensusComponent.PodsReady(ctx, sts)
			Expect(podsReady == false).Should(BeTrue())

			By("test pods are ready")
			// mock sts is ready
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				controllerRevision := fmt.Sprintf("%s-%s-%s", clusterName, consensusCompName, revisionID)
				sts.Status.CurrentRevision = controllerRevision
				sts.Status.UpdateRevision = controllerRevision
				testk8s.MockStatefulSetReady(sts)
			})).Should(Succeed())

			podsReady, _ = consensusComponent.PodsReady(ctx, sts)
			Expect(podsReady == true).Should(BeTrue())

			By("test component is running")
			isRunning, _ := consensusComponent.IsRunning(ctx, sts)
			Expect(isRunning == false).Should(BeTrue())

			podName := sts.Name + "-0"
			podList := testapps.MockConsensusComponentPods(&testCtx, sts, clusterName, consensusCompName)
			By("expect for pod is available")
			Expect(consensusComponent.PodIsAvailable(podList[0], defaultMinReadySeconds)).Should(BeTrue())

			By("test handle probe timed out")
			mockClusterStatusProbeTimeout(cluster)
			// mock leader pod is not ready
			testk8s.UpdatePodStatusNotReady(ctx, testCtx, podName)
			testk8s.DeletePodLabelKey(ctx, testCtx, podName, constant.RoleLabelKey)
			pod := &corev1.Pod{}
			Expect(testCtx.Cli.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)).Should(Succeed())
			phase, _ := consensusComponent.GetPhaseWhenPodsReadyAndProbeTimeout([]*corev1.Pod{pod})
			Expect(phase == appsv1alpha1.FailedClusterCompPhase).Should(BeTrue())

			By("test component is running")
			isRunning, _ = consensusComponent.IsRunning(ctx, sts)
			Expect(isRunning == false).Should(BeTrue())

			By("expect component phase is Failed when pod of component is failed")
			phase, _, _ = consensusComponent.GetPhaseWhenPodsNotReady(ctx, consensusCompName)
			Expect(phase == appsv1alpha1.FailedClusterCompPhase).Should(BeTrue())

			By("not ready pod is not controlled by latest revision, should return empty string")
			// mock pod is not controlled by latest revision
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				sts.Status.UpdateRevision = fmt.Sprintf("%s-%s-%s", clusterName, consensusCompName, "6fdd48d9cd1")
			})).Should(Succeed())
			phase, _, _ = consensusComponent.GetPhaseWhenPodsNotReady(ctx, consensusCompName)
			Expect(len(phase) == 0).Should(BeTrue())
		})
	})
})
