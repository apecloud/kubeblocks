/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

func TestRollingInstanceProgress(t *testing.T) {
	const (
		component = "mysql"
		pod0      = "cluster-mysql-0"
		pod1      = "cluster-mysql-1"
	)
	baseWorkload := func() *defaultWorkload {
		return &defaultWorkload{
			exists:               true,
			desiredReplicas:      2,
			currentRevisionMap:   map[string]string{pod0: "new", pod1: "new"},
			upToDateSet:          sets.New(pod0, pod1),
			notReadySet:          sets.New[string](),
			notAvailableSet:      sets.New[string](),
			failedSet:            sets.New[string](),
			instanceNames:        sets.New(pod0, pod1),
			activeInstanceNames:  sets.New(pod0, pod1),
			presentInstanceNames: sets.New(pod0, pod1),
		}
	}
	newResources := func() (*OpsResource, *progressResource, *opsv1alpha1.OpsRequestComponentStatus) {
		ops := &opsv1alpha1.OpsRequest{}
		ops.Status.Phase = opsv1alpha1.OpsRunningPhase
		return &OpsResource{
				OpsRequest: ops,
				Cluster: &appsv1.Cluster{Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{
					component: {},
				}}},
				Recorder: record.NewFakeRecorder(20),
			}, &progressResource{
				opsMessageKey:     "upgrade",
				clusterComponent:  &appsv1.ClusterComponentSpec{Name: component, Replicas: 2},
				fullComponentName: component,
			}, &opsv1alpha1.OpsRequestComponentStatus{}
	}

	tests := []struct {
		name          string
		mutate        func(*defaultWorkload, *progressResource)
		wantExpected  int32
		wantCompleted int32
		wantPod0      opsv1alpha1.ProgressStatus
	}{
		{name: "all instances up-to-date ready and available", wantExpected: 2, wantCompleted: 2, wantPod0: opsv1alpha1.SucceedProgressStatus},
		{name: "desired state not applied", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.upToDateSet.Delete(pod0)
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "failure before desired state is applied is ignored", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.upToDateSet.Delete(pod0)
			w.failedSet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "partial vertical scaling waits when resources are not applied", mutate: func(w *defaultWorkload, pg *progressResource) {
			w.upToDateSet.Delete(pod0)
			pg.updatedPodSet = map[string]string{pod0: "template"}
		}, wantExpected: 1, wantCompleted: 0, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "not ready", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.notReadySet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "not available", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.notAvailableSet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "current desired state failure", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.failedSet.Insert(pod0)
		}, wantExpected: 2, wantCompleted: 2, wantPod0: opsv1alpha1.FailedProgressStatus},
		{name: "active instance is absent", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.presentInstanceNames.Delete(pod0)
		}, wantExpected: 2, wantCompleted: 1, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "offline instance is excluded from full rollout progress", mutate: func(w *defaultWorkload, pg *progressResource) {
			w.activeInstanceNames.Delete(pod0)
			w.desiredReplicas = 1
			pg.clusterComponent.Replicas = 1
		}, wantExpected: 1, wantCompleted: 1},
		{name: "partial target cannot complete after becoming inactive", mutate: func(w *defaultWorkload, pg *progressResource) {
			w.activeInstanceNames.Delete(pod0)
			pg.updatedPodSet = map[string]string{pod0: "template"}
		}, wantExpected: 1, wantCompleted: 0, wantPod0: opsv1alpha1.ProcessingProgressStatus},
		{name: "missing InstanceSet", mutate: func(w *defaultWorkload, _ *progressResource) {
			w.exists = false
		}, wantExpected: 2, wantCompleted: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workload := baseWorkload()
			opsRes, pgRes, compStatus := newResources()
			if tt.mutate != nil {
				tt.mutate(workload, pgRes)
			}
			expected, completed := handleRollingProgressWithWorkload(opsRes, workload, pgRes, compStatus)
			if expected != tt.wantExpected || completed != tt.wantCompleted {
				t.Fatalf("progress %d/%d, want %d/%d", completed, expected, tt.wantCompleted, tt.wantExpected)
			}
			if tt.wantPod0 != "" {
				detail := findStatusProgressDetail(compStatus.ProgressDetails, getProgressObjectKey(constant.PodKind, pod0))
				if detail == nil || detail.Status != tt.wantPod0 {
					t.Fatalf("pod0 detail = %#v, want status %s", detail, tt.wantPod0)
				}
			}
		})
	}
}

