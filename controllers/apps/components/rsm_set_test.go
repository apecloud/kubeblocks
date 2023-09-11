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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("RSM Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "mysql1-clusterdef-" + randomStr
		clusterVersionName = "mysql1-clusterversion-" + randomStr
		clusterName        = "mysql1-" + randomStr
	)
	const (
		defaultMinReadySeconds = 10
		rsmCompDefRef          = "stateful"
		rsmCompName            = "stateful"
	)
	cleanAll := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
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

	Context("RSM Component test", func() {
		FIt("RSM Component test", func() {
			By(" init cluster, statefulSet, pods")
			clusterDef, _, cluster := testapps.InitConsensusMysql(&testCtx, clusterDefName,
				clusterVersionName, clusterName, rsmCompDefRef, rsmCompName)
			rsm := testapps.MockRSMComponent(&testCtx, clusterName, rsmCompName)
			Expect(testapps.ChangeObj(&testCtx, rsm, func(machine *workloads.ReplicatedStateMachine) {
				annotations := machine.Annotations
				if annotations == nil {
					annotations = make(map[string]string, 0)
				}
				annotations[constant.KubeBlocksGenerationKey] = strconv.FormatInt(cluster.Generation, 10)
				machine.Annotations = annotations
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
				cluster.Status.ObservedGeneration = cluster.Generation
			})).Should(Succeed())
			rsmList := &workloads.ReplicatedStateMachineList{}
			Eventually(func() bool {
				_ = k8sClient.List(ctx, rsmList, client.InNamespace(testCtx.DefaultNamespace), client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: rsmCompName,
				}, client.Limit(1))
				return len(rsmList.Items) > 0
			}).Should(BeTrue())
			_ = testapps.MockConsensusComponentStatefulSet(&testCtx, clusterName, rsmCompName)
			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				_ = k8sClient.List(ctx, stsList, client.InNamespace(testCtx.DefaultNamespace), client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: rsmCompName,
				}, client.Limit(1))
				return len(stsList.Items) > 0
			}).Should(BeTrue())
			Expect(testapps.ChangeObjStatus(&testCtx, &stsList.Items[0], func() {
				stsList.Items[0].Status.ObservedGeneration = stsList.Items[0].Generation
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, rsm, func() {
				rsm.Status.ObservedGeneration = rsm.Generation
				rsm.Status.CurrentGeneration = rsm.Generation
				rsm.Status.InitReplicas = *rsm.Spec.Replicas
				rsm.Status.Replicas = *rsm.Spec.Replicas
			})).Should(Succeed())

			By("test pods number of sts is 0")
			rsm = &rsmList.Items[0]
			clusterComponent := cluster.Spec.GetComponentByName(rsmCompName)
			componentDef := clusterDef.GetComponentDefByName(clusterComponent.ComponentDefRef)
			rsmComponent := newRSM(testCtx.Ctx, k8sClient, cluster, clusterDef, clusterComponent, *componentDef)
			phase, _, _ := rsmComponent.GetPhaseWhenPodsNotReady(ctx, rsmCompName, false)
			Expect(phase == appsv1alpha1.FailedClusterCompPhase).Should(BeTrue())

			By("test pods are not ready")
			updateRevision := fmt.Sprintf("%s-%s-%s", clusterName, rsmCompName, "6fdd48d9cd")
			sts := &stsList.Items[0]
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				availableReplicas := *sts.Spec.Replicas - 1
				sts.Status.AvailableReplicas = availableReplicas
				sts.Status.ReadyReplicas = availableReplicas
				sts.Status.Replicas = availableReplicas
				sts.Status.ObservedGeneration = 1
				sts.Status.UpdateRevision = updateRevision
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, rsm, func() {
				availableReplicas := *rsm.Spec.Replicas - 1
				rsm.Status.InitReplicas = *rsm.Spec.Replicas
				rsm.Status.AvailableReplicas = availableReplicas
				rsm.Status.ReadyReplicas = availableReplicas
				rsm.Status.Replicas = availableReplicas
				rsm.Status.ObservedGeneration = 1
				rsm.Status.UpdateRevision = updateRevision
			})).Should(Succeed())
			podsReady, _ := rsmComponent.PodsReady(ctx, rsm)
			Expect(podsReady).Should(BeFalse())

			By("create pods of sts")
			podList := testapps.MockConsensusComponentPods(&testCtx, sts, clusterName, rsmCompName)

			By("test rsm component is abnormal")
			pod := podList[0]
			// mock pod is not ready
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.Conditions = []corev1.PodCondition{}
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(rsm), func(g Gomega, tmpRSM *workloads.ReplicatedStateMachine) {
				g.Expect(tmpRSM.Status.AvailableReplicas == *rsm.Spec.Replicas-1).Should(BeTrue())
			})).Should(Succeed())

			By("should return empty string if pod of component is only not ready when component is not up running")
			phase, _, _ = rsmComponent.GetPhaseWhenPodsNotReady(ctx, rsmCompName, false)
			Expect(string(phase)).Should(Equal(""))

			By("expect component phase is Failed when pod of component is not ready and component is up running")
			phase, _, _ = rsmComponent.GetPhaseWhenPodsNotReady(ctx, rsmCompName, true)
			Expect(phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))

			By("expect component phase is Failed when pod of component is failed")
			testk8s.UpdatePodStatusScheduleFailed(ctx, testCtx, pod.Name, pod.Namespace)
			phase, _, _ = rsmComponent.GetPhaseWhenPodsNotReady(ctx, rsmCompName, false)
			Expect(phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))

			By("not ready pod is not controlled by latest revision, should return empty string")
			// mock pod is not controlled by latest revision
			Expect(testapps.ChangeObj(&testCtx, pod, func(lpod *corev1.Pod) {
				lpod.Labels[appsv1.ControllerRevisionHashLabelKey] = fmt.Sprintf("%s-%s-%s", clusterName, rsmCompName, "5wdsd8d9fs")
			})).Should(Succeed())
			phase, _, _ = rsmComponent.GetPhaseWhenPodsNotReady(ctx, rsmCompName, false)
			Expect(string(phase)).Should(Equal(""))
			// reset updateRevision
			Expect(testapps.ChangeObj(&testCtx, pod, func(lpod *corev1.Pod) {
				lpod.Labels[appsv1.ControllerRevisionHashLabelKey] = updateRevision
			})).Should(Succeed())

			By("test pod is available")
			lastTransTime := metav1.NewTime(time.Now().Add(-1 * (defaultMinReadySeconds + 1) * time.Second))
			testk8s.MockPodAvailable(pod, lastTransTime)
			Expect(rsmComponent.PodIsAvailable(pod, defaultMinReadySeconds)).Should(BeTrue())

			By("test pods are ready")
			// mock sts is ready
			testk8s.MockStatefulSetReady(sts)
			testk8s.MockRSMReady(rsm, podList...)
			Eventually(func() bool {
				podsReady, _ = rsmComponent.PodsReady(ctx, rsm)
				return podsReady
			}).Should(BeTrue())

			By("test component.replicas is inconsistent with rsm.spec.replicas")
			oldReplicas := rsmComponent.SynthesizedComponent.Replicas
			replicas := int32(4)
			rsmComponent.SynthesizedComponent.Replicas = replicas
			Expect(testapps.ChangeObj(&testCtx, rsm, func(machine *workloads.ReplicatedStateMachine) {
				rsm.Annotations[constant.KubeBlocksGenerationKey] = "new-generation"
			})).Should(Succeed())
			isRunning, _ := rsmComponent.IsRunning(ctx, rsm)
			Expect(isRunning).Should(BeFalse())
			// reset replicas
			rsmComponent.SynthesizedComponent.Replicas = oldReplicas
			Expect(testapps.ChangeObj(&testCtx, rsm, func(machine *workloads.ReplicatedStateMachine) {
				rsm.Annotations[constant.KubeBlocksGenerationKey] = strconv.FormatInt(cluster.Generation, 10)
			})).Should(Succeed())

			By("test component is running")
			isRunning, _ = rsmComponent.IsRunning(ctx, rsm)
			Expect(isRunning).Should(BeTrue())
		})
	})

})
