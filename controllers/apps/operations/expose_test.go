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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("", func() {
	var (
		randomStr   = testCtx.GetRandomStr()
		compDefName = "test-compdef-" + randomStr
		clusterName = "test-cluster-" + randomStr
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {
		It("Test expose OpsRequest", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, clusterObject := initOperationsResources2(compDefName, clusterName)

			By("create Expose opsRequest")
			ops := testapps.NewOpsRequestObj("expose-expose-"+randomStr, testCtx.DefaultNamespace,
				clusterObject.Name, appsv1alpha1.ExposeType)
			ops.Spec.ExposeList = []appsv1alpha1.Expose{
				{
					ComponentName: clusterObject.Spec.ComponentSpecs[0].Name,
					Switch:        appsv1alpha1.EnableExposeSwitch,
					Services: []appsv1alpha1.OpsService{
						{
							Name:         testapps.ServiceVPCName,
							ServiceType:  corev1.ServiceTypeLoadBalancer,
							RoleSelector: constant.Leader,
						},
					},
				},
			}
			opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase

			By("mock expose OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			// do expose action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("Test OpsManager.MainEnter function")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Test expose OpsRequest with empty ComponentName", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, clusterObject := initOperationsResources2(compDefName, clusterName)

			By("create Expose opsRequest")
			ops := testapps.NewOpsRequestObj("expose-expose-"+randomStr, testCtx.DefaultNamespace,
				clusterObject.Name, appsv1alpha1.ExposeType)
			ops.Spec.ExposeList = []appsv1alpha1.Expose{
				{
					Switch: appsv1alpha1.EnableExposeSwitch,
					Services: []appsv1alpha1.OpsService{
						{
							Name:        testapps.ServiceVPCName,
							ServiceType: corev1.ServiceTypeLoadBalancer,
							Ports: []corev1.ServicePort{
								{
									Name:       "http",
									Port:       80,
									TargetPort: intstr.FromInt(80),
								},
							},
						},
					},
				},
			}
			opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = appsv1alpha1.OpsPendingPhase

			By("mock expose OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))

			// do expose action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("Test OpsManager.MainEnter function")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
