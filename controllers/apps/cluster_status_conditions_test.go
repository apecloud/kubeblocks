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

package apps

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("test cluster Failed/Abnormal phase", func() {

	var (
		randomStr          = testCtx.GetRandomStr()
		clusterName        = "mysql-" + randomStr
		clusterDefName     = "mysql-definition-" + randomStr
		clusterVersionName = "mysql-cluster-version-" + randomStr
		consensusCompName  = "consensus"
		consensusCompType  = "consensus"
	)
	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	updateClusterAnnotation := func(cluster *appsv1alpha1.Cluster) {
		Expect(testapps.ChangeObj(&testCtx, cluster, func() {
			cluster.Annotations = map[string]string{
				"time": time.Now().Format(time.RFC3339),
			}
		})).ShouldNot(HaveOccurred())
	}

	Context("test cluster conditions", func() {
		It("test cluster conditions", func() {
			By("init cluster")
			cluster := testapps.CreateConsensusMysqlCluster(testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompType, consensusCompName)
			clusterKey := client.ObjectKeyFromObject(cluster)

			By("mock pvc created")
			for i := 0; i < 3; i++ {
				pvcName := fmt.Sprintf("%s-%s-%s-%d", testapps.DataVolumeName, clusterKey.Name, consensusCompName, i)
				pvc := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterKey.Name,
					consensusCompName, "data").SetStorage("2Gi").Create(&testCtx).GetObject()
				// mock pvc bound
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pvc), func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Phase = corev1.ClaimBound
				})()).ShouldNot(HaveOccurred())
			}

			By("test when clusterDefinition not found")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).Should(BeEquivalentTo(constant.ReasonNotFoundCR))
			})).Should(Succeed())

			By("test when clusterVersion not Available")
			_ = testapps.CreateConsensusMysqlClusterDef(testCtx, clusterDefName, consensusCompType)
			clusterVersion := testapps.CreateConsensusMysqlClusterVersion(testCtx, clusterDefName, clusterVersionName, consensusCompType)
			clusterVersionKey := client.ObjectKeyFromObject(clusterVersion)
			// mock clusterVersion unavailable
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				clusterVersion.Spec.ComponentVersions[0].ComponentDefRef = "test-n"
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, clusterVersionKey, func(g Gomega, clusterVersion *appsv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase == appsv1alpha1.UnavailablePhase).Should(BeTrue())
			})).Should(Succeed())

			// trigger reconcile
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error1"}
			})()).ShouldNot(HaveOccurred())

			Eventually(func(g Gomega) {
				updateClusterAnnotation(cluster)
				g.Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
					condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
					g.Expect(condition).ShouldNot(BeNil())
					g.Expect(condition.Reason).Should(BeEquivalentTo(constant.ReasonRefCRUnavailable))
				})).Should(Succeed())
			}).Should(Succeed())

			By("reset clusterVersion to Available")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterVersionKey, func(clusterVersion *appsv1alpha1.ClusterVersion) {
				clusterVersion.Spec.ComponentVersions[0].ComponentDefRef = "consensus"
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, clusterVersionKey, func(g Gomega, clusterVersion *appsv1alpha1.ClusterVersion) {
				g.Expect(clusterVersion.Status.Phase == appsv1alpha1.AvailablePhase).Should(BeTrue())
			})).Should(Succeed())

			// trigger reconcile
			updateClusterAnnotation(cluster)
			By("test preCheckFailed")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1alpha1.ConditionTypeProvisioningStarted)
				g.Expect(condition != nil && condition.Reason == ReasonPreCheckFailed).Should(BeTrue())
			})).Should(Succeed())

			By("reset and waiting cluster to Creating")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error"}
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				g.Expect(tmpCluster.Status.Phase).Should(Equal(appsv1alpha1.CreatingClusterPhase))
				g.Expect(tmpCluster.Status.ObservedGeneration).ShouldNot(BeZero())
			})).Should(Succeed())

			By("test apply resources failed")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cluster), func(tmpCluster *appsv1alpha1.Cluster) {
				tmpCluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1Gi")
			})()).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster),
				func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
					condition := meta.FindStatusCondition(tmpCluster.Status.Conditions, appsv1alpha1.ConditionTypeApplyResources)
					g.Expect(condition != nil && condition.Reason == ReasonApplyResourcesFailed).Should(BeTrue())
				})).Should(Succeed())
		})
	})

})
