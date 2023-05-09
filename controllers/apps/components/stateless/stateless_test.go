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

package stateless

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
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Stateful Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "stateless-definition-" + randomStr
		clusterVersionName = "stateless-cluster-version-" + randomStr
		clusterName        = "stateless-" + randomStr
	)
	const (
		statelessCompName      = "stateless"
		statelessCompDefName   = "stateless"
		defaultMinReadySeconds = 10
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResources(&testCtx, intctrlutil.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("Stateless Component test", func() {
		It("Stateless Component test", func() {
			By(" init cluster, deployment")
			clusterDef := testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatelessNginxComponent, statelessCompDefName).
				Create(&testCtx).GetObject()
			cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(statelessCompName, statelessCompDefName).SetReplicas(2).Create(&testCtx).GetObject()
			deploy := testapps.MockStatelessComponentDeploy(testCtx, clusterName, statelessCompName)
			clusterComponent := cluster.Spec.GetComponentByName(statelessCompName)
			componentDef := clusterDef.GetComponentDefByName(clusterComponent.ComponentDefRef)
			statelessComponent, err := NewStatelessComponent(k8sClient, cluster, clusterComponent, *componentDef)
			Expect(err).Should(Succeed())
			By("test pods number of deploy is 0 ")
			phase, _ := statelessComponent.GetPhaseWhenPodsNotReady(ctx, statelessCompName)
			Expect(phase == appsv1alpha1.FailedClusterCompPhase).Should(BeTrue())

			By("test pod is ready")
			rsName := deploy.Name + "-5847cb795c"
			pod := testapps.MockStatelessPod(testCtx, deploy, clusterName, statelessCompName, rsName+randomStr)
			lastTransTime := metav1.NewTime(time.Now().Add(-1 * (defaultMinReadySeconds + 1) * time.Second))
			testk8s.MockPodAvailable(pod, lastTransTime)
			Expect(statelessComponent.PodIsAvailable(pod, defaultMinReadySeconds)).Should(BeTrue())

			By("test a part pods of deploy are not ready")
			// mock pod is not ready
			Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
				pod.Status.Conditions = nil
			})).Should(Succeed())
			// mock deployment is processing rs
			Expect(testapps.ChangeObjStatus(&testCtx, deploy, func() {
				deploy.Status.Conditions = []appsv1.DeploymentCondition{
					{
						Type:    appsv1.DeploymentProgressing,
						Reason:  "ProcessingRs",
						Status:  corev1.ConditionTrue,
						Message: fmt.Sprintf(`ReplicaSet "%s" has progressing.`, rsName),
					},
				}
				deploy.Status.ObservedGeneration = 1
			})).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, deploy, func() {
				availableReplicas := *deploy.Spec.Replicas - 1
				deploy.Status.AvailableReplicas = availableReplicas
				deploy.Status.ReadyReplicas = availableReplicas
				deploy.Status.Replicas = availableReplicas
			})).Should(Succeed())
			podsReady, _ := statelessComponent.PodsReady(ctx, deploy)
			Expect(podsReady == false).Should(BeTrue())
			phase, _ = statelessComponent.GetPhaseWhenPodsNotReady(ctx, statelessCompName)
			Expect(phase == appsv1alpha1.AbnormalClusterCompPhase).Should(BeTrue())

			By("test pods of deployment are ready")
			testk8s.MockDeploymentReady(deploy, NewRSAvailableReason, rsName)
			podsReady, _ = statelessComponent.PodsReady(ctx, deploy)
			Expect(podsReady == true).Should(BeTrue())

			By("test component.replicas is inconsistent with deployment.spec.replicas")
			oldReplicas := clusterComponent.Replicas
			replicas := int32(4)
			clusterComponent.Replicas = replicas
			isRunning, _ := statelessComponent.IsRunning(ctx, deploy)
			Expect(isRunning == false).Should(BeTrue())
			// reset replicas
			clusterComponent.Replicas = oldReplicas

			By("test component is running")
			isRunning, _ = statelessComponent.IsRunning(ctx, deploy)
			Expect(isRunning == true).Should(BeTrue())

			By("test handle probe timed out")
			requeue, _ := statelessComponent.HandleProbeTimeoutWhenPodsReady(ctx, nil)
			Expect(requeue == false).Should(BeTrue())

			By("test pod is not ready and not controlled by new ReplicaSet of deployment")
			Expect(testapps.ChangeObjStatus(&testCtx, deploy, func() {
				deploy.Status.Conditions = []appsv1.DeploymentCondition{
					{
						Type:    appsv1.DeploymentProgressing,
						Reason:  "ProcessingRs",
						Status:  corev1.ConditionTrue,
						Message: fmt.Sprintf(`ReplicaSet "%s" has progressing.`, deploy.Name+"-584f7csdb"),
					},
				}
			})).Should(Succeed())
			phase, _ = statelessComponent.GetPhaseWhenPodsNotReady(ctx, statelessCompName)
			Expect(len(phase) == 0).Should(BeTrue())
		})
	})

})
