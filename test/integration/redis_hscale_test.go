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

package dbaastest

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Redis Horizontal Scale function", func() {

	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"

	const scriptConfigName = "test-cluster-redis-scripts"
	const primaryConfigName = "redis-primary-config"
	const secondaryConfigName = "redis-secondary-config"

	const replicas = 3

	// Cleanups

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// delete rest configurations
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)

	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	// Testcases

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

	testReplicationRedisHorizontalScale := func() {

		By("Mock a cluster obj with replication componentType.")
		pvcSpec := testdbaas.NewPVC("1Gi")
		clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testdbaas.DefaultRedisCompName, testdbaas.DefaultRedisCompType).
			SetPrimaryIndex(testdbaas.DefaultReplicationPrimaryIndex).
			SetReplicas(replicas).AddVolumeClaimTemplate(testdbaas.DataVolumeName, &pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for cluster creation")
		Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Waiting for the cluster to be running")
		Eventually(testdbaas.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(dbaasv1alpha1.RunningPhase))

		By("Checking statefulSet number")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		Expect(len(stsList.Items)).Should(BeEquivalentTo(replicas))

		By("Checking statefulSet role label")
		for _, sts := range stsList.Items {
			if strings.HasSuffix(sts.Name, strconv.Itoa(testdbaas.DefaultReplicationPrimaryIndex)) {
				Expect(sts.Labels[intctrlutil.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Primary))
			} else {
				Expect(sts.Labels[intctrlutil.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Secondary))
			}
		}

		By("Checking pods number and role label in StatefulSet")
		for _, sts := range stsList.Items {
			podList, err := util.GetPodListByStatefulSet(ctx, k8sClient, &sts)
			Expect(err).To(Succeed())
			Expect(len(podList)).Should(BeEquivalentTo(1))
			if strings.HasSuffix(sts.Name, strconv.Itoa(testdbaas.DefaultReplicationPrimaryIndex)) {
				Expect(podList[0].Labels[intctrlutil.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Primary))
			} else {
				Expect(podList[0].Labels[intctrlutil.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Secondary))
			}
		}

		By("Checking services status")
		svcList := &corev1.ServiceList{}
		Expect(k8sClient.List(ctx, svcList, client.MatchingLabels{
			intctrlutil.AppInstanceLabelKey: clusterKey.Name,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())
		// we should have both external service and headless service
		Expect(len(svcList.Items)).Should(Equal(2))
		var externalSvc corev1.Service
		for _, svc := range svcList.Items {
			if svc.Spec.ClusterIP != "None" {
				externalSvc = svc
			}
		}
		Expect(externalSvc).ShouldNot(BeNil())

		for _, newReplicas := range []int32{4, 2, 7, 1} {
			By(fmt.Sprintf("horizontal scale out to %d", newReplicas))
			Expect(testdbaas.ChangeObj(&testCtx, clusterObj, func() {
				*clusterObj.Spec.Components[0].Replicas = newReplicas
			})).Should(Succeed())

			By("Wait for the cluster to be running")
			Consistently(testdbaas.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(dbaasv1alpha1.RunningPhase))

			By("Checking pods' status and count are updated in cluster status after scale-out")
			Eventually(testdbaas.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *dbaasv1alpha1.Cluster) {
				compName := fetched.Spec.Components[0].Name
				g.Expect(fetched.Status.Components).NotTo(BeNil())
				g.Expect(fetched.Status.Components).To(HaveKey(compName))
				replicationStatus := fetched.Status.Components[compName].ReplicationSetStatus
				g.Expect(replicationStatus).NotTo(BeNil())
				g.Expect(len(replicationStatus.Secondaries)).To(BeEquivalentTo(newReplicas - 1))
			})).Should(Succeed())
		}
	}

	// Scenarios

	Context("with Redis defined as replication Type and doing Horizontal scale", func() {
		BeforeEach(func() {
			_ = testdbaas.CreateCustomizedObj(&testCtx, "resources/redis_scripts.yaml", &corev1.ConfigMap{},
				testdbaas.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

			_ = testdbaas.CreateCustomizedObj(&testCtx, "resources/redis_primary_config_cm.yaml", &corev1.ConfigMap{},
				testdbaas.WithName(primaryConfigName), testCtx.UseDefaultNamespace())

			_ = testdbaas.CreateCustomizedObj(&testCtx, "resources/redis_secondary_config_cm.yaml", &corev1.ConfigMap{},
				testdbaas.WithName(secondaryConfigName), testCtx.UseDefaultNamespace())

			replicationRedisConfigVolumeMounts := []corev1.VolumeMount{
				{
					Name:      string(replicationset.Primary),
					MountPath: "/etc/conf/primary",
				},
				{
					Name:      string(replicationset.Secondary),
					MountPath: "/etc/conf/secondary",
				},
			}

			By("Create a clusterDefinition obj with replication componentType.")
			mode := int32(0755)
			clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.RedisType).
				AddComponent(testdbaas.ReplicationRedisComponent, testdbaas.DefaultRedisCompType).
				AddConfigTemplate(scriptConfigName, scriptConfigName, "", testdbaas.ScriptsVolumeName, &mode).
				AddConfigTemplate(primaryConfigName, primaryConfigName, "", string(replicationset.Primary), &mode).
				AddConfigTemplate(secondaryConfigName, secondaryConfigName, "", string(replicationset.Secondary), &mode).
				AddInitContainerVolumeMounts(testdbaas.DefaultRedisInitContainerName, replicationRedisConfigVolumeMounts).
				AddContainerVolumeMounts(testdbaas.DefaultRedisContainerName, replicationRedisConfigVolumeMounts).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication componentType.")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testdbaas.DefaultRedisCompType).
				AddInitContainerShort(testdbaas.DefaultRedisInitContainerName, testdbaas.DefaultRedisImageName).
				AddContainerShort(testdbaas.DefaultRedisContainerName, testdbaas.DefaultRedisImageName).
				Create(&testCtx).GetObject()
		})

		It("Should success with one primary and x secondaries when changes the number of replicas", func() {
			testReplicationRedisHorizontalScale()
		})
	})
})
