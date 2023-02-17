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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Replication Component", func() {
	var (
		clusterName        = "test-cluster-repl"
		clusterDefName     = "test-cluster-def-repl"
		clusterVersionName = "test-cluster-version-repl"
	)

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testdbaas.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("Replication Component test", func() {
		It("Replication Component test", func() {

			By("Create a clusterDefinition obj with replication componentType.")
			clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.RedisType).
				AddComponent(testdbaas.ReplicationRedisComponent, testdbaas.DefaultRedisCompType).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication componentType.")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testdbaas.DefaultRedisCompType).AddContainerShort(testdbaas.DefaultRedisContainerName, testdbaas.DefaultRedisImageName).
				Create(&testCtx).GetObject()

			By("Creating a cluster with replication componentType.")
			clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(testdbaas.DefaultRedisCompName, testdbaas.DefaultRedisCompType).
				SetReplicas(testdbaas.DefaultReplicationReplicas).
				Create(&testCtx).GetObject()

			By("Creating a statefulSet of replication componentType.")
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
				string(Primary):   clusterObj.Name + "-" + testdbaas.DefaultRedisCompName + "-0",
				string(Secondary): clusterObj.Name + "-" + testdbaas.DefaultRedisCompName + "-1",
			} {
				sts := testdbaas.NewStatefulSetFactory(testCtx.DefaultNamespace, v, clusterObj.Name, testdbaas.DefaultRedisCompName).
					AddContainer(corev1.Container{Name: testdbaas.DefaultRedisContainerName, Image: testdbaas.DefaultRedisImageName}).
					AddLabels(intctrlutil.AppInstanceLabelKey, clusterObj.Name,
						intctrlutil.AppComponentLabelKey, testdbaas.DefaultRedisCompName,
						intctrlutil.AppManagedByLabelKey, testdbaas.KubeBlocks,
						intctrlutil.RoleLabelKey, k).
					SetReplicas(1).
					Create(&testCtx).GetObject()
				testdbaas.MockReplicationComponentPods(testCtx, sts, clusterName, testdbaas.DefaultRedisCompName, k)
				if k == string(Primary) {
					Expect(CheckStsIsPrimary(sts)).Should(BeTrue())
					primarySts = sts
				} else {
					Expect(CheckStsIsPrimary(sts)).ShouldNot(BeTrue())
					secondarySts = sts
				}
			}

			typeName := clusterObj.GetComponentTypeName(testdbaas.DefaultRedisCompName)
			componentDef := clusterDefObj.GetComponentDefByTypeName(typeName)
			component := clusterObj.GetComponentByName(testdbaas.DefaultRedisCompName)
			replicationComponent := NewReplicationSet(ctx, k8sClient, clusterObj, component, componentDef)

			for _, availableReplica := range []int32{0, 1} {
				status.AvailableReplicas = availableReplica
				testk8s.PatchStatefulSetStatus(&testCtx, primarySts.Name, status)
				testk8s.PatchStatefulSetStatus(&testCtx, secondarySts.Name, status)
				podsReady, _ := replicationComponent.PodsReady(primarySts)
				isRunning, _ := replicationComponent.IsRunning(primarySts)
				if availableReplica == 1 {
					By("test pods are ready")
					Expect(podsReady == true).Should(BeTrue())

					By("test component is running")
					Expect(isRunning == true).Should(BeTrue())
				} else {
					By("test pods are not ready")
					Expect(podsReady == false).Should(BeTrue())

					By("test component is not running")
					Expect(isRunning == false).Should(BeTrue())
				}
			}

			By("test handle probe timed out")
			requeue, _ := replicationComponent.HandleProbeTimeoutWhenPodsReady(nil)
			Expect(requeue == false).Should(BeTrue())

			By("test component phase when pods not ready")
			phase, _ := replicationComponent.GetPhaseWhenPodsNotReady(testdbaas.DefaultRedisCompName)
			Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
		})
	})
})
