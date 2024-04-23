/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
		Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
		})).ShouldNot(HaveOccurred())
	}

	Context("Test OpsRequest", func() {
		commonHScaleConsensusCompTest := func(reqCtx intctrlutil.RequestCtx, replicas int, offlineInstances []string, instances ...appsv1alpha1.InstanceTemplate) (*OpsResource, []corev1.Pod) {
			By("init operations resources with CLusterDefinition/ClusterVersion/Hybrid components Cluster/consensus Pods")
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			podList := initInstanceSetPods(ctx, k8sClient, opsRes, clusterName)

			By(fmt.Sprintf("create opsRequest for horizontal scaling of consensus component from 3 to %d", replicas))
			initClusterAnnotationAndPhaseForOps(opsRes)
			opsRes.OpsRequest = createHorizontalScaling(clusterName, replicas, offlineInstances, instances...)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
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
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsRunningPhase
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
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
			Expect(len(opsProgressDetails)).Should(Equal(2))
		}

		It("test scaling down replicas", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, 1, nil)
			By("mock two pods are deleted")
			for i := 1; i < 3; i++ {
				pod := &podList[i]
				pod.Kind = constant.PodKind
				testk8s.MockPodIsTerminating(ctx, testCtx, pod)
				testk8s.RemovePodFinalizer(ctx, testCtx, pod)
			}
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})

		It("test scaling out replicas", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _ := commonHScaleConsensusCompTest(reqCtx, 5, nil)
			By("mock two pods are created")
			for i := 3; i < 5; i++ {
				podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, i)
				testapps.MockInstanceSetPod(&testCtx, nil, clusterName, consensusComp, podName, "follower", "Readonly")
			}
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})

		It("test canceling HScale opsRequest which scales down replicas of component", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, 1, nil)

			By("mock one pod has been deleted")
			pod := &podList[2]
			pod.Kind = constant.PodKind
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)

			By("cancel HScale opsRequest after one pod has been deleted")
			cancelOpsRequest(reqCtx, opsRes, time.Now().Add(-1*time.Second))

			By("re-create the deleted pod")
			podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, 2)
			testapps.MockInstanceSetPod(&testCtx, nil, clusterName, consensusComp, podName, "follower", "ReadOnly")

			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			mockConsensusCompToRunning(opsRes)
			checkCancelledSucceed(reqCtx, opsRes)
			Expect(findStatusProgressDetail(opsRes.OpsRequest.Status.Components[consensusComp].ProgressDetails,
				getProgressObjectKey(constant.PodKind, podName)).Status).Should(Equal(appsv1alpha1.SucceedProgressStatus))
		})

		It("test canceling HScale opsRequest which scales out replicas of component", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _ := commonHScaleConsensusCompTest(reqCtx, 5, nil)

			By("mock one pod is created")
			podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, 3)
			pod := testapps.MockInstanceSetPod(&testCtx, nil, clusterName, consensusComp, podName, "follower", "Readonly")

			By("cancel HScale opsRequest after pne pod is created")
			cancelOpsRequest(reqCtx, opsRes, time.Now().Add(-1*time.Second))

			By("delete the created pod")
			pod.Kind = constant.PodKind
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)

			By("expect for opsRequest phase is Succeed after pods has been scaled and component phase is Running")
			mockConsensusCompToRunning(opsRes)
			checkCancelledSucceed(reqCtx, opsRes)
			Expect(findStatusProgressDetail(opsRes.OpsRequest.Status.Components[consensusComp].ProgressDetails,
				getProgressObjectKey(constant.PodKind, pod.Name)).Status).Should(Equal(appsv1alpha1.SucceedProgressStatus))
		})

		It("force run horizontal scaling opsRequests ", func() {
			By("create opsRequest for horizontal scaling")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _ := commonHScaleConsensusCompTest(reqCtx, 5, nil)
			firstOps := opsRes.OpsRequest.DeepCopy()
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
			})).ShouldNot(HaveOccurred())

			By("create opsRequest for horizontal scaling with force flag")
			ops := createHorizontalScaling(clusterName, 4, nil)
			opsRes.OpsRequest = ops
			opsRes.OpsRequest.Spec.Force = true
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.UpdatingClusterCompPhase, consensusComp)

			By("expect for opsRequest phase is Creating after doing action")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("mock the next reconcile")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(testapps.ChangeObjStatus(&testCtx, ops, func() {
				ops.Status.Phase = appsv1alpha1.OpsRunningPhase
			})).Should(Succeed())

			By("expect these opsRequest should not in the queue of the clusters")
			opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster)
			Expect(len(opsRequestSlice)).Should(Equal(2))
			for _, v := range opsRequestSlice {
				Expect(v.InQueue).Should(BeFalse())
			}

			By("reconcile the firstOpsRequest again")
			opsRes.OpsRequest = firstOps
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("expect the firstOpsRequest should be overwritten for the resources")
			override := opsRes.OpsRequest.Status.Components[consensusComp].OverrideBy
			Expect(override).ShouldNot(BeNil())
			Expect(*override.Replicas).Should(Equal(ops.Spec.HorizontalScalingList[0].Replicas))
		})

		It("test scaling down replicas with specified pod", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			specifiedOrdinal := 1
			offlineInstances := []string{fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, specifiedOrdinal)}
			opsRes, podList := commonHScaleConsensusCompTest(reqCtx, 2, offlineInstances)
			By("verify cluster spec is correct")
			var targetSpec *appsv1alpha1.ClusterComponentSpec
			for i := range opsRes.Cluster.Spec.ComponentSpecs {
				spec := &opsRes.Cluster.Spec.ComponentSpecs[i]
				if spec.Name == consensusComp {
					targetSpec = spec
				}
			}
			Expect(targetSpec.Instances).Should(BeNil())
			Expect(targetSpec.OfflineInstances).Should(HaveLen(1))
			Expect(targetSpec.OfflineInstances).Should(Equal(offlineInstances))

			By("mock specified pod (with ordinal 1) deleted")
			pod := &podList[specifiedOrdinal]
			pod.Kind = constant.PodKind
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})

		It("test scaling out replicas with heterogeneous pod", func() {
			templateFoo := appsv1alpha1.InstanceTemplate{
				Name:     "foo",
				Replicas: func() *int32 { r := int32(3); return &r }(),
			}
			templateBar := appsv1alpha1.InstanceTemplate{
				Name:     "bar",
				Replicas: func() *int32 { r := int32(3); return &r }(),
			}
			instances := []appsv1alpha1.InstanceTemplate{templateFoo, templateBar}
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _ := commonHScaleConsensusCompTest(reqCtx, 6, nil, instances...)
			By("verify cluster spec is correct")
			var targetSpec *appsv1alpha1.ClusterComponentSpec
			for i := range opsRes.Cluster.Spec.ComponentSpecs {
				spec := &opsRes.Cluster.Spec.ComponentSpecs[i]
				if spec.Name == consensusComp {
					targetSpec = spec
				}
			}
			Expect(targetSpec.Replicas).Should(BeEquivalentTo(6))
			Expect(targetSpec.Instances).Should(HaveLen(2))
			Expect(targetSpec.Instances).Should(Equal(instances))
			By("mock two pods are created")
			for i := 3; i < 6; i++ {
				podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, i)
				testapps.MockInstanceSetPod(&testCtx, nil, clusterName, consensusComp, podName, "follower", "Readonly")
			}
			checkOpsRequestPhaseIsSucceed(reqCtx, opsRes)
		})
	})
})

func createHorizontalScaling(clusterName string, replicas int, offlineInstances []string, instances ...appsv1alpha1.InstanceTemplate) *appsv1alpha1.OpsRequest {
	horizontalOpsName := "horizontal-scaling-ops-" + testCtx.GetRandomStr()
	ops := testapps.NewOpsRequestObj(horizontalOpsName, testCtx.DefaultNamespace,
		clusterName, appsv1alpha1.HorizontalScalingType)
	ops.Spec.HorizontalScalingList = []appsv1alpha1.HorizontalScaling{
		{
			ComponentOps:     appsv1alpha1.ComponentOps{ComponentName: consensusComp},
			Replicas:         int32(replicas),
			Instances:        instances,
			OfflineInstances: offlineInstances,
		},
	}
	opsRequest := testapps.CreateOpsRequest(ctx, testCtx, ops)
	opsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
	return opsRequest
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
