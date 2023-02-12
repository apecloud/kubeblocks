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
	"k8s.io/apimachinery/pkg/types"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Replication Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterName        = "cluster-replication" + randomStr
		clusterDefName     = "cluster-def-replication-" + randomStr
		clusterVersionName = "cluster-version-replication-" + randomStr
	)

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

	const redisImage = "redis:7.0.5"
	const redisCompType = "replication"
	const redisCompName = "redis-rsts"

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
				AddComponent(testdbaas.ReplicationRedisComponent, redisCompType).
				Create(&testCtx).GetClusterDef()

			By("Create a clusterVersion obj with replication componentType.")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(redisCompType).AddContainerShort("redis", redisImage).
				Create(&testCtx).GetClusterVersion()

			By("Creating a cluster with replication componentType.")
			clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
				AddComponent(redisCompName, redisCompType).Create(&testCtx).GetCluster()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster initialized")
			Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

			stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
			sts := &stsList.Items[0]
			typeName := clusterObj.GetComponentTypeName(redisCompName)
			componentDef := clusterDefObj.GetComponentDefByTypeName(typeName)
			component := clusterObj.GetComponentByName(redisCompName)

			By("test pods are not ready")
			replicationComponent := NewReplicationSet(ctx, k8sClient, clusterObj, component, componentDef)
			sts.Status.AvailableReplicas = *sts.Spec.Replicas - 1
			podsReady, _ := replicationComponent.PodsReady(sts)
			Expect(podsReady == false).Should(BeTrue())

			By("test component is not running")
			sts.Status.AvailableReplicas = *sts.Spec.Replicas
			isRunning, _ := replicationComponent.IsRunning(sts)
			Expect(isRunning == false).Should(BeTrue())

			By("test handle probe timed out")
			requeue, _ := replicationComponent.HandleProbeTimeoutWhenPodsReady(nil)
			Expect(requeue == false).Should(BeTrue())

			By("test component phase when pods not ready")
			phase, _ := replicationComponent.GetPhaseWhenPodsNotReady(redisCompName)
			Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
		})
	})
})
