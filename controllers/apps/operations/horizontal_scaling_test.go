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

package operations

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("HorizontalScaling OpsRequest", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initClusterAnnotationAndPhaseForOps := func(opsRes *OpsResource) {
		Expect(opsutil.PatchClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
		})).ShouldNot(HaveOccurred())
	}

	Context("Test OpsRequest", func() {

		commonHScaleConsensusCompTest := func(reqCtx intctrlutil.RequestCtx, replicas int) (*OpsResource, []corev1.Pod) {
			By("init operations resources with CLusterDefinition/ClusterVersion/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			podList := initConsensusPods(ctx, k8sClient, opsRes, clusterName)

			By(fmt.Sprintf("create opsRequest for horizontal scaling of consensus component from 3 to %d", replicas))
			initClusterAnnotationAndPhaseForOps(opsRes)
			opsRes.OpsRequest = createHorizontalScaling(clusterName, replicas)
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.UpdatingClusterCompPhase, consensusComp)

			By("expect for opsRequest phase is Creating after doing action")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By(fmt.Sprintf("expect for the replicas of consensus component is %d after doing action again when opsRequest phase is Creating", replicas))
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Spec.GetComponentByName(consensusComp).Replicas).Should(BeEquivalentTo(replicas))
			})).Should(Succeed())

			By("Test OpsManager.Reconcile function when horizontal scaling OpsRequest is Running")
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsRequestKind
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			return opsRes, podList
		}

		checkOpsRequestPhaseIsSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			// mock consensus component is Running
			mockConsensusCompToRunning(opsRes)
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		}

		checkCancelledSucceed := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsCancelledPhase))
			opsProgressDetails := opsRes.OpsRequest.Status.Components[consensusComp].ProgressDetails
			Expect(len(opsProgressDetails)).Should(Equal(1))
			Expect(opsProgressDetails[0].Status).Should(Equal(appsv1alpha1.SucceedProgressStatus))
		}

		It("test scaling down replicas", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, 1)
			By("mock two pods are deleted")
			for i := 0; i < 2; i++ {
				pod := &podList[i]
				pod.Kind = constant.PodKind
				testk8s.MockPodIsTerminating(ctx, testCtx, pod)
				testk8s.RemovePodFinalizer(ctx, testCtx, pod)
			}
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})

		It("test scaling out replicas", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _ := commonHScaleConsensusCompTest(reqCtx, 5)
			By("mock two pods are created")
			for i := 3; i < 5; i++ {
				podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, i)
				testapps.MockConsensusComponentStsPod(&testCtx, nil, clusterName, consensusComp, podName, "follower", "Readonly")
			}
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})

		It("test canceling HScale opsRequest which scales down replicas of component", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, 1)

			By("mock one pod has been deleted")
			pod := &podList[0]
			pod.Kind = constant.PodKind
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)

			By("cancel HScale opsRequest after one pod has been deleted")
			cancelOpsRequest(reqCtx, opsRes, time.Now().Add(-1*time.Second))

			By("re-create the deleted pod")
			podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, 0)
			testapps.MockConsensusComponentStsPod(&testCtx, nil, clusterName, consensusComp, podName, "leader", "ReadWrite")

			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			mockConsensusCompToRunning(opsRes)
			checkCancelledSucceed(reqCtx, opsRes)
		})

		It("test canceling HScale opsRequest which scales out replicas of component", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _ := commonHScaleConsensusCompTest(reqCtx, 5)

			By("mock one pod is created")
			podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, 3)
			pod := testapps.MockConsensusComponentStsPod(&testCtx, nil, clusterName, consensusComp, podName, "follower", "Readonly")

			By("cancel HScale opsRequest after pne pod is created")
			cancelOpsRequest(reqCtx, opsRes, time.Now().Add(-1*time.Second))

			By("delete the created pod")
			pod.Kind = constant.PodKind
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)

			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			mockConsensusCompToRunning(opsRes)
			checkCancelledSucceed(reqCtx, opsRes)
		})
	})
})

func createHorizontalScaling(clusterName string, replicas int) *appsv1alpha1.OpsRequest {
	horizontalOpsName := "horizontal-scaling-ops-" + testCtx.GetRandomStr()
	ops := testapps.NewOpsRequestObj(horizontalOpsName, testCtx.DefaultNamespace,
		clusterName, appsv1alpha1.HorizontalScalingType)
	ops.Spec.HorizontalScalingList = []appsv1alpha1.HorizontalScaling{
		{
			ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
			Replicas:     int32(replicas),
		},
	}
	return testapps.CreateOpsRequest(ctx, testCtx, ops)
}

func cancelOpsRequest(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, cancelTime time.Time) {
	opsRequest := opsRes.OpsRequest
	opsRequest.Spec.Cancel = true
	opsBehaviour := GetOpsManager().OpsMap[opsRequest.Spec.Type]
	Expect(testapps.ChangeObjStatus(&testCtx, opsRequest, func() {
		opsRequest.Status.CancelTimestamp = metav1.Time{Time: cancelTime}
		opsRequest.Status.Phase = appsv1alpha1.OpsCancellingPhase
	})).Should(Succeed())
	Expect(opsBehaviour.CancelFunc(reqCtx, k8sClient, opsRes)).ShouldNot(HaveOccurred())
}

func mockConsensusCompToRunning(opsRes *OpsResource) {
	// mock consensus component is Running
	compStatus := opsRes.Cluster.Status.Components[consensusComp]
	compStatus.Phase = appsv1alpha1.RunningClusterCompPhase
	opsRes.Cluster.Status.Components[consensusComp] = compStatus
}