var _ = Describe("Ops ProgressDetails", func() {

	var (
		randomStr   = testCtx.GetRandomStr()
		compDefName = "test-compdef-" + randomStr
		clusterName = "test-cluster-" + randomStr
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		// default GracePeriod is 30s
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initClusterForOps := func(opsRes *OpsResource) {
		Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		opsRes.Cluster.Status.Phase = appsv1.RunningClusterPhase
	}

	testProgressDetailsWithStatefulPodUpdating := func(reqCtx intctrlutil.RequestCtx, opsRes *OpsResource, pods []*corev1.Pod) {
		By("mock pod of InstanceSet updating by deleting the pod")
		pod := pods[0]
		testk8s.MockPodIsTerminating(ctx, testCtx, pod)
		mockRollingInstanceStatus(opsRes.Cluster, defaultCompName)
		_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(getProgressDetailStatus(opsRes, defaultCompName, pod)).Should(Equal(opsv1alpha1.ProcessingProgressStatus))

		By("mock one pod of InstanceSet to update successfully")
		testk8s.RemovePodFinalizer(ctx, testCtx, pod)
		testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName,
			pod.Name, "leader")
		mockRollingInstanceStatus(opsRes.Cluster, defaultCompName, pod.Name)

		_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
		Expect(getProgressDetailStatus(opsRes, defaultCompName, pod)).Should(Equal(opsv1alpha1.SucceedProgressStatus))
		Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/3"))
	}

	Context("Test Ops ProgressDetails", func() {
		It("Test Ops ProgressDetails for rolling update", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)

			By("create restart ops and pods of component")
			opsRes.OpsRequest = createRestartOpsObj(clusterName, "restart-"+randomStr)
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName)
			podList := initInstanceSetPods(ctx, k8sClient, opsRes)

			By("mock restart OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("test the progressDetails when stateful pod updates during restart operation")
			testProgressDetailsWithStatefulPodUpdating(reqCtx, opsRes, podList)
		})

		It("Test Ops ProgressDetails with scale-in replicas", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			podList := testapps.MockInstanceSetPods(&testCtx, its, opsRes.Cluster, defaultCompName)

			By("create horizontalScaling operation to test the progressDetails when scaling in the replicas")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ScaleIn: &opsv1alpha1.ScaleIn{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{
						ReplicaChanges: pointer.Int32(2),
					},
				},
			}, false)
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName) // appsv1.HorizontalScalingPhase
			initClusterForOps(opsRes)

			By("mock HorizontalScaling OpsRequest phase is running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			// do h-scale action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("mock the pod is terminating, pod[1] is target pod to delete. and mock pod[2] is failed and deleted by stateful controller")
			for i := 1; i < 3; i++ {
				pod := podList[i]
				testk8s.MockPodIsTerminating(ctx, testCtx, pod)
				testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
				_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
				Expect(getProgressDetailStatus(opsRes, defaultCompName, pod)).Should(Equal(opsv1alpha1.ProcessingProgressStatus))

			}
			By("mock the target pod is deleted and progressDetail status should be succeed")
			targetPod := podList[1]
			testk8s.RemovePodFinalizer(ctx, testCtx, targetPod)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(getProgressDetailStatus(opsRes, defaultCompName, targetPod)).Should(Equal(opsv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/2"))

			By("delete the pod[2]")
			pod := podList[2]
			testk8s.RemovePodFinalizer(ctx, testCtx, pod)
			// expect the progress is 2/2
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(getProgressDetailStatus(opsRes, defaultCompName, targetPod)).Should(Equal(opsv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("2/2"))
		})

		It("Test Ops ProgressDetails with scale-out replicas", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)
			podList := testapps.MockInstanceSetPods(&testCtx, its, opsRes.Cluster, defaultCompName)

			// ops will use the startTimestamp to make decision, start time should not equal the pod createTime during testing.
			time.Sleep(time.Second)

			By("create horizontalScaling operation to test the progressDetails when scaling out the replicas ")
			opsRes.OpsRequest = createHorizontalScaling(clusterName, opsv1alpha1.HorizontalScaling{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				ScaleOut: &opsv1alpha1.ScaleOut{
					ReplicaChanger: opsv1alpha1.ReplicaChanger{
						ReplicaChanges: pointer.Int32(1),
					},
				},
			}, false)
			mockComponentIsOperating(opsRes.Cluster, appsv1.UpdatingComponentPhase, defaultCompName) // appsv1.HorizontalScalingPhase
			initClusterForOps(opsRes)

			By("mock HorizontalScaling OpsRequest phase is running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			// do h-scale action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("test the progressDetails when scaling out replicas")
			tokens := strings.Split(podList[2].Name, "-")
			targetPodName := fmt.Sprintf("%s-3", strings.Join(tokens[0:len(tokens)-1], "-"))
			testapps.MockInstanceSetPod(&testCtx, nil, clusterName, defaultCompName,
				targetPodName, "follower")
			targetPod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: targetPodName, Namespace: testCtx.DefaultNamespace}, targetPod)).Should(Succeed())
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)
			_, _ = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(getProgressDetailStatus(opsRes, defaultCompName, targetPod)).Should(Equal(opsv1alpha1.SucceedProgressStatus))
			Expect(opsRes.OpsRequest.Status.Progress).Should(Equal("1/1"))
		})
	})
})

func getProgressDetailStatus(opsRes *OpsResource, componentName string, pod *corev1.Pod) opsv1alpha1.ProgressStatus {
	objectKey := getProgressObjectKey(constant.PodKind, pod.Name)
	progressDetails := opsRes.OpsRequest.Status.Components[componentName].ProgressDetails
	progressDetail := findStatusProgressDetail(progressDetails, objectKey)
	var status opsv1alpha1.ProgressStatus
	if progressDetail != nil {
		status = progressDetail.Status
	}
	return status
}
