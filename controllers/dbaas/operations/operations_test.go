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
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: testdbaas.ConsensusComponentName},
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
		opsRes.OpsRequest = ops
		initClusterForOps(opsRes)
		testdbaas.CreateOpsRequest(testCtx, ops)
		By("test save last configuration and OpsRequest phase is Running")
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Eventually(testdbaas.ExpectOpsRequestPhase(testCtx, ops.Name, dbaasv1alpha1.RunningPhase)).Should(BeTrue())

		By("test vertical scale action function")
		vsHandler := verticalScalingHandler{}
		Expect(vsHandler.Action(opsRes)).Should(Succeed())
		_, _, err := vsHandler.ReconcileAction(opsRes)
		Expect(err == nil).Should(BeTrue())
	}

	expectProgressDetailStatus := func(opsRes *OpsResource,
		componentName string,
		pod *corev1.Pod,
		expectStatus dbaasv1alpha1.ProgressStatus) {
		objectKey := GetProgressObjectKey(pod.Kind, pod.Name)
		progressDetail := opsRes.OpsRequest.Status.Components[componentName].ProgressDetails
		Expect(FindStatusProgressDetail(progressDetail, objectKey).Status == expectStatus).Should(BeTrue())
	}

	testConsensusSetPodUpdating := func(opsRes *OpsResource, consensusPodList []*corev1.Pod) {
		By("mock pod of statefulSet updating by deleting the pod")
		pod := consensusPodList[0]
		testk8s.MockPodIsTerminating(testCtx, pod)
		_, _ = GetOpsManager().Reconcile(opsRes)
		expectProgressDetailStatus(opsRes, testdbaas.ConsensusComponentName, consensusPodList[0], dbaasv1alpha1.ProcessingProgressStatus)

		By("mock one pod of StatefulSet to update successfully")
		testk8s.RemovePodFinalizer(testCtx, consensusPodList[0])
		testdbaas.MockConsensusComponentStsPod(testCtx, clusterName, pod.Name, "leader", "ReadWrite")
		Eventually(func() string {
			_, _ = GetOpsManager().Reconcile(opsRes)
			expectProgressDetailStatus(opsRes, testdbaas.ConsensusComponentName, pod, dbaasv1alpha1.SucceedProgressStatus)
			return opsRes.OpsRequest.Status.Progress
		}, timeout, interval).Should(Equal("1/4"))
	}

	testStatelessPodUpdating := func(opsRes *OpsResource, pod *corev1.Pod) {
		By("create a new pod")
		newPod := testdbaas.MockStatelessPod(testCtx, clusterName, testdbaas.StatelessComponentName, "nginx-"+testCtx.GetRandomStr())
		_, _ = GetOpsManager().Reconcile(opsRes)
		expectProgressDetailStatus(opsRes, testdbaas.StatelessComponentName, newPod, dbaasv1alpha1.ProcessingProgressStatus)
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/4"))

		By("mock new pod is ready")
		lastTransTime := metav1.NewTime(time.Now().Add(-1 * (intctrlutil.DefaultMinReadySeconds + 1) * time.Second))
		patch := client.MergeFrom(newPod.DeepCopy())
		testk8s.MockPodAvailable(newPod, lastTransTime)
		Expect(k8sClient.Status().Patch(ctx, newPod, patch)).Should(Succeed())
		_, _ = GetOpsManager().Reconcile(opsRes)
		expectProgressDetailStatus(opsRes, testdbaas.StatelessComponentName, newPod, dbaasv1alpha1.SucceedProgressStatus)
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/4"))
	}

	testRestart := func(opsRes *OpsResource, consensusPodList []*corev1.Pod, statelessPod *corev1.Pod) {
		By("Test Restart")
		ops := testdbaas.GenerateOpsRequestObj("restart-ops-"+randomStr, clusterName, dbaasv1alpha1.RestartType)
		ops.Spec.RestartList = []dbaasv1alpha1.ComponentOps{
			{ComponentName: testdbaas.ConsensusComponentName},
			{ComponentName: testdbaas.StatelessComponentName},
		}
		testdbaas.CreateOpsRequest(testCtx, ops)

		By("test restart OpsRequest is Running")
		initClusterForOps(opsRes)
		opsRes.OpsRequest = ops
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Eventually(testdbaas.ExpectOpsRequestPhase(testCtx, ops.Name, dbaasv1alpha1.RunningPhase)).Should(BeTrue())

		By("test restart action and reconcile function")
		testdbaas.MockConsensusComponentStatefulSet(testCtx, clusterName)
		testdbaas.MockStatelessComponentDeploy(testCtx, clusterName)
		rHandler := restartOpsHandler{}
		_ = rHandler.Action(opsRes)
		_, _, err := rHandler.ReconcileAction(opsRes)
		Expect(err == nil).Should(BeTrue())

		if !testCtx.UsingExistingCluster() {
			By("mock testing the updates of consensus component")
			testConsensusSetPodUpdating(opsRes, consensusPodList)

			By("mock testing the updates of stateless component")
			testStatelessPodUpdating(opsRes, statelessPod)
		}
	}

	testUpgrade := func(opsRes *OpsResource, clusterObject *dbaasv1alpha1.Cluster) {
		By("Test Upgrade Ops")
		newClusterVersionName := "clusterversion-upgrade-" + randomStr
		_ = testdbaas.CreateHybridCompsClusterVersionForUpgrade(testCtx, clusterDefinitionName, newClusterVersionName)
		ops := testdbaas.GenerateOpsRequestObj("upgrade-ops-"+randomStr, clusterObject.Name, dbaasv1alpha1.UpgradeType)
		ops.Spec.Upgrade = &dbaasv1alpha1.Upgrade{ClusterVersionRef: newClusterVersionName}
		testdbaas.CreateOpsRequest(testCtx, ops)
		opsRes.OpsRequest = ops

		By("test upgrade OpsRequest phase is Running")
		Expect(GetOpsManager().Do(opsRes)).Should(Succeed())
		Expect(ops.Status.Phase == dbaasv1alpha1.RunningPhase).Should(BeTrue())

		By("Test OpsManager.MainEnter function ")
		_, _ = GetOpsManager().Reconcile(opsRes)
	}

	createHorizontalScaling := func(replicas int) *dbaasv1alpha1.OpsRequest {
		horizontalOpsName := "horizontalscaling-ops-" + testCtx.GetRandomStr()
		ops := testdbaas.GenerateOpsRequestObj(horizontalOpsName, clusterName, dbaasv1alpha1.HorizontalScalingType)
		ops.Spec.HorizontalScalingList = []dbaasv1alpha1.HorizontalScaling{
			{
				ComponentOps: dbaasv1alpha1.ComponentOps{ComponentName: testdbaas.ConsensusComponentName},
				Replicas:     int32(replicas),
			},
		}
		testdbaas.CreateOpsRequest(testCtx, ops)
		return ops
	}

	testHorizontalScaling := func(opsRes *OpsResource, podList []*corev1.Pod) {
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
			testk8s.MockPodIsTerminating(testCtx, podList[0])
			_, _ = GetOpsManager().Reconcile(opsRes)
			expectProgressDetailStatus(opsRes, testdbaas.ConsensusComponentName, podList[0], dbaasv1alpha1.ProcessingProgressStatus)

			By("mock the pod is deleted and progressDetail status should be succeed")
			testk8s.RemovePodFinalizer(testCtx, podList[0])
			_, _ = GetOpsManager().Reconcile(opsRes)
			expectProgressDetailStatus(opsRes, testdbaas.ConsensusComponentName, podList[0], dbaasv1alpha1.SucceedProgressStatus)
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
			testdbaas.ConsensusComponentName: {
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
			podName := fmt.Sprintf("%s-%s-%d", clusterName, testdbaas.ConsensusComponentName, 0)
			pod := testdbaas.MockConsensusComponentStsPod(testCtx, clusterName, podName, "leader", "ReadWrite")
			_, _ = GetOpsManager().Reconcile(opsRes)
			expectProgressDetailStatus(opsRes, testdbaas.ConsensusComponentName, pod, dbaasv1alpha1.SucceedProgressStatus)
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
		}
	}

	Context("Test OpsRequest", func() {
		It("Should Test all OpsRequest", func() {
			_, _, clusterObject := testdbaas.InitClusterWithHybridComps(testCtx, clusterDefinitionName, clusterVersionName, clusterName)
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
				testdbaas.ConsensusComponentName: {
					Phase: dbaasv1alpha1.RunningPhase,
					Type:  testdbaas.ConsensusComponentType,
				},
				testdbaas.StatelessComponentName: {
					Phase: dbaasv1alpha1.RunningPhase,
					Type:  testdbaas.StatelessComponentType,
				},
			}
			clusterObject.Status.Operations = &dbaasv1alpha1.Operations{
				Upgradable:       true,
				Restartable:      []string{testdbaas.ConsensusComponentName, testdbaas.StatelessComponentName},
				VerticalScalable: []string{testdbaas.ConsensusComponentName, testdbaas.StatelessComponentName},
				HorizontalScalable: []dbaasv1alpha1.OperationComponent{
					{
						Name: testdbaas.ConsensusComponentName,
					},
					{
						Name: testdbaas.StatelessComponentName,
					},
				},
			}
			opsRes.Cluster = clusterObject
			Expect(k8sClient.Status().Patch(context.Background(), clusterObject, patch)).Should(Succeed())

			var (
				consensusPodList []*corev1.Pod
				statelessPod     *corev1.Pod
			)
			if !testCtx.UsingExistingCluster() {
				// mock the pods of consensusSet component
				consensusPodList = testdbaas.MockConsensusComponentPods(testCtx, clusterName)
				// mock the pods od stateless component
				statelessPod = testdbaas.MockStatelessPod(testCtx, clusterName, testdbaas.StatelessComponentName, "nginx-"+randomStr)
			}

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
