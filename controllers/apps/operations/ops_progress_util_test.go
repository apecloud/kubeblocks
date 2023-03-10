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
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Ops ProgressDetails", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
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

	initClusterForOps := func(opsRes *OpsResource) {
		Expect(opsutil.PatchClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		opsRes.Cluster.Status.Phase = appsv1alpha1.RunningPhase
	}

	testProgressDetailsWithStatefulPodUpdating := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, consensusPodList []corev1.Pod) {
		By("mock pod of statefulSet updating by deleting the pod")
		pod := &consensusPodList[0]
		testk8s.MockPodIsTerminating(ctx, testCtx, pod)
		_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(appsv1alpha1.ProcessingProgressStatus))

		By("mock one pod of StatefulSet to update successfully")
		testk8s.RemovePodFinalizer(ctx, testCtx, pod)
		testapps.MockConsensusComponentStsPod(testCtx, nil, clusterName, consensusComp,
			pod.Name, "leader", "ReadWrite")

		_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(appsv1alpha1.SucceedProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/4"))
	}

	testProgressDetailsWithStatelessPodUpdating := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource) {
		By("create a new pod")
		newPodName := "busybox-" + testCtx.GetRandomStr()
		testapps.MockStatelessPod(testCtx, nil, clusterName, statelessComp, newPodName)
		newPod := &corev1.Pod{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: newPodName, Namespace: testCtx.DefaultNamespace}, newPod)).Should(Succeed())
		_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(getProgressDetailStatus(opsRes, statelessComp, newPod)).Should(Equal(appsv1alpha1.ProcessingProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/4"))

		By("mock new pod is ready")
		Expect(testapps.ChangeObjStatus(&testCtx, newPod, func() {
			lastTransTime := metav1.NewTime(time.Now().Add(-11 * time.Second))
			testk8s.MockPodAvailable(newPod, lastTransTime)
		})).Should(Succeed())

		_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(getProgressDetailStatus(opsRes, statelessComp, newPod)).Should(Equal(appsv1alpha1.SucceedProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/4"))
	}

	Context("Test Ops ProgressDetails", func() {

		It("Test Ops ProgressDetails", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

			By("create restart ops and pods of consensus component")
			opsRes.OpsRequest = createRestartOpsObj(clusterName, "restart-"+randomStr)
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.RebootingPhase, consensusComp, statelessComp)
			podList := initConsensusPods(ctx, k8sClient, opsRes, clusterName)

			By("mock restart OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			By("test the progressDetails when stateful pod updates during restart operation")
			testProgressDetailsWithStatefulPodUpdating(reqCtx, opsRes, podList)

			By("test the progressDetails when stateless pod updates during restart operation")
			Expect(opsRes.OpsRequest.Status.Components[statelessComp].Phase).Should(Equal(appsv1alpha1.RebootingPhase))
			testProgressDetailsWithStatelessPodUpdating(reqCtx, opsRes)

			By("create horizontalScaling operation to test the progressDetails when scaling down the replicas")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, 1)
			mockComponentIsOperating(opsRes.Cluster, appsv1alpha1.HorizontalScalingPhase, consensusComp)
			initClusterForOps(opsRes)

			By("mock HorizontalScaling OpsRequest phase is running")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))
			// do h-scale action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("mock the pod is terminating")
			pod := &podList[0]
			pod.Kind = constant.PodKind
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(appsv1alpha1.ProcessingProgressStatus))

			By("mock the pod is deleted and progressDetail status should be succeed")
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(appsv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/2"))

			By("create horizontalScaling operation to test the progressDetails when scaling up the replicas ")
			initClusterForOps(opsRes)
			expectClusterComponentReplicas := int32(2)
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Spec.ComponentSpecs[1].Replicas = expectClusterComponentReplicas
			})).Should(Succeed())
			opsRes.OpsRequest = createHorizontalScaling(clusterName, 3)
			// update ops phase to Running first
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))
			// do h-scale cluster
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("test the progressDetails when scaling up replicas")
			podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusComp, 0)
			testapps.MockConsensusComponentStsPod(testCtx, nil, clusterName, consensusComp,
				podName, "leader", "ReadWrite")
			pod = &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)).Should(Succeed())
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusComp, pod)).Should(Equal(appsv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
		})

	})
})

func getProgressDetailStatus(opsRes *OpsResource, componentName string, pod *corev1.Pod) appsv1alpha1.ProgressStatus {
	objectKey := GetProgressObjectKey(pod.Kind, pod.Name)
	progressDetails := opsRes.OpsRequest.Status.Components[componentName].ProgressDetails
	progressDetail := FindStatusProgressDetail(progressDetails, objectKey)
	var status appsv1alpha1.ProgressStatus
	if progressDetail != nil {
		status = progressDetail.Status
	}
	return status
}
