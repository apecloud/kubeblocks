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

package replicationset

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Replication Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterName        = "cluster-replication" + randomStr
		clusterDefName     = "cluster-def-replication-" + randomStr
		clusterVersionName = "cluster-version-replication-" + randomStr
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
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

	Context("Replication Component test", func() {
		It("Replication Component test", func() {
			By(" init cluster, statefulSet, pods")
			clusterDef, _, cluster := testdbaas.InitReplicationRedis(testCtx, clusterDefName, clusterVersionName, clusterName)

			sts := testdbaas.MockReplicationComponentStatefulSet(testCtx, clusterName)
			componentName := testdbaas.ReplicationComponentName
			typeName := util.GetComponentTypeName(*cluster, componentName)
			componentDef := util.GetComponentDefFromClusterDefinition(clusterDef, typeName)
			component := util.GetComponentByName(cluster, componentName)

			By("test pods are not ready")
			replicationComponent := NewReplicationSet(ctx, k8sClient, cluster, component, componentDef)
			sts.Status.AvailableReplicas = *sts.Spec.Replicas - 1
			podsReady, _ := replicationComponent.PodsReady(sts)
			Expect(podsReady == false).Should(BeTrue())

			By("test component is running")
			sts.Status.AvailableReplicas = *sts.Spec.Replicas
			isRunning, _ := replicationComponent.IsRunning(sts)
			Expect(isRunning == false).Should(BeTrue())

			By("test handle probe timed out")
			requeue, _ := replicationComponent.HandleProbeTimeoutWhenPodsReady()
			Expect(requeue == true).Should(BeTrue())

			By("test component phase when pods not ready")
			phase, _ := replicationComponent.CalculatePhaseWhenPodsNotReady(testdbaas.ReplicationComponentName)
			Expect(phase == dbaasv1alpha1.FailedPhase).Should(BeTrue())
		})
	})
})
