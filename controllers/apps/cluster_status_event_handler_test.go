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

package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("test cluster Failed/Abnormal phase", func() {
	var (
		clusterDefName = ""
		clusterVerName = ""
		clusterName    = ""
		clusterKey     types.NamespacedName
	)

	setupResourceNames := func() {
		suffix := testCtx.GetRandomStr()
		clusterDefName = "test-clusterdef-" + suffix
		clusterVerName = "test-clusterver-" + suffix
		clusterName = "test-cluster-" + suffix
	}

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		if clusterName != "" {
			testapps.ClearClusterResources(&testCtx)

			inNS := client.InNamespace(testCtx.DefaultNamespace)
			ml := client.HasLabels{testCtx.TestObjLabelKey}
			// testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
			// testapps.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
			testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PodSignature, true, inNS, ml)
		}

		// reset all resource names
		setupResourceNames()
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	const statefulMySQLCompType = "stateful"
	const statefulMySQLCompName = "stateful"

	const consensusMySQLCompType = "consensus"
	const consensusMySQLCompName = "consensus"

	const statelessCompType = "stateless"
	const statelessCompName = "nginx"

	createClusterDef := func() {
		_ = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, statefulMySQLCompType).
			AddComponentDef(testapps.ConsensusMySQLComponent, consensusMySQLCompType).
			AddComponentDef(testapps.StatelessNginxComponent, statelessCompType).
			Create(&testCtx)
	}

	createClusterVer := func() {
		_ = testapps.NewClusterVersionFactory(clusterVerName, clusterDefName).
			AddComponentVersion(statefulMySQLCompType).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
			AddComponentVersion(consensusMySQLCompType).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
			AddComponentVersion(statelessCompType).AddContainerShort(testapps.DefaultNginxContainerName, testapps.NginxImage).
			Create(&testCtx)
	}

	createCluster := func() *appsv1alpha1.Cluster {
		clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVerName).
			AddComponent(statefulMySQLCompName, statefulMySQLCompType).SetReplicas(3).
			AddComponent(consensusMySQLCompName, consensusMySQLCompType).SetReplicas(3).
			AddComponent(statelessCompName, statelessCompType).SetReplicas(3).
			Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
		return clusterObj
	}

	Context("test cluster Failed/Abnormal phase", func() {
		It("test cluster Failed/Abnormal phase", func() {
			By("create cluster related resources")
			createClusterDef()
			createClusterVer()
			// cluster := createCluster()
			createCluster()

			// wait for cluster's status to become stable so that it won't interfere with later tests
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1alpha1.Cluster) {
				g.Expect(cluster.Generation).To(BeEquivalentTo(1))
				g.Expect(cluster.Status.ObservedGeneration).To(BeEquivalentTo(1))
				g.Expect(cluster.Status.Phase).To(Equal(appsv1alpha1.CreatingClusterPhase))
			})).Should(Succeed())

			By("watch normal event")
			event := &corev1.Event{
				Count:   1,
				Type:    corev1.EventTypeNormal,
				Message: "create pod failed because the pvc is deleting",
			}
			Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())

			By("watch warning event from workload, but mismatch condition ")
			rsmKey := types.NamespacedName{
				Namespace: clusterKey.Namespace,
				Name:      clusterKey.Name + "-" + statefulMySQLCompName,
			}
			Eventually(testapps.CheckObjExists(&testCtx, rsmKey, &workloads.ReplicatedStateMachine{}, true)).Should(Succeed())

			involvedObject := corev1.ObjectReference{
				Name:      rsmKey.Name,
				Kind:      constant.RSMKind,
				Namespace: testCtx.DefaultNamespace,
			}
			event.InvolvedObject = involvedObject
			event.Type = corev1.EventTypeWarning
			Expect(handleEventForClusterStatus(ctx, k8sClient, clusterRecorder, event)).Should(Succeed())
		})
	})
})
