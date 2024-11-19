/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package operations

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Start OpsRequest", func() {
	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
		clusterDefName        = "test-clusterdef-" + randomStr
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {
		createStartOpsRequest := func(opsRes *OpsResource, startCompNames ...string) *appsv1alpha1.OpsRequest {
			By("create Start opsRequest")
			ops := testapps.NewOpsRequestObj("start-ops-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterName, appsv1alpha1.StartType)
			var startList []appsv1alpha1.ComponentOps
			for _, startCompName := range startCompNames {
				startList = append(startList, appsv1alpha1.ComponentOps{
					ComponentName: startCompName,
				})
			}
			ops.Spec.StartList = startList
			opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase
			return ops
		}

		It("Test start OpsRequest", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, consensusComp)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, statelessComp)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, statefulComp)
			By("create 'Start' opsRequest")
			createStartOpsRequest(opsRes)

			By("test start action and reconcile function")
			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
			// mock cluster phase to stopped
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1alpha1.StoppedClusterPhase
			})).ShouldNot(HaveOccurred())

			// set ops phase to Pending
			runAction(reqCtx, opsRes, appsv1alpha1.OpsCreatingPhase)
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))
			// do start action
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
				Expect(v.Stop).Should(BeNil())
			}
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err == nil).Should(BeTrue())
		})
		It("Test start specific components OpsRequest", func() {
			By("init operations resources with topology")
			opsRes, _, _ := initOperationsResourcesWithTopology(clusterDefName, clusterDefName, clusterName)
			// mock components is stopped
			Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(pobj *appsv1alpha1.Cluster) {
				for i := range pobj.Spec.ComponentSpecs {
					pobj.Spec.ComponentSpecs[i].Stop = pointer.Bool(true)
				}
			})).Should(Succeed())

			By("create 'Start' opsRequest for specific components")
			createStartOpsRequest(opsRes, defaultCompName)

			By("mock 'Start' OpsRequest to Creating phase")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			runAction(reqCtx, opsRes, appsv1alpha1.OpsCreatingPhase)

			By("test start action")
			startHandler := StartOpsHandler{}
			err := startHandler.Action(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("verify components are being started")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, pobj *appsv1alpha1.Cluster) {
				for _, v := range pobj.Spec.ComponentSpecs {
					if v.Name == defaultCompName {
						Expect(v.Stop).Should(BeNil())
					} else {
						Expect(v.Stop).ShouldNot(BeNil())
						Expect(*v.Stop).Should(BeTrue())
					}
				}
			})).Should(Succeed())

			By("mock components start successfully")
			testapps.MockInstanceSetPods(&testCtx, nil, opsRes.Cluster, defaultCompName)
			testapps.MockInstanceSetStatus(testCtx, opsRes.Cluster, defaultCompName)

			By("test reconcile")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("verify ops request completed")
			Eventually(testapps.GetOpsRequestPhase(&testCtx,
				client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsSucceedPhase))
		})

		It("Test abort running 'Stop' opsRequest", func() {
			By("init operations resources with topology")
			opsRes, _, _ := initOperationsResourcesWithTopology(clusterDefName, clusterDefName, clusterName)
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}

			By("create 'Stop' opsRequest for all components")
			stopOps := createStopOpsRequest(opsRes, defaultCompName)
			runAction(reqCtx, opsRes, appsv1alpha1.OpsCreatingPhase)

			By("create a start opsRequest")
			createStartOpsRequest(opsRes, defaultCompName)
			startHandler := StartOpsHandler{}
			err := startHandler.Action(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("expect the 'Stop' OpsRequest to be Aborted")
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(stopOps))).Should(Equal(appsv1alpha1.OpsAbortedPhase))
		})
	})
})
