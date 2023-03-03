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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("MySQL Reconfigure function", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"
	const scriptConfigName = "test-cluster-mysql-scripts"

	//const mysqlCompType = "replicasets"
	//const mysqlCompName = "mysql"

	const mysqlConfigTemplatePath = "resources/mysql_consensus_config_template.yaml"
	const mysqlConfigConstraintPath = "resources/mysql_consensus_config_constraint.yaml"
	const mysqlScriptsPath = "resources/mysql_consensus_scripts.yaml"

	const leader = "leader"
	const follower = "follower"

	// ctx := context.Background()

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
	/*
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
	*/
	testReconfigureThreeReplicas := func() {
		By("Create a cluster obj")
		clusterName := testapps.GetRandomizedKey("", clusterNamePrefix).Name
		clusterDefObj, clusterVersionObj, clusterObj = CreateSimpleConsensusMySQLClusterWithConfig(
			testCtx, clusterDefName, clusterVersionName, clusterName, mysqlConfigTemplatePath, mysqlConfigConstraintPath, mysqlScriptsPath)
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		fmt.Printf("%s %s %s \n", clusterDefObj.Name, clusterVersionObj.Name, clusterObj.Name)

		By("Waiting the cluster is created")
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.RunningPhase))

		By("Checking pods' role label")
		sts := testk8s.ListAndCheckStatefulSet(&testCtx, clusterKey).Items[0]
		pods, err := util.GetPodListByStatefulSet(testCtx.Ctx, k8sClient, &sts)
		Expect(err).To(Succeed())
		// should have 3 pods
		Expect(len(pods)).Should(Equal(3))

		// get role->count map
		roleCountMap := testapps.GetConsensusRoleCountMap(testCtx, k8sClient, clusterObj)
		Expect(roleCountMap[leader]).Should(Equal(1))
		Expect(roleCountMap[follower]).Should(Equal(2))
	}

	// Scenarios

	Context("with MySQL defined as Consensus type and three replicas", func() {
		BeforeEach(func() {
			By("Create configmap")
			_ = testapps.CreateCustomizedObj(&testCtx, "resources/mysql_scripts.yaml", &corev1.ConfigMap{},
				testapps.WithName(scriptConfigName), testCtx.UseDefaultNamespace())
		})

		It("should have one leader pod and two follower pods, and the service routes to the leader pod", func() {
			testReconfigureThreeReplicas()
		})
	})
})
