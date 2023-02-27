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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Replication Component", func() {
	var (
		clusterName        = "test-cluster-repl"
		clusterDefName     = "test-cluster-def-repl"
		clusterVersionName = "test-cluster-version-repl"
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
				AddComponent(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testapps.DefaultRedisCompType).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

			By("Creating a cluster with replication workloadType.")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
				SetReplicas(testapps.DefaultReplicationReplicas).
				Create(&testCtx).GetObject()

			By("Creating two statefulSets of replication workloadType.")
			status := appsv1.StatefulSetStatus{
				AvailableReplicas:  1,
				ObservedGeneration: 1,
				Replicas:           1,
				ReadyReplicas:      1,
				UpdatedReplicas:    1,
				CurrentRevision:    "mock-revision",
				UpdateRevision:     "mock-revision",
			}

			var (
				primarySts   *appsv1.StatefulSet
				secondarySts *appsv1.StatefulSet
			)
			for k, v := range map[string]string{
				string(Primary):   clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-0",
				string(Secondary): clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-1",
			} {
				sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, v, clusterObj.Name, testapps.DefaultRedisCompName).
					AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
					AddAppInstanceLabel(clusterObj.Name).
					AddAppComponentLabel(testapps.DefaultRedisCompName).
					AddAppManangedByLabel().
					AddRoleLabel(k).
					SetReplicas(1).
					Create(&testCtx).GetObject()
				if k == string(Primary) {
					Expect(CheckStsIsPrimary(sts)).Should(BeTrue())
					primarySts = sts
				} else {
					Expect(CheckStsIsPrimary(sts)).ShouldNot(BeTrue())
					secondarySts = sts
				}
			}

			compDefName := clusterObj.GetComponentDefRefName(testapps.DefaultRedisCompName)
			componentDef := clusterDefObj.GetComponentDefByName(compDefName)
			component := clusterObj.GetComponentByName(testapps.DefaultRedisCompName)
			replicationComponent := NewReplicationSet(ctx, k8sClient, clusterObj, component, componentDef)

			for _, availableReplica := range []int32{0, 1} {
				status.AvailableReplicas = availableReplica
				primarySts.Status = status
				testk8s.PatchStatefulSetStatus(&testCtx, primarySts.Name, status)
				secondarySts.Status = status
				testk8s.PatchStatefulSetStatus(&testCtx, secondarySts.Name, status)
				// Create pod of the statefulset
				if availableReplica == 1 {
					testapps.MockReplicationComponentPods(testCtx, primarySts, clusterName, testapps.DefaultRedisCompName, string(Primary))
					testapps.MockReplicationComponentPods(testCtx, secondarySts, clusterName, testapps.DefaultRedisCompName, string(Secondary))
				}

				podsReady, _ := replicationComponent.PodsReady(primarySts)
				isRunning, _ := replicationComponent.IsRunning(primarySts)
				if availableReplica == 1 {
					By("Testing pods are ready")
					Expect(podsReady == true).Should(BeTrue())

					By("Testing component is running")
					Expect(isRunning == true).Should(BeTrue())
				} else {
					By("Testing pods are not ready")
					Expect(podsReady == false).Should(BeTrue())

					By("Testing component is not running")
					Expect(isRunning == false).Should(BeTrue())
				}
			}

			By("Testing handle probe timed out")
			requeue, _ := replicationComponent.HandleProbeTimeoutWhenPodsReady(nil)
			Expect(requeue == false).Should(BeTrue())

			By("Testing component phase when pods not ready")
			phase, _ := replicationComponent.GetPhaseWhenPodsNotReady(testapps.DefaultRedisCompName)
			Expect(phase == appsv1alpha1.FailedPhase).Should(BeTrue())

			By("Checking if the pod is not updated when statefulset is not updated")
			Expect(replicationComponent.HandleUpdate(primarySts)).To(Succeed())
			podList, err := util.GetPodListByStatefulSet(ctx, k8sClient, primarySts)
			Expect(err).To(Succeed())
			Expect(len(podList)).To(Equal(1))
			Expect(util.IsStsAndPodsRevisionConsistent(ctx, k8sClient, primarySts)).Should(BeTrue())

			By("Checking if the pod is deleted when statefulset is updated")
			status.UpdateRevision = "new-mock-revision"
			testk8s.PatchStatefulSetStatus(&testCtx, primarySts.Name, status)
			Expect(replicationComponent.HandleUpdate(primarySts)).To(Succeed())
			podList, err = util.GetPodListByStatefulSet(ctx, k8sClient, primarySts)
			Expect(err).To(Succeed())
			Expect(len(podList)).To(Equal(0))
		})
	})
})
