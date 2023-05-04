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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Pod Controller", func() {

	var (
		randomStr          = testCtx.GetRandomStr()
		clusterName        = "mysql-" + randomStr
		clusterDefName     = "cluster-definition-consensus-" + randomStr
		clusterVersionName = "cluster-version-operations-" + randomStr
	)

	const (
		revisionID        = "6fdd48d9cd"
		consensusCompName = "consensus"
		consensusCompType = "consensus"
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
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("test controller", func() {
		It("test pod controller", func() {

			leaderName := "test-leader-name"
			podName := "test-pod-name"

			By("mock cluster object")
			_, _, cluster := testapps.InitConsensusMysql(testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompType, consensusCompName)

			By("mock cluster's consensus status")
			Expect(testapps.ChangeObjStatus(&testCtx, cluster, func() {
				cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{}
				cluster.Status.Components[consensusCompName] = appsv1alpha1.ClusterComponentStatus{
					ConsensusSetStatus: &appsv1alpha1.ConsensusSetStatus{
						Leader: appsv1alpha1.ConsensusMemberStatus{
							Pod:        leaderName,
							AccessMode: "ReadWrite",
						},
					},
				}
			})).Should(Succeed())

			By("triggering pod reconcile")
			pod := testapps.NewPodFactory(cluster.Namespace, podName).
				AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
				AddLabels(constant.AppInstanceLabelKey, cluster.Name).
				AddLabels(constant.KBAppComponentLabelKey, consensusCompName).
				Create(&testCtx).GetObject()
			podKey := client.ObjectKeyFromObject(pod)

			By("checking pod has leader annotation")
			testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Annotations).ShouldNot(BeNil())
				g.Expect(pod.Annotations[constant.LeaderAnnotationKey]).Should(Equal(leaderName))
			})
		})
	})
})
