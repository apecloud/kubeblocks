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

package operations

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/dbaas/operations/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("OpsRequest Controller", func() {

	var (
		timeout               = 10 * time.Second
		interval              = 1 * time.Second
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
		consensusCompName     = "consensus"
		statelessCompName     = "stateless"
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey},
			client.GracePeriodSeconds(0))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	initClusterForOps := func(opsRes *OpsResource) {
		Expect(opsutil.PatchClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
	}

	testVerticalScaling := func(opsRes *OpsResource) {
		By("Test VerticalScaling")
		ops := testdbaas.GenerateOpsRequestObj("verticalscaling-ops-"+randomStr, clusterName, dbaasv1alpha1.VerticalScalingType)
		ops.Spec.VerticalScalingList = []dbaasv1alpha1.VerticalScaling{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: consensusCompName},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("300Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("400m"),
						corev1.ResourceMemory: resource.MustParse("300Mi"),
					},
				},
			},
		}
		opsRes.OpsRequest = testdbaas.CreateOpsRequest(ctx, testCtx, ops)
		initClusterForOps(opsRes)
		By("test save last configuration and OpsRequest phase is Running")
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Eventually(testdbaas.GetOpsRequestPhase(ctx, testCtx, ops.Name), timeout, interval).Should(Equal(dbaasv1alpha1.RunningPhase))

		By("test vertical scale action function")
		vsHandler := verticalScalingHandler{}
		Expect(vsHandler.Action(opsRes)).Should(Succeed())
		_, _, err := vsHandler.ReconcileAction(opsRes)
		Expect(err == nil).Should(BeTrue())
	}

	getProgressDetailStatus := func(opsRes *OpsResource,
		componentName string,
		pod *corev1.Pod) dbaasv1alpha1.ProgressStatus {
		objectKey := GetProgressObjectKey(pod.Kind, pod.Name)
		progressDetails := opsRes.OpsRequest.Status.Components[componentName].ProgressDetails
		progressDetail := FindStatusProgressDetail(progressDetails, objectKey)
		var status dbaasv1alpha1.ProgressStatus
		if progressDetail != nil {
			status = progressDetail.Status
		}
		return status
	}

	testConsensusSetPodUpdating := func(opsRes *OpsResource, consensusPodList []corev1.Pod) {
		By("mock pod of statefulSet updating by deleting the pod")
		pod := &consensusPodList[0]
		testk8s.MockPodIsTerminating(ctx, testCtx, pod)
		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(getProgressDetailStatus(opsRes, consensusCompName, pod)).Should(Equal(dbaasv1alpha1.ProcessingProgressStatus))

		By("mock one pod of StatefulSet to update successfully")
		testk8s.RemovePodFinalizer(ctx, testCtx, pod)
		testdbaas.MockConsensusComponentStsPod(ctx, testCtx, clusterName, consensusCompName,
			pod.Name, "leader", "ReadWrite")

		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(getProgressDetailStatus(opsRes, consensusCompName, pod)).Should(Equal(dbaasv1alpha1.SucceedProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/4"))
	}

	testStatelessPodUpdating := func(opsRes *OpsResource, pod *corev1.Pod) {
		By("create a new pod")
		newPodName := "busybox-" + testCtx.GetRandomStr()
		testdbaas.MockStatelessPod(ctx, testCtx, clusterName, statelessCompName, newPodName)
		newPod := &corev1.Pod{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: newPodName, Namespace: testCtx.DefaultNamespace}, newPod)).Should(Succeed())
		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(getProgressDetailStatus(opsRes, statelessCompName, newPod)).Should(Equal(dbaasv1alpha1.ProcessingProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/4"))

		By("mock new pod is ready")
		lastTransTime := metav1.NewTime(time.Now().Add(-11 * time.Second))
		patch := client.MergeFrom(newPod.DeepCopy())
		testk8s.MockPodAvailable(newPod, lastTransTime)
		Expect(k8sClient.Status().Patch(ctx, newPod, patch)).Should(Succeed())
		_, _ = GetOpsManager().Reconcile(opsRes)
		Expect(getProgressDetailStatus(opsRes, statelessCompName, newPod)).Should(Equal(dbaasv1alpha1.SucceedProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/4"))
	}

	testRestart := func(opsRes *OpsResource, consensusPodList []corev1.Pod, statelessPod *corev1.Pod) {
		By("Test Restart")
		ops := testdbaas.GenerateOpsRequestObj("restart-ops-"+randomStr, clusterName, dbaasv1alpha1.RestartType)
		ops.Spec.RestartList = []dbaasv1alpha1.ComponentOps{
			{ComponentName: consensusCompName},
			{ComponentName: statelessCompName},
		}

		By("test restart OpsRequest is Running")
		initClusterForOps(opsRes)
		opsRes.OpsRequest = testdbaas.CreateOpsRequest(ctx, testCtx, ops)
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Eventually(testdbaas.GetOpsRequestPhase(ctx, testCtx, ops.Name), timeout, interval).Should(Equal(dbaasv1alpha1.RunningPhase))

		By("test restart action and reconcile function")
		testdbaas.MockConsensusComponentStatefulSet(ctx, testCtx, clusterName, consensusCompName)
		testdbaas.MockStatelessComponentDeploy(ctx, testCtx, clusterName, statelessCompName)
		rHandler := restartOpsHandler{}
		_ = rHandler.Action(opsRes)
		_, err := GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())

		if !testCtx.UsingExistingCluster() {
			By("mock testing the updates of consensus component")
			testConsensusSetPodUpdating(opsRes, consensusPodList)

			By("mock testing the updates of stateless component")
			Expect(opsRes.OpsRequest.Status.Components[statelessCompName].Phase).Should(Equal(dbaasv1alpha1.UpdatingPhase))
			testStatelessPodUpdating(opsRes, statelessPod)
		}
	}

	testUpgrade := func(opsRes *OpsResource, clusterObject *dbaasv1alpha1.Cluster) {
		By("Test Upgrade Ops")
		newClusterVersionName := "clusterversion-upgrade-" + randomStr
		_ = testdbaas.CreateHybridCompsClusterVersionForUpgrade(ctx, testCtx, clusterDefinitionName, newClusterVersionName)
		ops := testdbaas.GenerateOpsRequestObj("upgrade-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.UpgradeType)
		ops.Spec.Upgrade = &dbaasv1alpha1.Upgrade{ClusterVersionRef: newClusterVersionName}
		opsRes.OpsRequest = testdbaas.CreateOpsRequest(ctx, testCtx, ops)

		By("test upgrade OpsRequest phase is Running")
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Expect(opsRes.OpsRequest.Status.Phase == dbaasv1alpha1.RunningPhase).Should(BeTrue())

		By("Test OpsManager.MainEnter function ")
		_, _ = GetOpsManager().Reconcile(opsRes)
	}

	createHorizontalScaling := func(replicas int) *dbaasv1alpha1.OpsRequest {
		horizontalOpsName := "horizontalscaling-ops-" + testCtx.GetRandomStr()
		ops := testdbaas.GenerateOpsRequestObj(horizontalOpsName, clusterName, dbaasv1alpha1.HorizontalScalingType)
		ops.Spec.HorizontalScalingList = []dbaasv1alpha1.HorizontalScaling{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: consensusCompName},
				Replicas:     int32(replicas),
			},
		}
		return testdbaas.CreateOpsRequest(ctx, testCtx, ops)
	}

	testHorizontalScaling := func(opsRes *OpsResource, podList []corev1.Pod) {
		By("Test HorizontalScaling with scale down replicas")
		opsRes.OpsRequest = createHorizontalScaling(1)
		initClusterForOps(opsRes)

		By("Test HorizontalScaling OpsRequest phase is running and do action")
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Expect(opsRes.OpsRequest.Status.Phase == dbaasv1alpha1.RunningPhase).Should(BeTrue())

		By("Test OpsManager.Reconcile function when horizontal scaling OpsRequest is Running")
		opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
		_, err := GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())

		if !testCtx.UsingExistingCluster() {
			By("mock the pod is terminating")
			pod := &podList[0]
			testk8s.MockPodIsTerminating(ctx, testCtx, pod)
			_, _ = GetOpsManager().Reconcile(opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusCompName, pod)).Should(Equal(dbaasv1alpha1.ProcessingProgressStatus))

			By("mock the pod is deleted and progressDetail status should be succeed")
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)
			_, _ = GetOpsManager().Reconcile(opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusCompName, pod)).Should(Equal(dbaasv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/2"))
		}

		By("test GetOpsRequestAnnotation function")
		patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
		opsAnnotationString := fmt.Sprintf(`[{"name":"%s","clusterPhase":"Updating"},{"name":"test-not-exists-ops","clusterPhase":"VolumeExpanding"}]`,
			opsRes.OpsRequest.Name)
		opsRes.Cluster.Annotations = map[string]string{
			intctrlutil.OpsRequestAnnotationKey: opsAnnotationString,
		}
		Expect(k8sClient.Patch(ctx, opsRes.Cluster, patch)).Should(Succeed())
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())

		By("Test OpsManager.Reconcile when opsRequest is succeed")
		opsRes.OpsRequest.Status.Phase = dbaasv1alpha1.SucceedPhase
		opsRes.Cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
			consensusCompName: {
				Phase: dbaasv1alpha1.RunningPhase,
			},
		}
		_, err = GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())

		By("Test HorizontalScaling with scale up replica")
		initClusterForOps(opsRes)
		expectClusterComponentReplicas := int32(2)
		opsRes.Cluster.Spec.Components[1].Replicas = &expectClusterComponentReplicas
		opsRes.OpsRequest = createHorizontalScaling(3)
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())

		_, err = GetOpsManager().Reconcile(opsRes)
		Expect(err == nil).Should(BeTrue())
		if !testCtx.UsingExistingCluster() {
			By("mock scale up pods")
			podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusCompName, 0)
			testdbaas.MockConsensusComponentStsPod(ctx, testCtx, clusterName, consensusCompName,
				podName, "leader", "ReadWrite")
			pod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)).Should(Succeed())
			_, _ = GetOpsManager().Reconcile(opsRes)
			Expect(getProgressDetailStatus(opsRes, consensusCompName, pod)).Should(Equal(dbaasv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
		}
	}

	Context("Test OpsRequest", func() {
		It("Should Test all OpsRequest", func() {
			_, _, clusterObject := testdbaas.InitClusterWithHybridComps(ctx, testCtx, clusterDefinitionName,
				clusterVersionName, clusterName, statelessCompName, consensusCompName)
			opsRes := &OpsResource{
				Ctx:      context.Background(),
				Cluster:  clusterObject,
				Client:   k8sClient,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
			By("mock cluster is Running and the status operations")
			patch := client.MergeFrom(clusterObject.DeepCopy())
			clusterObject.Status.Phase = dbaasv1alpha1.RunningPhase
			clusterObject.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				consensusCompName: {
					Phase: dbaasv1alpha1.RunningPhase,
					Type:  "consensus",
				},
				statelessCompName: {
					Phase: dbaasv1alpha1.RunningPhase,
					Type:  "stateless",
				},
			}
			clusterObject.Status.Operations = &dbaasv1alpha1.Operations{
				Upgradable:       true,
				Restartable:      []string{consensusCompName, statelessCompName},
				VerticalScalable: []string{consensusCompName, statelessCompName},
				HorizontalScalable: []dbaasv1alpha1.OperationComponent{
					{
						Name: consensusCompName,
					},
					{
						Name: statelessCompName,
					},
				},
			}
			opsRes.Cluster = clusterObject
			Expect(k8sClient.Status().Patch(context.Background(), clusterObject, patch)).Should(Succeed())

			var (
				consensusPodList []corev1.Pod
				statelessPod     = &corev1.Pod{}
			)
			if !testCtx.UsingExistingCluster() {
				// mock the pods of consensusSet component
				testdbaas.MockConsensusComponentPods(ctx, testCtx, clusterName, consensusCompName)
				podList, err := util.GetComponentPodList(opsRes.Ctx, opsRes.Client, opsRes.Cluster, consensusCompName)
				Expect(err).Should(Succeed())
				consensusPodList = podList.Items

				// mock the pods od stateless component
				podName := "busybox-" + randomStr
				testdbaas.MockStatelessPod(ctx, testCtx, clusterName, statelessCompName, podName)
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, statelessPod)).Should(Succeed())
			}
			// the opsRequest will use startTime to check some condition.
			// if there is no sleep for 1 second, unstable error may occur.
			time.Sleep(time.Second)

			// test upgrade OpsRequest
			testUpgrade(opsRes, clusterObject)

			// test vertical scaling OpsRequest
			testVerticalScaling(opsRes)

			// test restart consensus component and stateless component
			testRestart(opsRes, consensusPodList, statelessPod)

			// test horizontalScaling and test the progressDetail
			testHorizontalScaling(opsRes, consensusPodList)

			By("Test the functions in ops_util.go")
			Expect(PatchValidateErrorCondition(opsRes, "validate error")).Should(Succeed())
			Expect(patchOpsHandlerNotSupported(opsRes)).Should(Succeed())
			Expect(isOpsRequestFailedPhase(dbaasv1alpha1.FailedPhase)).Should(BeTrue())
			Expect(PatchClusterNotFound(opsRes)).Should(Succeed())
			opsRecorder := []dbaasv1alpha1.OpsRecorder{
				{
					Name:           "mysql-restart",
					ToClusterPhase: dbaasv1alpha1.UpdatingPhase,
				},
			}
			Expect(patchClusterPhaseWhenExistsOtherOps(opsRes, opsRecorder)).Should(Succeed())
			index, opsRecord := GetOpsRecorderFromSlice(opsRecorder, "mysql-restart")
			Expect(index == 0 && opsRecord.Name == "mysql-restart").Should(BeTrue())
		})
	})
})
