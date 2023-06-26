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

package replication

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
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

	Context("Replication Component test", func() {
		It("Replication Component test", func() {

			By("Create a clusterDefinition obj with replication workloadType.")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponentVersion(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

			By("Creating a cluster with replication workloadType.")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testapps.DefaultRedisCompSpecName, testapps.DefaultRedisCompDefName).
				SetReplicas(testapps.DefaultReplicationReplicas).
				Create(&testCtx).GetObject()

			// mock cluster is Running
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
				clusterObj.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
					testapps.DefaultRedisCompSpecName: {
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
				clusterObj.Name+"-"+testapps.DefaultRedisCompSpecName, clusterObj.Name, testapps.DefaultRedisCompSpecName).
				AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
				AddAppInstanceLabel(clusterObj.Name).
				AddAppComponentLabel(testapps.DefaultRedisCompSpecName).
				AddAppManangedByLabel().
				SetReplicas(replicas).
				Create(&testCtx).GetObject()
			stsObjectKey := client.ObjectKey{Name: replicationSetSts.Name, Namespace: testCtx.DefaultNamespace}

			Expect(replicationSetSts.Spec.VolumeClaimTemplates).Should(BeEmpty())

			compDefName := clusterObj.Spec.GetComponentDefRefName(testapps.DefaultRedisCompSpecName)
			componentDef := clusterDefObj.GetComponentDefByName(compDefName)
			componentSpec := clusterObj.Spec.GetComponentByName(testapps.DefaultRedisCompSpecName)
			replicationComponent := newReplicationSet(k8sClient, clusterObj, componentSpec, *componentDef)
			var podList []*corev1.Pod

			for _, availableReplica := range []int32{0, replicas} {
				status.AvailableReplicas = availableReplica
				replicationSetSts.Status = status
				testk8s.PatchStatefulSetStatus(&testCtx, replicationSetSts.Name, status)

				if availableReplica > 0 {
					// Create pods of the statefulset
					stsPods := testapps.MockReplicationComponentPods(nil, testCtx, replicationSetSts, clusterObj.Name,
						testapps.DefaultRedisCompSpecName, map[int32]string{
							0: constant.Primary,
							1: constant.Secondary,
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

			// TODO(refactor): probe timed-out pod
			// By("Testing handle probe timed out")
			// requeue, _ := replicationComponent.HandleProbeTimeoutWhenPodsReady(ctx, nil)
			// Expect(requeue == false).Should(BeTrue())

			By("Testing pod is available")
			primaryPod := podList[0]
			Expect(replicationComponent.PodIsAvailable(primaryPod, 10)).Should(BeTrue())

			By("should return empty string if pod of component is only not ready when component is not up running")
			pod := podList[1]
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.Conditions = []corev1.PodCondition{}
			})).Should(Succeed())
			status.AvailableReplicas -= 1
			testk8s.PatchStatefulSetStatus(&testCtx, replicationSetSts.Name, status)
			phase, _, _ := replicationComponent.GetPhaseWhenPodsNotReady(ctx, testapps.DefaultRedisCompSpecName, false)
			Expect(string(phase)).Should(Equal(""))

			By("expect component phase is Abnormal when pod of component is not ready and component is up running")
			phase, _, _ = replicationComponent.GetPhaseWhenPodsNotReady(ctx, testapps.DefaultRedisCompSpecName, true)
			Expect(phase).Should(Equal(appsv1alpha1.AbnormalClusterCompPhase))

			// mock pod label is empty
			Expect(testapps.ChangeObj(&testCtx, primaryPod, func(pod *corev1.Pod) {
				pod.Labels[constant.RoleLabelKey] = ""
			})).Should(Succeed())
			_, statusMessages, _ := replicationComponent.GetPhaseWhenPodsNotReady(ctx, testapps.DefaultRedisCompSpecName, false)
			Expect(statusMessages[fmt.Sprintf("%s/%s", primaryPod.Kind, primaryPod.Name)]).
				Should(ContainSubstring("empty label for pod, please check"))

			// mock primary pod failed
			testk8s.UpdatePodStatusScheduleFailed(ctx, testCtx, primaryPod.Name, primaryPod.Namespace)
			phase, _, _ = replicationComponent.GetPhaseWhenPodsNotReady(ctx, testapps.DefaultRedisCompSpecName, true)
			Expect(phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))

			By("Checking if the pod is not updated when statefulSet is not updated")
			Expect(testCtx.Cli.Get(testCtx.Ctx, stsObjectKey, replicationSetSts)).Should(Succeed())
			vertexes, err := replicationComponent.HandleRestart(ctx, replicationSetSts)
			Expect(err).To(Succeed())
			Expect(len(vertexes)).To(Equal(0))
			pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, replicationSetSts)
			Expect(err).To(Succeed())
			Expect(len(pods)).To(Equal(int(replicas)))
			Expect(util.IsStsAndPodsRevisionConsistent(ctx, k8sClient, replicationSetSts)).Should(BeTrue())

			By("Checking if the pod is deleted when statefulSet is updated")
			status.UpdateRevision = "new-mock-revision"
			testk8s.PatchStatefulSetStatus(&testCtx, replicationSetSts.Name, status)
			Expect(testCtx.Cli.Get(testCtx.Ctx, stsObjectKey, replicationSetSts)).Should(Succeed())
			vertexes, err = replicationComponent.HandleRestart(ctx, replicationSetSts)
			Expect(err).To(Succeed())
			Expect(len(vertexes)).To(Equal(int(replicas)))
			Expect(*vertexes[0].(*ictrltypes.LifecycleVertex).Action == ictrltypes.DELETE).To(BeTrue())

			By("Test handleRoleChange when statefulSet Pod with role label but without primary annotation")
			Expect(testapps.ChangeObj(&testCtx, primaryPod, func(pod *corev1.Pod) {
				pod.Labels[constant.RoleLabelKey] = constant.Primary
			})).Should(Succeed())
			status.UpdateRevision = "new-mock-revision-for-role-change"
			testk8s.PatchStatefulSetStatus(&testCtx, replicationSetSts.Name, status)
			Expect(testCtx.Cli.Get(testCtx.Ctx, stsObjectKey, replicationSetSts)).Should(Succeed())
			vertexes, err = replicationComponent.HandleRoleChange(ctx, replicationSetSts)
			Expect(err).To(Succeed())
			Expect(len(vertexes)).To(Equal(int(replicas)))
			Expect(*vertexes[0].(*ictrltypes.LifecycleVertex).Action == ictrltypes.UPDATE).To(BeTrue())

			By("Test handleRoleChange when statefulSet h-scale out a new Pod with no role label")
			status.Replicas = 3
			status.AvailableReplicas = 3
			status.ReadyReplicas = 3
			testk8s.PatchStatefulSetStatus(&testCtx, replicationSetSts.Name, status)
			Expect(testCtx.Cli.Get(testCtx.Ctx, stsObjectKey, replicationSetSts)).Should(Succeed())
			newPodName := fmt.Sprintf("%s-%d", replicationSetSts.Name, 2)
			_ = testapps.MockReplicationComponentPod(nil, testCtx, replicationSetSts, clusterObj.Name, testapps.DefaultRedisCompSpecName, newPodName, "")
			vertexes, err = replicationComponent.HandleRoleChange(ctx, replicationSetSts)
			Expect(err).To(Succeed())
			Expect(len(vertexes)).To(Equal(3))
			Expect(*vertexes[0].(*ictrltypes.LifecycleVertex).Action == ictrltypes.UPDATE).To(BeTrue())
		})
	})
})
