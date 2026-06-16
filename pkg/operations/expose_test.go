/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
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
			opsRes, _, clusterObject := initOperationsResources(compDefName, clusterName)

			By("create Expose opsRequest")
			ops := testops.NewOpsRequestObj("expose-expose-"+randomStr, testCtx.DefaultNamespace,
				clusterObject.Name, opsv1alpha1.ExposeType)
			ops.Spec.ExposeList = []opsv1alpha1.Expose{
				{
					ComponentName: clusterObject.Spec.ComponentSpecs[0].Name,
					Switch:        opsv1alpha1.EnableExposeSwitch,
					Services: []opsv1alpha1.OpsService{
						{
							Name:         testapps.ServiceVPCName,
							ServiceType:  corev1.ServiceTypeLoadBalancer,
							RoleSelector: testapps.Leader,
						},
					},
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock expose OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			// do expose action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("Test OpsManager.MainEnter function")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Test expose OpsRequest with empty ComponentName", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			opsRes, _, clusterObject := initOperationsResources(compDefName, clusterName)

			By("create Expose opsRequest")
			ops := testops.NewOpsRequestObj("expose-expose-"+randomStr, testCtx.DefaultNamespace,
				clusterObject.Name, opsv1alpha1.ExposeType)
			ops.Spec.ExposeList = []opsv1alpha1.Expose{
				{
					Switch: opsv1alpha1.EnableExposeSwitch,
					Services: []opsv1alpha1.OpsService{
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
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock expose OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			// do expose action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("Test OpsManager.MainEnter function")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("builds and removes cluster services from expose service specs", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			handler := ExposeOpsHandler{}
			compDef := &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "expose-cmpd-" + randomStr},
				Spec: appsv1.ComponentDefinitionSpec{
					Services: []appsv1.ComponentService{
						{
							Service: appsv1.Service{
								Name: "default",
								Spec: corev1.ServiceSpec{
									Type: corev1.ServiceTypeClusterIP,
									Ports: []corev1.ServicePort{
										{Name: "mysql", Protocol: corev1.ProtocolTCP, Port: 3306, TargetPort: intstr.FromInt(3306)},
										{Name: "mysql-dup", Protocol: corev1.ProtocolTCP, Port: 3306, TargetPort: intstr.FromInt(3306)},
									},
								},
							},
						},
						{
							Service: appsv1.Service{
								Name: "skip-node-port",
								Spec: corev1.ServiceSpec{
									Type:  corev1.ServiceTypeNodePort,
									Ports: []corev1.ServicePort{{Name: "skip", Port: 30000}},
								},
							},
						},
					},
				},
			}
			fakeScheme := runtime.NewScheme()
			Expect(appsv1.AddToScheme(fakeScheme)).Should(Succeed())
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(compDef).Build()

			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: testCtx.DefaultNamespace}}
			ipFamilyPolicy := corev1.IPFamilyPolicyPreferDualStack
			internalTrafficPolicy := corev1.ServiceInternalTrafficPolicyCluster
			err := handler.buildClusterServices(reqCtx, fakeClient, cluster, defaultCompName, compDef.Name, []opsv1alpha1.OpsService{
				{
					Name:                  "client",
					Annotations:           map[string]string{"service.beta.kubernetes.io/test": "true"},
					ServiceType:           corev1.ServiceTypeLoadBalancer,
					IPFamilyPolicy:        &ipFamilyPolicy,
					IPFamilies:            []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					InternalTrafficPolicy: &internalTrafficPolicy,
					PodSelector:           map[string]string{"app": "mysql"},
				},
				{
					Name:        "explicit",
					ServiceType: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					}},
					RoleSelector: "leader",
				},
			})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cluster.Spec.Services).Should(HaveLen(2))
			Expect(cluster.Spec.Services[0].Name).Should(Equal(defaultCompName + "-client"))
			Expect(cluster.Spec.Services[0].Spec.Ports).Should(HaveLen(1))
			Expect(cluster.Spec.Services[0].Spec.IPFamilyPolicy).Should(Equal(&ipFamilyPolicy))
			Expect(cluster.Spec.Services[0].Spec.ExternalTrafficPolicy).Should(Equal(corev1.ServiceExternalTrafficPolicyLocal))
			Expect(cluster.Spec.Services[0].Spec.InternalTrafficPolicy).Should(Equal(&internalTrafficPolicy))
			Expect(cluster.Spec.Services[1].Spec.Ports[0].TargetPort.IntVal).Should(Equal(int32(8080)))
			Expect(cluster.Spec.Services[1].RoleSelector).Should(Equal("leader"))

			err = handler.buildClusterServices(reqCtx, fakeClient, cluster, defaultCompName, compDef.Name, []opsv1alpha1.OpsService{{Name: "client"}})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(cluster.Spec.Services).Should(HaveLen(2))

			Expect(handler.removeClusterServices(cluster, defaultCompName, []opsv1alpha1.OpsService{{Name: "client"}})).Should(Succeed())
			Expect(cluster.Spec.Services).Should(HaveLen(1))
			Expect(cluster.Spec.Services[0].Name).Should(Equal(defaultCompName + "-explicit"))
			Expect(handler.removeClusterServices(nil, defaultCompName, []opsv1alpha1.OpsService{{Name: "client"}})).Should(Succeed())
			Expect(handler.buildClusterServices(reqCtx, fakeClient, nil, defaultCompName, compDef.Name, []opsv1alpha1.OpsService{{Name: "client"}})).Should(Succeed())
		})

		It("validates expose defaults when component definitions are incomplete", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			handler := ExposeOpsHandler{}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: testCtx.DefaultNamespace}}
			fakeScheme := runtime.NewScheme()
			Expect(appsv1.AddToScheme(fakeScheme)).Should(Succeed())
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).Build()

			err := handler.buildClusterServices(reqCtx, fakeClient, cluster, defaultCompName, "", []opsv1alpha1.OpsService{{Name: "client"}})
			Expect(err).Should(HaveOccurred())
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			compDef := &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "role-cmpd-" + randomStr},
				Spec: appsv1.ComponentDefinitionSpec{
					Roles: []appsv1.ReplicaRole{{Name: "leader"}},
					Services: []appsv1.ComponentService{{
						Service: appsv1.Service{
							Name: "default",
							Spec: corev1.ServiceSpec{
								Type:  corev1.ServiceTypeClusterIP,
								Ports: []corev1.ServicePort{{Name: "mysql", Port: 3306}},
							},
						},
					}},
				},
			}
			fakeClient = fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(compDef).Build()
			err = handler.buildClusterServices(reqCtx, fakeClient, cluster, defaultCompName, compDef.Name, []opsv1alpha1.OpsService{{Name: "client"}})
			Expect(err).Should(HaveOccurred())
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		})
	})
})
