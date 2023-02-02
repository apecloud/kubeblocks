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
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Annotations = map[string]string{
			"time": time.Now().Format(time.RFC3339),
		}
		Expect(k8sClient.Patch(ctx, cluster, patch)).Should(Succeed())
	}

	Context("test cluster conditions", func() {
		It("test cluster conditions", func() {
			By("init cluster")
			_ = testdbaas.CreateConsensusMysqlCluster(ctx, testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompName)
			By("test when clusterDefinition not found")
			cluster := &dbaasv1alpha1.Cluster{}
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)).Should(Succeed())
				condition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeProvisioningStarted)
				return condition != nil && condition.Reason == intctrlutil.ReasonNotFoundCR
			}, timeout, interval).Should(BeTrue())

			By("test conditionsError phase")
			patch := client.MergeFrom(cluster.DeepCopy())
			condition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeProvisioningStarted)
			condition.LastTransitionTime = metav1.Time{Time: time.Now().Add(-(ClusterControllerErrorDuration + time.Second))}
			cluster.SetStatusCondition(*condition)
			Expect(k8sClient.Status().Patch(ctx, cluster, patch)).Should(Succeed())

			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)).Should(Succeed())
				return cluster.Status.Phase == dbaasv1alpha1.ConditionsErrorPhase
			}, timeout*2, interval).Should(BeTrue())

			By("test when clusterVersion not Available")
			_ = testdbaas.CreateConsensusMysqlClusterDef(ctx, testCtx, clusterDefName)
			_ = testdbaas.CreateConsensusMysqlClusterVersion(ctx, testCtx, clusterDefName, clusterVersionName)
			// mock clusterVersion unavailable
			Eventually(func() bool {
				clusterVersion := &dbaasv1alpha1.ClusterVersion{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterVersionName}, clusterVersion)).Should(Succeed())
				clusterVersion.Spec.Components[0].Type = "test-n"
				err := k8sClient.Update(ctx, clusterVersion)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				clusterVersion := &dbaasv1alpha1.ClusterVersion{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: clusterVersionName}, clusterVersion)
				return clusterVersion.Status.Phase == dbaasv1alpha1.UnavailablePhase
			}, timeout, interval).Should(BeTrue())
			// trigger reconcile
			Eventually(func() bool {
				tmpCluster := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, tmpCluster)).Should(Succeed())
				tmpCluster.Spec.Components[0].EnabledLogs = []string{"error1"}
				err := k8sClient.Update(ctx, tmpCluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				updateClusterAnnotation(cluster)
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)).Should(Succeed())
				condition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeProvisioningStarted)
				return condition != nil && condition.Reason == intctrlutil.ReasonRefCRUnavailable
			}, timeout, interval).Should(BeTrue())

			By("reset clusterVersion to Available")
			Eventually(func() bool {
				clusterVersion := &dbaasv1alpha1.ClusterVersion{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterVersionName}, clusterVersion)).Should(Succeed())
				clusterVersion.Spec.Components[0].Type = "consensus"
				err := k8sClient.Update(ctx, clusterVersion)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				clusterVersion := &dbaasv1alpha1.ClusterVersion{}
				_ = k8sClient.Get(ctx, client.ObjectKey{Name: clusterVersionName}, clusterVersion)
				return clusterVersion.Status.Phase == dbaasv1alpha1.AvailablePhase
			}, timeout, interval).Should(BeTrue())

			// trigger reconcile
			updateClusterAnnotation(cluster)
			By("test preCheckFailed")
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)).Should(Succeed())
				condition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeProvisioningStarted)
				return condition != nil && condition.Reason == ReasonPreCheckFailed
			}, timeout*2, interval).Should(BeTrue())

			By("reset and waiting cluster to Creating")
			Eventually(func() bool {
				tmpCluster := &dbaasv1alpha1.Cluster{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, tmpCluster)).Should(Succeed())
				tmpCluster.Spec.Components[0].EnabledLogs = []string{"error"}
				err := k8sClient.Update(ctx, tmpCluster)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)).Should(Succeed())
				return cluster.Status.Phase == dbaasv1alpha1.CreatingPhase
			}, timeout*2, interval).Should(BeTrue())

			By("test apply resources failed")
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)).Should(Succeed())
				cluster.Spec.Components[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1Gi")
				return k8sClient.Update(ctx, cluster) == nil
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: testCtx.DefaultNamespace}, cluster)).Should(Succeed())
				condition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionTypeApplyResources)
				return condition != nil && condition.Reason == ReasonApplyResourcesFailed
			}, timeout*2, interval).Should(BeTrue())

		})
	})

})
