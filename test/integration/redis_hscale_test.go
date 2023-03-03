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

package appstest

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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
		testapps.ClearClusterResources(&testCtx)

		// delete rest configurations
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)

	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	// Testcases

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		clusterObj        *appsv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

	testReplicationRedisHorizontalScale := func() {

		By("Mock a cluster obj with replication workloadType.")
		pvcSpec := testapps.NewPVC("1Gi")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
			SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
			SetReplicas(replicas).AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting for cluster creation")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))

		By("Waiting for the cluster to be running")
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningPhase))

		By("Checking statefulSet number")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		Expect(len(stsList.Items)).Should(BeEquivalentTo(replicas))

		By("Checking statefulSet role label")
		for _, sts := range stsList.Items {
			if strings.HasSuffix(sts.Name, strconv.Itoa(testapps.DefaultReplicationPrimaryIndex)) {
				Expect(sts.Labels[constant.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Primary))
			} else {
				Expect(sts.Labels[constant.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Secondary))
			}
		}

		By("Checking pods number and role label in StatefulSet")
		for _, sts := range stsList.Items {
			podList, err := util.GetPodListByStatefulSet(ctx, k8sClient, &sts)
			Expect(err).To(Succeed())
			Expect(len(podList)).Should(BeEquivalentTo(1))
			if strings.HasSuffix(sts.Name, strconv.Itoa(testapps.DefaultReplicationPrimaryIndex)) {
				Expect(podList[0].Labels[constant.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Primary))
			} else {
				Expect(podList[0].Labels[constant.RoleLabelKey]).Should(BeEquivalentTo(replicationset.Secondary))
			}
		}

		By("Checking services status")
		svcList := &corev1.ServiceList{}
		Expect(k8sClient.List(ctx, svcList, client.MatchingLabels{
			constant.AppInstanceLabelKey: clusterKey.Name,
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
			Expect(testapps.ChangeObj(&testCtx, clusterObj, func() {
				clusterObj.Spec.ComponentSpecs[0].Replicas = newReplicas
			})).Should(Succeed())

			By("Wait for the cluster to be running")
			Consistently(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningPhase))

			By("Checking pods' status and count are updated in cluster status after scale-out")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, fetched *appsv1alpha1.Cluster) {
				compName := fetched.Spec.ComponentSpecs[0].Name
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
			_ = testapps.CreateCustomizedObj(&testCtx, "resources/redis_scripts.yaml", &corev1.ConfigMap{},
				testapps.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

			_ = testapps.CreateCustomizedObj(&testCtx, "resources/redis_primary_config_cm.yaml", &corev1.ConfigMap{},
				testapps.WithName(primaryConfigName), testCtx.UseDefaultNamespace())

			_ = testapps.CreateCustomizedObj(&testCtx, "resources/redis_secondary_config_cm.yaml", &corev1.ConfigMap{},
				testapps.WithName(secondaryConfigName), testCtx.UseDefaultNamespace())

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

			By("Create a clusterDefinition obj with replication workloadType.")
			mode := int32(0755)
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompType).
				AddConfigTemplate(scriptConfigName, scriptConfigName, "", testCtx.DefaultNamespace, testapps.ScriptsVolumeName, &mode).
				AddConfigTemplate(primaryConfigName, primaryConfigName, "", testCtx.DefaultNamespace, string(replicationset.Primary), &mode).
				AddConfigTemplate(secondaryConfigName, secondaryConfigName, "", testCtx.DefaultNamespace, string(replicationset.Secondary), &mode).
				AddInitContainerVolumeMounts(testapps.DefaultRedisInitContainerName, replicationRedisConfigVolumeMounts).
				AddContainerVolumeMounts(testapps.DefaultRedisContainerName, replicationRedisConfigVolumeMounts).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(testapps.DefaultRedisCompType).
				AddInitContainerShort(testapps.DefaultRedisInitContainerName, testapps.DefaultRedisImageName).
				AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()
		})

		It("Should success with one primary and x secondaries when changes the number of replicas", func() {
			testReplicationRedisHorizontalScale()
		})
	})
})
