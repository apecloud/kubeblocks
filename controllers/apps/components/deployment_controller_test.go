/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package components

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Deployment Controller", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "stateless-definition1-" + randomStr
		clusterVersionName = "stateless-cluster-version1-" + randomStr
		clusterName        = "stateless1-" + randomStr
	)

	const (
		namespace            = "default"
		statelessCompName    = "stateless"
		statelessCompDefName = "stateless"
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("test controller", func() {
		It("", func() {
			testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatelessNginxComponent, statelessCompDefName).
				Create(&testCtx).GetObject()

			cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(statelessCompName, statelessCompDefName).SetReplicas(2).Create(&testCtx).GetObject()

			clusterKey := client.ObjectKeyFromObject(cluster)

			By("patch cluster to Running")
			Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
				cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
			}))

			By("create the deployment of the stateless component")
			deploy := testapps.MockStatelessComponentDeploy(&testCtx, clusterName, statelessCompName)
			newDeploymentKey := client.ObjectKey{Name: deploy.Name, Namespace: namespace}
			Eventually(testapps.CheckObj(&testCtx, newDeploymentKey, func(g Gomega, deploy *appsv1.Deployment) {
				g.Expect(deploy.Generation == 1).Should(BeTrue())
			})).Should(Succeed())

			By("check stateless component phase is Failed")
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, statelessCompName)).Should(Equal(appsv1alpha1.FailedClusterCompPhase))

			By("mock error message and PodCondition about some pod's failure")
			podName := fmt.Sprintf("%s-%s-%s", clusterName, statelessCompName, testCtx.GetRandomStr())
			pod := testapps.MockStatelessPod(&testCtx, deploy, clusterName, statelessCompName, podName)
			// mock pod container is failed
			errMessage := "Back-off pulling image nginx:latest"
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "ImagePullBackOff",
								Message: errMessage,
							},
						},
					},
				}
			})).Should(Succeed())
			// mock failed container timed out
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.Conditions = []corev1.PodCondition{
					{
						Type:               corev1.ContainersReady,
						Status:             corev1.ConditionFalse,
						LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
					},
				}
			})).Should(Succeed())
			// mark deployment to reconcile
			Expect(testapps.ChangeObj(&testCtx, deploy, func(ldeploy *appsv1.Deployment) {
				ldeploy.Annotations = map[string]string{
					"reconcile": "1",
				}
			})).Should(Succeed())

			By("check component.Status.Message contains pod error message")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cluster), func(g Gomega, tmpCluster *appsv1alpha1.Cluster) {
				compStatus := tmpCluster.Status.Components[statelessCompName]
				g.Expect(compStatus.GetObjectMessage("Pod", pod.Name)).Should(Equal(errMessage))
			})).Should(Succeed())

			By("mock deployment is ready")
			Expect(testapps.ChangeObjStatus(&testCtx, deploy, func() {
				testk8s.MockDeploymentReady(deploy, stateless.NewRSAvailableReason, deploy.Name+"-5847cb795c")
			})).Should(Succeed())

			By("waiting for the component to be running")
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, statelessCompName)).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		})
	})
})
