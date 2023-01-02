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

package components

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Deployment Controller", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		timeout            = time.Second * 10
		interval           = time.Second
		clusterDefName     = "nginx-definition1-" + randomStr
		clusterVersionName = "nginx-cluster-version1-" + randomStr
		clusterName        = "nginx1-" + randomStr
		namespace          = "default"
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace(testCtx.DefaultNamespace), client.MatchingLabels{intctrlutil.AppInstanceLabelKey: clusterName}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey}, client.GracePeriodSeconds(0))
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

	Context("test controller", func() {
		It("", func() {
			if testCtx.UsingExistingCluster() {
				timeout = 3 * timeout
			}
			cluster := testdbaas.CreateStatelessCluster(testCtx, clusterDefName, clusterVersionName, clusterName)
			deploy := testdbaas.MockStatelessComponentDeploy(testCtx, clusterName)

			By("patch cluster to Updating")
			componentName := "nginx"
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Phase = dbaasv1alpha1.UpdatingPhase
			cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				componentName: {
					Phase: dbaasv1alpha1.UpdatingPhase,
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), cluster, patch)).Should(Succeed())

			By("mock deployment is ready")
			newDeployment := &appsv1.Deployment{}
			newDeploymentKey := client.ObjectKey{Name: deploy.Name, Namespace: namespace}
			Expect(k8sClient.Get(context.Background(), newDeploymentKey, newDeployment)).Should(Succeed())
			deployPatch := client.MergeFrom(newDeployment.DeepCopy())
			newDeployment.Status.ObservedGeneration = 1
			newDeployment.Status.AvailableReplicas = *newDeployment.Spec.Replicas
			newDeployment.Status.ReadyReplicas = *newDeployment.Spec.Replicas
			newDeployment.Status.Replicas = *newDeployment.Spec.Replicas
			Expect(k8sClient.Status().Patch(context.Background(), newDeployment, deployPatch)).Should(Succeed())

			By("check deployment status")
			Eventually(func() bool {
				deploy := &appsv1.Deployment{}
				if err := k8sClient.Get(context.Background(), newDeploymentKey, deploy); err != nil {
					return false
				}
				return deploy.Status.AvailableReplicas == newDeployment.Status.AvailableReplicas &&
					deploy.Status.ReadyReplicas == newDeployment.Status.ReadyReplicas &&
					deploy.Status.Replicas == newDeployment.Status.Replicas
			}, timeout, interval).Should(BeTrue())

			By("waiting the component is Running")
			Eventually(func() bool {
				cluster := &dbaasv1alpha1.Cluster{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterName, Namespace: namespace}, cluster)
				return cluster.Status.Components[componentName].Phase == dbaasv1alpha1.RunningPhase
			}, timeout, interval).Should(BeTrue())
		})
	})
})
