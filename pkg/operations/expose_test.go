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
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

func TestExposeActionRejectsDuplicateGeneratedNamesBeforeUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec:       appsv1.ClusterSpec{Services: []appsv1.ClusterService{{Service: appsv1.Service{Name: "existing"}}}},
	}
	ops := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{
		Type: opsv1alpha1.ExposeType,
		SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{ExposeList: []opsv1alpha1.Expose{
			{ComponentName: "db", Services: []opsv1alpha1.OpsService{{Name: "read"}}},
			{Services: []opsv1alpha1.OpsService{{Name: "db-read"}}},
		}},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, ops).Build()
	res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(10)}

	if err := (ExposeOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, res); err == nil {
		t.Fatal("duplicate generated service name was accepted")
	}
	if len(cluster.Spec.Services) != 1 || cluster.Spec.Services[0].Name != "existing" {
		t.Fatalf("cluster services changed after rejected input: %#v", cluster.Spec.Services)
	}
}

func TestExposeActionDoesNotPartiallyApplyLaterInvalidTarget(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec:       appsv1.ClusterSpec{},
	}
	ops := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{
		Type: opsv1alpha1.ExposeType,
		SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{ExposeList: []opsv1alpha1.Expose{
			{Switch: opsv1alpha1.EnableExposeSwitch, Services: []opsv1alpha1.OpsService{{Name: "read", Ports: []corev1.ServicePort{{Port: 80}}}}},
			{ComponentName: "missing", Switch: opsv1alpha1.DisableExposeSwitch, Services: []opsv1alpha1.OpsService{{Name: "write"}}},
		}},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, ops).Build()
	res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(10)}

	if err := (ExposeOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, res); err == nil {
		t.Fatal("missing component was accepted")
	}
	if len(cluster.Spec.Services) != 0 {
		t.Fatalf("cluster services changed after rejected input: %#v", cluster.Spec.Services)
	}
}

func TestValidateExposeTargetsRejectsDuplicateLogicalTargets(t *testing.T) {
	err := validateExposeTargets([]opsv1alpha1.Expose{
		{ComponentName: "db", Services: []opsv1alpha1.OpsService{{Name: "read"}}},
		{ComponentName: "db", Services: []opsv1alpha1.OpsService{{Name: "read"}}},
	})
	if err == nil {
		t.Fatal("duplicate logical target was accepted")
	}
}

func TestExposeActionKeepsMultipleServicesForOneComponent(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}, Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "db", ComponentDef: "mysql"}}}}
	ops := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.ExposeType, SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{ExposeList: []opsv1alpha1.Expose{
		{ComponentName: "db", Switch: opsv1alpha1.EnableExposeSwitch, Services: []opsv1alpha1.OpsService{{Name: "read", ServiceType: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}}}},
		{ComponentName: "db", Switch: opsv1alpha1.EnableExposeSwitch, Services: []opsv1alpha1.OpsService{{Name: "write", ServiceType: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Name: "http", Port: 81, TargetPort: intstr.FromInt(8081)}}}}},
	}}}}
	compDef := &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "mysql"}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, ops, compDef).Build()
	res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(10)}
	if err := (ExposeOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, res); err != nil {
		t.Fatal(err)
	}
	if len(cluster.Spec.Services) != 2 || cluster.Spec.Services[0].Name != "db-read" || cluster.Spec.Services[1].Name != "db-write" {
		t.Fatalf("services = %#v", cluster.Spec.Services)
	}
}

func TestExposeReconcileAggregatesComponentServiceProgress(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, disable := range []bool{false, true} {
			t.Run(fmt.Sprintf("reverse-%t-disable-%t", reverse, disable), func(t *testing.T) {
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, corev1.AddToScheme} {
					if err := add(scheme); err != nil {
						t.Fatal(err)
					}
				}
				cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}
				targets := []opsv1alpha1.Expose{
					{ComponentName: "db", Switch: opsv1alpha1.EnableExposeSwitch, Services: []opsv1alpha1.OpsService{{Name: "read"}}},
					{ComponentName: "db", Switch: opsv1alpha1.EnableExposeSwitch, Services: []opsv1alpha1.OpsService{{Name: "write"}}},
					{ComponentName: "cache", Switch: opsv1alpha1.EnableExposeSwitch, Services: []opsv1alpha1.OpsService{{Name: "read"}}},
				}
				if disable {
					targets[1].Switch = opsv1alpha1.DisableExposeSwitch
				}
				if reverse {
					targets[0], targets[1] = targets[1], targets[0]
				}
				ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "expose", Namespace: "default"},
					Spec:   opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.ExposeType, SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{ExposeList: targets}},
					Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase}}
				service := func(name string) *corev1.Service {
					return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name, Namespace: "default", Labels: map[string]string{constant.AppInstanceLabelKey: "demo"}}}
				}
				objects := []client.Object{cluster, ops, service("db-read")}
				if disable {
					objects = append(objects, service("db-write"))
				}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(ops).Build()
				ctx := context.Background()
				req := intctrlutil.RequestCtx{Ctx: ctx, Recorder: record.NewFakeRecorder(10)}
				check := func(phase opsv1alpha1.OpsPhase, progress string, dbPhase appsv1.ComponentPhase) {
					t.Helper()
					current := &opsv1alpha1.OpsRequest{}
					if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), current); err != nil {
						t.Fatal(err)
					}
					res := &OpsResource{Cluster: cluster, OpsRequest: current, Recorder: req.Recorder}
					if _, err := GetOpsManager().Reconcile(req, cli, res); err != nil {
						t.Fatal(err)
					}
					if err := cli.Get(ctx, client.ObjectKeyFromObject(ops), current); err != nil {
						t.Fatal(err)
					}
					if current.Status.Phase != phase || current.Status.Progress != progress || current.Status.Components["db"].Phase != dbPhase {
						t.Fatalf("phase=%s progress=%s db=%s, want %s %s %s", current.Status.Phase, current.Status.Progress, current.Status.Components["db"].Phase, phase, progress, dbPhase)
					}
				}
				check(opsv1alpha1.OpsRunningPhase, "1/3", appsv1.UpdatingComponentPhase)
				check(opsv1alpha1.OpsRunningPhase, "1/3", appsv1.UpdatingComponentPhase)
				if disable {
					if err := cli.Delete(ctx, service("db-write")); err != nil {
						t.Fatal(err)
					}
				} else if err := cli.Create(ctx, service("db-write")); err != nil {
					t.Fatal(err)
				}
				check(opsv1alpha1.OpsRunningPhase, "2/3", appsv1.RunningComponentPhase)
				if err := cli.Delete(ctx, service("db-read")); err != nil {
					t.Fatal(err)
				}
				check(opsv1alpha1.OpsRunningPhase, "1/3", appsv1.UpdatingComponentPhase)
				for _, name := range []string{"db-read", "cache-read"} {
					if err := cli.Create(ctx, service(name)); err != nil {
						t.Fatal(err)
					}
				}
				check(opsv1alpha1.OpsSucceedPhase, "3/3", appsv1.RunningComponentPhase)
			})
		}
	}
}

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

			persisted := &appsv1.Cluster{}
			Expect(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(clusterObject), persisted)).Should(Succeed())
			Expect(persisted.Spec.Services).Should(HaveLen(1))
			Expect(persisted.Spec.Services[0].Name).Should(Equal(defaultCompName + "-" + testapps.ServiceVPCName))
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
