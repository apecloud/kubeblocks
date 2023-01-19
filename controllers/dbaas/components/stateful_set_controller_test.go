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

package components

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("StatefulSet Controller", func() {

	var (
		randomStr          = testCtx.GetRandomStr()
		timeout            = time.Second * 10
		interval           = time.Second
		clusterName        = "mysql-" + randomStr
		clusterDefName     = "cluster-definition-consensus-" + randomStr
		clusterVersionName = "cluster-version-operations-" + randomStr
		opsRequestName     = "wesql-restart-test-" + randomStr
		revisionID         = "6fdd48d9cd"
		consensusCompName  = "consensus"
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.OpsRequest{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey},
			client.GracePeriodSeconds(0))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	patchPodLabel := func(podName, podRole, accessMode, revision string) {
		pod := &corev1.Pod{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: podName, Namespace: testCtx.DefaultNamespace}, pod)
			return err == nil
		}, timeout, interval).Should(BeTrue())
		patch := client.MergeFrom(pod.DeepCopy())
		pod.Labels[intctrlutil.ConsensusSetRoleLabelKey] = podRole
		pod.Labels[intctrlutil.ConsensusSetAccessModeLabelKey] = accessMode
		pod.Labels[appsv1.ControllerRevisionHashLabelKey] = revision
		Expect(k8sClient.Status().Patch(context.Background(), pod, patch)).Should(Succeed())
	}

	testUpdateStrategy := func(updateStrategy dbaasv1alpha1.UpdateStrategy, componentName string, index int) {
		clusterDef := &dbaasv1alpha1.ClusterDefinition{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDefName}, clusterDef)).Should(Succeed())
		clusterDef.Spec.Components[0].ConsensusSpec.UpdateStrategy = dbaasv1alpha1.SerialStrategy
		Expect(k8sClient.Update(context.Background(), clusterDef)).Should(Succeed())

		newSts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-" + componentName,
			Namespace: testCtx.DefaultNamespace}, newSts)).Should(Succeed())
		stsPatch := client.MergeFrom(newSts.DeepCopy())
		// mock consensus component is not ready
		newSts.Status.UpdateRevision = fmt.Sprintf("%s-%s-%dfdd48d8cd", clusterName, componentName, index)
		newSts.Status.ObservedGeneration = newSts.Generation - 1
		Expect(k8sClient.Status().Patch(context.Background(), newSts, stsPatch)).Should(Succeed())

		By("waiting the component is Updating")
		Eventually(testdbaas.GetClusterComponentPhase(testCtx, clusterName, componentName),
			timeout, interval).Should(Equal(dbaasv1alpha1.UpdatingPhase))
	}

	testUsingEnvTest := func(sts *appsv1.StatefulSet) {
		By("create pod of statefulset")
		_ = testdbaas.MockConsensusComponentPods(ctx, testCtx, clusterName, consensusCompName)

		By("mock restart cluster")
		sts.Spec.Template.Annotations = map[string]string{
			"kubeblocks.io/restart": time.Now().Format(time.RFC3339),
		}
		Expect(k8sClient.Update(context.Background(), sts)).Should(Succeed())

		By("mock statefulset is ready")
		newSts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: sts.Name, Namespace: testCtx.DefaultNamespace}, newSts)).Should(Succeed())
		stsPatch := client.MergeFrom(newSts.DeepCopy())
		updateRevision := fmt.Sprintf("%s-%s-%s", clusterName, consensusCompName, revisionID)
		newSts.Status.UpdateRevision = updateRevision
		testk8s.MockStatefulSetReady(newSts)
		Expect(k8sClient.Status().Patch(context.Background(), newSts, stsPatch)).Should(Succeed())
		Eventually(func(g Gomega) string {
			g.Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: sts.Name, Namespace: testCtx.DefaultNamespace}, newSts)).Should(Succeed())
			return newSts.Status.UpdateRevision
		}, timeout, interval).Should(Equal(updateRevision))
	}

	testUsingRealCluster := func() {
		newSts := &appsv1.StatefulSet{}
		// wait for StatefulSet to create all pods
		Eventually(func() bool {
			_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName + "-" + consensusCompName,
				Namespace: testCtx.DefaultNamespace}, newSts)
			return newSts.Status.ObservedGeneration == 1
		}, timeout, interval).Should(BeTrue())
		By("patch pod label of StatefulSet")
		for i := 0; i < 3; i++ {
			podName := fmt.Sprintf("%s-%s-%d", clusterName, consensusCompName, i)
			podRole := "follower"
			accessMode := "Readonly"
			if i == 0 {
				podRole = "leader"
				accessMode = "ReadWrite"
			}
			// patch pod label to reach the conditions, then component status will change to Running
			patchPodLabel(podName, podRole, accessMode, newSts.Status.UpdateRevision)
		}
	}

	Context("test controller", func() {
		It("test statefulSet controller", func() {
			_, _, cluster := testdbaas.InitConsensusMysql(ctx, testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompName)
			By("patch cluster to Updating")
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Phase = dbaasv1alpha1.UpdatingPhase
			cluster.Status.ObservedGeneration = cluster.Generation
			cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				consensusCompName: {
					Phase: dbaasv1alpha1.RunningPhase,
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), cluster, patch)).Should(Succeed())
			// waite for patching cluster successfully
			Eventually(testdbaas.GetClusterComponentPhase(testCtx, clusterName, consensusCompName), timeout, interval).Should(Equal(dbaasv1alpha1.RunningPhase))

			_ = testdbaas.CreateRestartOpsRequest(ctx, testCtx, clusterName, opsRequestName, []string{consensusCompName})
			sts := testdbaas.MockConsensusComponentStatefulSet(ctx, testCtx, clusterName, consensusCompName)
			clusterPatch := client.MergeFrom(cluster.DeepCopy())
			cluster.Annotations = map[string]string{
				intctrlutil.OpsRequestAnnotationKey: fmt.Sprintf(`[{"name":"%s","clusterPhase":"Updating"}]`, opsRequestName),
			}
			Expect(k8sClient.Patch(ctx, cluster, clusterPatch)).Should(Succeed())

			By("waiting the component is Updating by statefulSet controller")
			Eventually(testdbaas.GetClusterComponentPhase(testCtx, clusterName, consensusCompName), timeout, interval).Should(Equal(dbaasv1alpha1.UpdatingPhase))

			By("mock the StatefulSet and pods are ready")
			if testCtx.UsingExistingCluster() {
				testUsingRealCluster()
			} else {
				// mock statefulSet available and consensusSet component is running
				testUsingEnvTest(sts)
			}

			By("waiting the component is Running")
			Eventually(testdbaas.GetClusterComponentPhase(testCtx, clusterName, consensusCompName), 2*timeout, interval).Should(Equal(dbaasv1alpha1.RunningPhase))

			By("test updateStrategy with Serial")
			testUpdateStrategy(dbaasv1alpha1.SerialStrategy, consensusCompName, 1)

			By("test updateStrategy with Parallel")
			testUpdateStrategy(dbaasv1alpha1.ParallelStrategy, consensusCompName, 2)
		})
	})
})
