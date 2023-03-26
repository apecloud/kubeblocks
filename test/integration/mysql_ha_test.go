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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(mysqlCompName, mysqlCompType).
			SetReplicas(3).AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("Waiting the cluster is created")
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningClusterPhase))

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
			switch pod.Labels[constant.RoleLabelKey] {
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
		// getRole should be leader through service
		Eventually(func() string {
			return getRole(&externalSvc)
		}).Should(Equal(leader))

		By("Deleting leader pod")
		leaderPod := &corev1.Pod{}
		for _, pod := range pods {
			if pod.Labels[constant.RoleLabelKey] == leader {
				leaderPod = &pod
				break
			}
		}
		Expect(k8sClient.Delete(ctx, leaderPod)).Should(Succeed())

		By("Waiting for pod recovered and new leader elected")
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sts),
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
			_ = testapps.CreateCustomizedObj(&testCtx, "resources/mysql-scripts.yaml", &corev1.ConfigMap{},
				testapps.WithName(scriptConfigName), testCtx.UseDefaultNamespace())

			By("Create a clusterDef obj")
			mode := int32(0755)
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				SetConnectionCredential(map[string]string{"username": "root", "password": ""}, nil).
				AddComponent(testapps.ConsensusMySQLComponent, mysqlCompType).
				AddScriptTemplate(scriptConfigName, scriptConfigName, testCtx.DefaultNamespace, testapps.ScriptsVolumeName, &mode).
				AddContainerEnv(testapps.DefaultMySQLContainerName, corev1.EnvVar{Name: "MYSQL_ALLOW_EMPTY_PASSWORD", Value: "yes"}).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(mysqlCompType).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()

		})

		It("should have one leader pod and two follower pods, and the service routes to the leader pod", func() {
			testThreeReplicasAndFailover()
		})
	})
})
