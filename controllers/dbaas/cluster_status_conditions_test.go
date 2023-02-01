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

package dbaas

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("test cluster Failed/Abnormal phase", func() {

	var (
		randomStr          = testCtx.GetRandomStr()
		clusterName        = "mysql-" + randomStr
		clusterDefName     = "mysql-definition-" + randomStr
		clusterVersionName = "mysql-cluster-version-" + randomStr
		timeout            = time.Second * 10
		interval           = time.Second
		consensusCompName  = "consensus"
	)
	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	updateClusterAnnotation := func(cluster *dbaasv1alpha1.Cluster) {
		Expect(testdbaas.ChangeObj(&testCtx, cluster, func() {
			cluster.Annotations = map[string]string{
				"time": time.Now().Format(time.RFC3339),
			}
		})).Should(Succeed())
	}

	Context("test cluster conditions", func() {
		It("test cluster conditions", func() {
			By("init cluster")
			cluster := testdbaas.CreateConsensusMysqlCluster(testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompName)
			By("test when clusterDefinition not found")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
				condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, ConditionTypeProvisioningStarted)
				g.Expect(condition != nil && condition.Reason == intctrlutil.ReasonNotFoundCR).Should(BeTrue())
			})).Should(Succeed())

			By("test conditionsError phase")
			Expect(testdbaas.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *dbaasv1alpha1.Cluster) {
				condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, ConditionTypeProvisioningStarted)
				condition.LastTransitionTime = metav1.Time{Time: time.Now().Add(-(ClusterControllerErrorDuration + time.Second))}
				tmpCluster.SetStatusCondition(*condition)
			})()).Should(Succeed())

			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase == dbaasv1alpha1.ConditionsErrorPhase).Should(BeTrue())
			}), timeout*2, interval).Should(Succeed())

			By("test when clusterVersion not Available")
			_ = testdbaas.CreateConsensusMysqlClusterDef(testCtx, clusterDefName)
			clusterVersion := testdbaas.CreateConsensusMysqlClusterVersion(testCtx, clusterDefName, clusterVersionName)
			// mock clusterVersion unavailable
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterVersion), func(clusterVersion *dbaasv1alpha1.ClusterVersion) {
				clusterVersion.Spec.Components[0].Type = "test-n"
			})).Should(Succeed())

			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersion), func(g Gomega, clusterVersion *dbaasv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase == dbaasv1alpha1.UnavailablePhase).Should(BeTrue())
			}), timeout*2, interval).Should(Succeed())

			// trigger reconcile
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *dbaasv1alpha1.Cluster) {
				tmpCluster.Spec.Components[0].EnabledLogs = []string{"error1"}
			})).Should(Succeed())

			Eventually(func(g Gomega) {
				updateClusterAnnotation(cluster)
				g.Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
					condition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeProvisioningStarted)
					g.Expect(condition != nil && condition.Reason == intctrlutil.ReasonRefCRUnavailable).Should(BeTrue())
				})).Should(Succeed())
			}).Should(Succeed())

			By("reset clusterVersion to Available")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterVersion), func(clusterVersion *dbaasv1alpha1.ClusterVersion) {
				clusterVersion.Spec.Components[0].Type = "consensus"
			})).Should(Succeed())

			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersion), func(g Gomega, clusterVersion *dbaasv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase == dbaasv1alpha1.AvailablePhase).Should(BeTrue())
			}), timeout*2, interval).Should(Succeed())

			// trigger reconcile
			updateClusterAnnotation(cluster)
			By("test preCheckFailed")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, cluster *dbaasv1alpha1.Cluster) {
				condition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeProvisioningStarted)
				g.Expect(condition != nil && condition.Reason == ReasonPreCheckFailed).Should(BeTrue())
			}), timeout*2, interval).Should(Succeed())

			By("reset and waiting cluster to Creating")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *dbaasv1alpha1.Cluster) {
				tmpCluster.Spec.Components[0].EnabledLogs = []string{"error"}
			})).Should(Succeed())

			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase == dbaasv1alpha1.CreatingPhase).Should(BeTrue())
			}), timeout*2, interval).Should(Succeed())

			By("test apply resources failed")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *dbaasv1alpha1.Cluster) {
				tmpCluster.Spec.Components[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1Gi")
			})).Should(Succeed())

			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *dbaasv1alpha1.Cluster) {
				condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, ConditionTypeApplyResources)
				g.Expect(condition != nil && condition.Reason == ReasonApplyResourcesFailed).Should(BeTrue())
			}), timeout*2, interval).Should(Succeed())
		})
	})

})
