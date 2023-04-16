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

package replicationset

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Replication Component", func() {
	var (
		clusterName        = "test-cluster-repl"
		clusterDefName     = "test-cluster-def-repl"
		clusterVersionName = "test-cluster-version-repl"
		controllerRivision = "mock-revision"
	)

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		clusterObj        *appsv1alpha1.Cluster
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

	Context("Replication Component test", func() {
		It("Replication Component test", func() {

			By("Create a clusterDefinition obj with replication workloadType.")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

			By("Creating a cluster with replication workloadType.")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompDefName).
				SetReplicas(testapps.DefaultReplicationReplicas).
				Create(&testCtx).GetObject()

			// mock cluster is Running
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
				clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
					testapps.DefaultRedisCompName: {
						Phase: appsv1alpha1.RunningClusterCompPhase,
					},
				}
			})).Should(Succeed())

			By("Creating statefulSet of replication workloadType.")
			replicas := int32(2)
			status := appsv1.StatefulSetStatus{
				AvailableReplicas:  replicas,
				ObservedGeneration: 1,
				Replicas:           replicas,
				ReadyReplicas:      replicas,
				UpdatedReplicas:    replicas,
				CurrentRevision:    controllerRivision,
				UpdateRevision:     controllerRivision,
			}

			replicationSetSts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
				clusterObj.Name+"-"+testapps.DefaultRedisCompName, clusterObj.Name, testapps.DefaultRedisCompName).
				AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
				AddAppInstanceLabel(clusterObj.Name).
				AddAppComponentLabel(testapps.DefaultRedisCompName).
				AddAppManangedByLabel().
				SetReplicas(replicas).
				Create(&testCtx).GetObject()

			Expect(replicationSetSts.Spec.VolumeClaimTemplates).Should(BeEmpty())

			compDefName := clusterObj.Spec.GetComponentDefRefName(testapps.DefaultRedisCompName)
			componentDef := clusterDefObj.GetComponentDefByName(compDefName)
			component := clusterObj.Spec.GetComponentByName(testapps.DefaultRedisCompName)
			replicationComponent, err := NewReplicationComponent(k8sClient, clusterObj, component, *componentDef)
			Expect(err).Should(Succeed())
			var podList []*corev1.Pod

			for _, availableReplica := range []int32{0, replicas} {
				status.AvailableReplicas = availableReplica
				replicationSetSts.Status = status
				testk8s.PatchStatefulSetStatus(&testCtx, replicationSetSts.Name, status)

				if availableReplica > 0 {
					// Create pods of the statefulset
					stsPods := testapps.MockReplicationComponentPods(nil, testCtx, replicationSetSts, clusterObj.Name,
						testapps.DefaultRedisCompName, map[int32]string{
							0: string(Primary),
							1: string(Secondary),
						})
					podList = append(podList, stsPods...)
					By("Testing pods are ready")
					podsReady, _ := replicationComponent.PodsReady(ctx, replicationSetSts)
					Expect(podsReady).Should(BeTrue())
					By("Testing component is running")
					isRunning, _ := replicationComponent.IsRunning(ctx, replicationSetSts)
					Expect(isRunning).Should(BeTrue())
				} else {
					podsReady, _ := replicationComponent.PodsReady(ctx, replicationSetSts)
					By("Testing pods are not ready")
					Expect(podsReady).Should(BeFalse())
					By("Testing component is not running")
					isRunning, _ := replicationComponent.IsRunning(ctx, replicationSetSts)
					Expect(isRunning).Should(BeFalse())
				}
			}

			By("Testing handle probe timed out")
			requeue, _ := replicationComponent.HandleProbeTimeoutWhenPodsReady(ctx, nil)
			Expect(requeue == false).Should(BeTrue())

			By("Testing pod is available")
			primaryPod := podList[0]
			Expect(replicationComponent.PodIsAvailable(primaryPod, 10)).Should(BeTrue())

			By("Testing component phase when pods not ready")
			// mock secondary pod is not ready.
			testk8s.UpdatePodStatusNotReady(ctx, testCtx, podList[1].Name)
			status.AvailableReplicas -= 1
			testk8s.PatchStatefulSetStatus(&testCtx, replicationSetSts.Name, status)
			phase, _ := replicationComponent.GetPhaseWhenPodsNotReady(ctx, testapps.DefaultRedisCompName)
			Expect(phase).Should(Equal(appsv1alpha1.AbnormalClusterCompPhase))

			// mock primary pod is not ready
			testk8s.UpdatePodStatusNotReady(ctx, testCtx, primaryPod.Name)
			phase, _ = replicationComponent.GetPhaseWhenPodsNotReady(ctx, testapps.DefaultRedisCompName)
			Expect(phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))

			// mock pod label is empty
			Expect(testapps.ChangeObj(&testCtx, primaryPod, func(lpod *corev1.Pod) {
				lpod.Labels[constant.RoleLabelKey] = ""
			})).Should(Succeed())
			_, _ = replicationComponent.GetPhaseWhenPodsNotReady(ctx, testapps.DefaultRedisCompName)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterObj),
				func(g Gomega, cluster *appsv1alpha1.Cluster) {
					compStatus := cluster.Status.Components[testapps.DefaultRedisCompName]
					g.Expect(compStatus.GetObjectMessage(primaryPod.Kind, primaryPod.Name)).
						Should(ContainSubstring("empty label for pod, please check"))
				})).Should(Succeed())

			By("Checking if the pod is not updated when statefulset is not updated")
			Expect(replicationComponent.HandleUpdate(ctx, replicationSetSts)).To(Succeed())
			Expect(err).To(Succeed())
			Expect(util.IsStsAndPodsRevisionConsistent(ctx, k8sClient, replicationSetSts)).Should(BeTrue())
		})
	})
})
