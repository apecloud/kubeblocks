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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("MySQL High-Availability function", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"
	const scriptConfigName = "test-cluster-mysql-scripts"

	const mysqlCompType = "replicasets"
	const mysqlCompName = "mysql"

	const leader = "leader"
	const follower = "follower"

	ctx := context.Background()

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

	getRole := func(svc *corev1.Service) (role string) {
		tunnel, err := testk8s.OpenTunnel(svc)
		defer func() {
			_ = tunnel.Close()
		}()
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(time.Second)

		db, err := tunnel.GetMySQLConn()
		defer func() {
			_ = db.Close()
		}()
		Expect(err).NotTo(HaveOccurred())

		if role, err = db.GetRole(ctx); err != nil {
			return ""
		}
		return role
	}

	testThreeReplicasAndFailover := func() {
		By("Create a cluster obj")
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		}
		clusterObj = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			SetReplicas(3).AddVolumeClaimTemplate(testdbaas.DataVolumeName, pvcSpec).
			Create(&testCtx).GetCluster()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting the cluster is created")
		Eventually(testdbaas.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(dbaasv1alpha1.RunningPhase))

		By("Checking pods' role label")
		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey)
		sts := &stsList.Items[0]
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())
		// should have 3 pods
		Expect(len(pods)).Should(Equal(3))
		// 1 leader
		// 2 followers
		leaderCount, followerCount := 0, 0
		for _, pod := range pods {
			switch pod.Labels[intctrlutil.RoleLabelKey] {
			case leader:
				leaderCount++
			case follower:
				followerCount++
			}
		}
		Expect(leaderCount).Should(Equal(1))
		Expect(followerCount).Should(Equal(2))

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
		// getRole should be leader through service
		Eventually(func() string {
			return getRole(&externalSvc)
		}).Should(Equal(leader))

		By("Deleting leader pod")
		leaderPod := &corev1.Pod{}
		for _, pod := range pods {
			if pod.Labels[intctrlutil.RoleLabelKey] == leader {
				leaderPod = &pod
				break
			}
		}
		Expect(k8sClient.Delete(ctx, leaderPod)).Should(Succeed())

		By("Waiting for pod recovered and new leader elected")
		Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(sts),
			func(g Gomega, sts *appsv1.StatefulSet) {
				g.Expect(sts.Status.AvailableReplicas == 3).To(BeTrue())
			})).Should(Succeed())

		Eventually(func() string {
			return getRole(&externalSvc)
		}).Should(Equal(leader))
	}

	// Scenarios

	Context("with MySQL defined as Consensus type and three replicas", func() {
		BeforeEach(func() {
			By("Create configmap")
			_ = testdbaas.CreateCustomizedObj(&testCtx, "resources/mysql_scripts.yaml", &corev1.ConfigMap{},
				testdbaas.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

			By("Create a clusterDef obj")
			mode := int32(0755)
			clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.MySQLType).
				SetConnectionCredential(map[string]string{"username": "root", "password": ""}).
				AddComponent(testdbaas.ConsensusMySQLComponent, mysqlCompType).
				AddConfigTemplate(scriptConfigName, scriptConfigName, "", testdbaas.ScriptsVolumeName, &mode).
				AddContainerEnv(testdbaas.DefaultMySQLContainerName, corev1.EnvVar{Name: "MYSQL_ALLOW_EMPTY_PASSWORD", Value: "yes"}).
				Create(&testCtx).GetClusterDef()

			By("Create a clusterVersion obj")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort(testdbaas.DefaultMySQLContainerName, testdbaas.ApeCloudMySQLImage).
				Create(&testCtx).GetClusterVersion()

		})

		It("should have one leader pod and two follower pods, and the service routes to the leader pod", func() {
			testThreeReplicasAndFailover()
		})
	})
})
