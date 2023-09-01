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

	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("Stateful Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "mysql1-clusterdef-" + randomStr
		clusterVersionName = "mysql1-clusterversion-" + randomStr
		clusterName        = "mysql1-" + randomStr
	)
	const (
		defaultMinReadySeconds = 10
		statefulCompDefRef     = "stateful"
		statefulCompName       = "stateful"
	)
	cleanAll := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		apps.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		apps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		apps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("Stateful Component test", func() {
		It("Stateful Component test", func() {
			By(" init cluster, statefulSet, pods")
			clusterDef, _, cluster := apps.InitConsensusMysql(&testCtx, clusterDefName,
				clusterVersionName, clusterName, statefulCompDefRef, statefulCompName)
			_ = apps.MockConsensusComponentStatefulSet(&testCtx, clusterName, statefulCompName)
			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				_ = k8sClient.List(ctx, stsList, client.InNamespace(testCtx.DefaultNamespace), client.MatchingLabels{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: statefulCompName,
				}, client.Limit(1))
				return len(stsList.Items) > 0
			}).Should(BeTrue())

			By("test pods number of sts is 0")
			sts := &stsList.Items[0]
			clusterComponent := cluster.Spec.GetComponentByName(statefulCompName)
			componentDef := clusterDef.GetComponentDefByName(clusterComponent.ComponentDefRef)
			stateful := newStateful(k8sClient, cluster, clusterComponent, *componentDef)
			phase, _, _ := stateful.GetPhaseWhenPodsNotReady(ctx, statefulCompName, false)
			Expect(phase == appsv1alpha1.FailedClusterCompPhase).Should(BeTrue())

			By("test pods are not ready")
			updateRevision := fmt.Sprintf("%s-%s-%s", clusterName, statefulCompName, "6fdd48d9cd")
			Expect(apps.ChangeObjStatus(&testCtx, sts, func() {
				availableReplicas := *sts.Spec.Replicas - 1
				sts.Status.AvailableReplicas = availableReplicas
				sts.Status.ReadyReplicas = availableReplicas
				sts.Status.Replicas = availableReplicas
				sts.Status.ObservedGeneration = 1
				sts.Status.UpdateRevision = updateRevision
			})).Should(Succeed())
			podsReady, _ := stateful.PodsReady(ctx, sts)
			Expect(podsReady).Should(BeFalse())

			By("create pods of sts")
			podList := apps.MockConsensusComponentPods(&testCtx, sts, clusterName, statefulCompName)

			By("test stateful component is abnormal")
			pod := podList[0]
			// mock pod is not ready
			Expect(apps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.Conditions = []corev1.PodCondition{}
			})).Should(Succeed())
			// mock pod scheduled failure
			// testk8s.UpdatePodStatusScheduleFailed(ctx, testCtx, pod.Name, pod.Namespace)
			Eventually(apps.CheckObj(&testCtx, client.ObjectKeyFromObject(sts), func(g Gomega, tmpSts *appsv1.StatefulSet) {
				g.Expect(tmpSts.Status.AvailableReplicas == *sts.Spec.Replicas-1).Should(BeTrue())
			})).Should(Succeed())

			By("should return empty string if pod of component is only not ready when component is not up running")
			phase, _, _ = stateful.GetPhaseWhenPodsNotReady(ctx, statefulCompName, false)
			Expect(string(phase)).Should(Equal(""))

			By("expect component phase is Failed when pod of component is not ready and component is up running")
			phase, _, _ = stateful.GetPhaseWhenPodsNotReady(ctx, statefulCompName, true)
			Expect(phase).Should(Equal(appsv1alpha1.AbnormalClusterCompPhase))

			By("expect component phase is Abnormal when pod of component is failed")
			testutil.UpdatePodStatusScheduleFailed(ctx, testCtx, pod.Name, pod.Namespace)
			phase, _, _ = stateful.GetPhaseWhenPodsNotReady(ctx, statefulCompName, false)
			Expect(phase).Should(Equal(appsv1alpha1.AbnormalClusterCompPhase))

			By("not ready pod is not controlled by latest revision, should return empty string")
			// mock pod is not controlled by latest revision
			Expect(apps.ChangeObj(&testCtx, pod, func(lpod *corev1.Pod) {
				lpod.Labels[appsv1.ControllerRevisionHashLabelKey] = fmt.Sprintf("%s-%s-%s", clusterName, statefulCompName, "5wdsd8d9fs")
			})).Should(Succeed())
			phase, _, _ = stateful.GetPhaseWhenPodsNotReady(ctx, statefulCompName, false)
			Expect(string(phase)).Should(Equal(""))
			// reset updateRevision
			Expect(apps.ChangeObj(&testCtx, pod, func(lpod *corev1.Pod) {
				lpod.Labels[appsv1.ControllerRevisionHashLabelKey] = updateRevision
			})).Should(Succeed())

			By("test pod is available")
			lastTransTime := metav1.NewTime(time.Now().Add(-1 * (defaultMinReadySeconds + 1) * time.Second))
			testutil.MockPodAvailable(pod, lastTransTime)
			Expect(stateful.PodIsAvailable(pod, defaultMinReadySeconds)).Should(BeTrue())

			By("test pods are ready")
			// mock sts is ready
			testutil.MockStatefulSetReady(sts)
			podsReady, _ = stateful.PodsReady(ctx, sts)
			Expect(podsReady).Should(BeTrue())

			By("test component.replicas is inconsistent with sts.spec.replicas")
			oldReplicas := clusterComponent.Replicas
			replicas := int32(4)
			clusterComponent.Replicas = replicas
			isRunning, _ := stateful.IsRunning(ctx, sts)
			Expect(isRunning).Should(BeFalse())
			// reset replicas
			clusterComponent.Replicas = oldReplicas

			By("test component is running")
			isRunning, _ = stateful.IsRunning(ctx, sts)
			Expect(isRunning).Should(BeTrue())

			// TODO(refactor): probe timed-out pod
			// By("test handle probe timed out")
			// requeue, _ := stateful.HandleProbeTimeoutWhenPodsReady(ctx, nil)
			// Expect(requeue == false).Should(BeTrue())
		})
	})

})
