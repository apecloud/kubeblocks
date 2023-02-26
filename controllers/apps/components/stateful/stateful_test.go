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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
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

	Context("Stateful Component test", func() {
		It("Stateful Component test", func() {
			By(" init cluster, statefulSet, pods")
			clusterDef, _, cluster := testapps.InitConsensusMysql(testCtx, clusterDefName,
				clusterVersionName, clusterName, statefulCompDefRef, statefulCompName)
			_ = testapps.MockConsensusComponentStatefulSet(testCtx, clusterName, statefulCompName)
			stsList := &appsv1.StatefulSetList{}
			Eventually(func() bool {
				_ = k8sClient.List(ctx, stsList, client.InNamespace(testCtx.DefaultNamespace), client.MatchingLabels{
					intctrlutil.AppInstanceLabelKey:    clusterName,
					intctrlutil.KBAppComponentLabelKey: statefulCompName,
				}, client.Limit(1))
				return len(stsList.Items) > 0
			}).Should(BeTrue())

			By("test pods number of sts is 0")
			sts := &stsList.Items[0]
			clusterComponent := cluster.GetComponentByName(statefulCompName)
			componentDef := clusterDef.GetComponentDefByName(clusterComponent.ComponentDefRef)
			stateful := NewStateful(ctx, k8sClient, cluster, clusterComponent, componentDef)
			phase, _ := stateful.GetPhaseWhenPodsNotReady(statefulCompName)
			Expect(phase == appsv1alpha1.FailedPhase).Should(BeTrue())

			By("test pods are not ready")
			updateRevison := fmt.Sprintf("%s-%s-%s", clusterName, statefulCompName, "6fdd48d9cd")
			Expect(testapps.ChangeObjStatus(&testCtx, sts, func() {
				availableReplicas := *sts.Spec.Replicas - 1
				sts.Status.AvailableReplicas = availableReplicas
				sts.Status.ReadyReplicas = availableReplicas
				sts.Status.Replicas = availableReplicas
				sts.Status.ObservedGeneration = 1
				sts.Status.UpdateRevision = updateRevison
			})).Should(Succeed())
			podsReady, _ := stateful.PodsReady(sts)
			Expect(podsReady == false).Should(BeTrue())

			By("create pods of sts")
			podList := testapps.MockConsensusComponentPods(testCtx, sts, clusterName, statefulCompName)

			By("test stateful component is abnormal")
			// mock pod is not ready
			pod := podList[0]
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.Conditions = nil
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sts), func(g Gomega, tmpSts *appsv1.StatefulSet) {
				g.Expect(tmpSts.Status.AvailableReplicas == *sts.Spec.Replicas-1).Should(BeTrue())
			})).Should(Succeed())
			phase, _ = stateful.GetPhaseWhenPodsNotReady(statefulCompName)
			Expect(phase == appsv1alpha1.AbnormalPhase).Should(BeTrue())

			By("not ready pod is not controlled by latest revision, should return empty string")
			// mock pod is not controlled by latest revision
			Expect(testapps.ChangeObj(&testCtx, pod, func() {
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = fmt.Sprintf("%s-%s-%s", clusterName, statefulCompName, "5wdsd8d9fs")
			})).Should(Succeed())
			phase, _ = stateful.GetPhaseWhenPodsNotReady(statefulCompName)
			Expect(len(phase) == 0).Should(BeTrue())
			// reset updateRevision
			Expect(testapps.ChangeObj(&testCtx, pod, func() {
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = updateRevison
			})).Should(Succeed())

			By("test pod is available")
			lastTransTime := metav1.NewTime(time.Now().Add(-1 * (defaultMinReadySeconds + 1) * time.Second))
			testk8s.MockPodAvailable(pod, lastTransTime)
			Expect(stateful.PodIsAvailable(pod, defaultMinReadySeconds)).Should(BeTrue())

			By("test pods are ready")
			// mock sts is ready
			testk8s.MockStatefulSetReady(sts)
			podsReady, _ = stateful.PodsReady(sts)
			Expect(podsReady == true).Should(BeTrue())

			By("test component.replicas is inconsistent with sts.spec.replicas")
			oldReplicas := clusterComponent.Replicas
			replicas := int32(4)
			clusterComponent.Replicas = replicas
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
