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

package parameters

import (
	"context"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/pkg/parameters/unstructured"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("ComponentParameter Controller", func() {
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("reconcile", func() {
		It("should reconcile success", func() {
			_, _, _, _, itsObj := mockReconcileResource()

			By("wait for component parameter to be ready")
			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
				status := parameters.GetItemStatus(&cfg.Status, configSpecName)
				g.Expect(status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			By("update parameters")
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&cfg.Spec, configSpecName)
				item.ConfigFileParams = map[string]parametersv1alpha1.ParametersInFile{
					"my.cnf": {
						Parameters: map[string]*string{
							"max_connections": cfgutil.ToPointer("1000"),
							"gtid_mode":       cfgutil.ToPointer("ON"),
						},
					},
				}
			})).Should(Succeed())

			By("check component parameter status is updated to Upgrading")
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				status := parameters.GetItemStatus(&cfg.Status, configSpecName)
				g.Expect(status).ShouldNot(BeNil())
				g.Expect(status.UpdateRevision).Should(BeEquivalentTo("3"))
				g.Expect(status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CUpgradingPhase))
			})).Should(Succeed())

			By("mock the reconfigure done")
			mockReconfigureDone(itsObj.Namespace, itsObj.Name, configSpecName,
				waitRenderedConfigHash(
					testCtx.DefaultNamespace, clusterName, defaultCompName, configSpecName,
					"max_connections=1000", "gtid_mode=ON",
				))

			By("check component parameter status is updated to Finished")
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				status := parameters.GetItemStatus(&cfg.Status, configSpecName)
				g.Expect(status).ShouldNot(BeNil())
				g.Expect(status.UpdateRevision).Should(BeEquivalentTo("3"))
				g.Expect(status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		})

		It("should rerender config when the component changes", func() {
			templateObj, clusterObj, compObj, _, _ := mockReconcileResource()

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			By("update the template to depend on the live component replicas")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(templateObj), func(tpl *corev1.ConfigMap) {
				tpl.Data[testparameters.MysqlConfigFile] = strings.ReplaceAll(
					tpl.Data[testparameters.MysqlConfigFile],
					"server-id=1",
					"server-id={{ $.component.replicas }}",
				)
			})).Should(Succeed())

			By("update the cluster and component replicas without touching ComponentParameter spec")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterObj), func(cluster *appsv1.Cluster) {
				cluster.Spec.ComponentSpecs[0].Replicas = 2
			})).Should(Succeed())
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compObj), func(comp *appsv1.Component) {
				comp.Spec.Replicas = 2
			})).Should(Succeed())

			configKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GetComponentCfgName(clusterName, defaultCompName, configSpecName),
			}
			Eventually(testapps.CheckObj(&testCtx, configKey, func(g Gomega, cfg *corev1.ConfigMap) {
				g.Expect(cfg.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("server-id=2"))
				g.Expect(cfg.Annotations[constant.ParametersAppliedComponentGenerationKey]).ShouldNot(BeEmpty())
			})).Should(Succeed())
		})

		It("should project desired parameters into config item details", func() {
			_, _, _, _, _ = mockReconcileResource()

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			customTemplate := testparameters.NewComponentTemplateFactory("desired-template", testCtx.DefaultNamespace).
				AddConfigFile(testparameters.MysqlConfigFile, "max_connections=2000\n").
				Create(&testCtx).
				GetObject()

			By("update desired state instead of config item details directly")
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				cfg.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{
						"max_connections": cfgutil.ToPointer("2000"),
					},
					Templates: map[string]parametersv1alpha1.ConfigTemplateExtension{
						configSpecName: {
							TemplateRef: customTemplate.Name,
							Namespace:   customTemplate.Namespace,
							Policy:      parametersv1alpha1.ReplacePolicy,
						},
					},
				}
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&cfg.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates.TemplateRef).Should(Equal(customTemplate.Name))
				g.Expect(item.ConfigFileParams).Should(HaveKey("my.cnf"))
				g.Expect(item.ConfigFileParams["my.cnf"].Parameters).Should(HaveKeyWithValue("max_connections", cfgutil.ToPointer("2000")))
			})).Should(Succeed())
		})

		It("should fail when desired parameters violate ParametersDefinition schema", func() {
			_, _, _, _, _ = mockReconcileResource()

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			By("write an invalid desired parameter directly to ComponentParameter")
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				cfg.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{
						"max_connections": cfgutil.ToPointer("0"),
					},
				}
			})).Should(Succeed())

			By("verify ComponentParameter fails closed instead of finishing the change")
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				g.Expect(cfg.Status.ObservedGeneration).Should(Equal(cfg.Generation))
				g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CMergeFailedPhase))
				g.Expect(cfg.Status.Message).Should(ContainSubstring("max_connections"))
				g.Expect(cfg.Status.Message).Should(ContainSubstring("invalid"))
			})).Should(Succeed())
		})

		It("should project init first and let desired override it", func() {
			_, _, _, _, _ = mockReconcileResource()

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				cfg.Spec.Initial = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{
						"max_connections": cfgutil.ToPointer("1000"),
					},
				}
				cfg.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{
						"max_connections": cfgutil.ToPointer("2000"),
					},
				}
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&cfg.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.ConfigFileParams).Should(HaveKey("my.cnf"))
				g.Expect(item.ConfigFileParams["my.cnf"].Parameters).Should(HaveKeyWithValue("max_connections", cfgutil.ToPointer("2000")))
			})).Should(Succeed())
		})

		It("should let explicit desired remove override assignments", func() {
			_, _, comp, _, _ := mockReconcileResource()

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				cfg.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{
						"max_connections": cfgutil.ToPointer("2000"),
					},
					Updates: []parametersv1alpha1.ParameterUpdate{{
						Type: parametersv1alpha1.ParameterUpdateRemove,
						Key:  "max_connections",
					}},
				}
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&cfg.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.ConfigFileParams).Should(HaveKey("my.cnf"))
				decoded := parameters.DecodeParameterOverlay(item.ConfigFileParams["my.cnf"].Parameters)
				g.Expect(decoded).Should(HaveKey("max_connections"))
				g.Expect(decoded["max_connections"]).Should(BeNil())
			})).Should(Succeed())

			configKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
			checkRemoval := func(g Gomega, cm *corev1.ConfigMap) {
				config, err := unstructured.LoadConfig(testparameters.MysqlConfigFile, cm.Data[testparameters.MysqlConfigFile], parametersv1alpha1.Ini)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(config.Get("mysqld.max_connections")).Should(BeNil())
				g.Expect(config.Get("mysqld.gtid_mode")).Should(Equal("OFF"))
			}
			Eventually(testapps.CheckObj(&testCtx, configKey, checkRemoval)).Should(Succeed())

			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(obj *appsv1.Component) {
				obj.Spec.Replicas = 2
			})).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), comp)).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, configKey, func(g Gomega, cm *corev1.ConfigMap) {
				g.Expect(cm.Annotations[constant.ParametersAppliedComponentGenerationKey]).Should(Equal(strconv.FormatInt(comp.Generation, 10)))
				checkRemoval(g, cm)
			})).Should(Succeed())
		})

		It("should project desired unmanaged updates into file content", func() {
			_, _, _, _, _ = mockReconcileResource()

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				cfg.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{
						"max_connections": cfgutil.ToPointer("2000"),
					},
					UnmanagedUpdates: []parametersv1alpha1.UnmanagedParameterUpdate{{
						Template: configSpecName,
						File:     testparameters.MysqlConfigFile,
						Updates: []parametersv1alpha1.UnmanagedParameterSectionUpdate{{
							Section: cfgutil.ToPointer("mysqld"),
							Updates: []parametersv1alpha1.ParameterUpdate{
								{
									Type:  parametersv1alpha1.ParameterUpdateSet,
									Key:   "custom_local",
									Value: cfgutil.ToPointer("on"),
								},
								{
									Type: parametersv1alpha1.ParameterUpdateRemove,
									Key:  "gtid_mode",
								},
							},
						}},
					}},
				}
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&cfg.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.ConfigFileParams).Should(HaveKey(testparameters.MysqlConfigFile))
				fileParams := item.ConfigFileParams[testparameters.MysqlConfigFile]
				g.Expect(fileParams.UnmanagedUpdates).Should(HaveLen(1))
				g.Expect(fileParams.UnmanagedUpdates[0].Section).Should(Equal(cfgutil.ToPointer("mysqld")))
				g.Expect(fileParams.UnmanagedUpdates[0].Updates).Should(HaveLen(2))
				g.Expect(fileParams.Parameters).Should(HaveKeyWithValue("max_connections", cfgutil.ToPointer("2000")))
			})).Should(Succeed())
			configKey := client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
			Eventually(testapps.CheckObj(&testCtx, configKey, func(g Gomega, cm *corev1.ConfigMap) {
				config, err := unstructured.LoadConfig(testparameters.MysqlConfigFile, cm.Data[testparameters.MysqlConfigFile], parametersv1alpha1.Ini)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(config.Get("mysqld.gtid_mode")).Should(BeNil())
				g.Expect(config.Get("mysqld.custom_local")).Should(Equal("on"))
				g.Expect(config.Get("mysqld.max_connections")).Should(Equal("2000"))
			})).Should(Succeed())
		})

		It("should render both new PD and legacy PCR files in mixed mode", func() {
			const legacyFile = "log.conf"
			mockReconcileResource(func(templateObj *corev1.ConfigMap, compDefObj *appsv1.ComponentDefinition) {
				By("prepare the legacy PCR compatibility resources before creating the Component")
				Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(templateObj), func(tpl *corev1.ConfigMap) {
					tpl.Data[legacyFile] = "slow_query_log=1\n"
				})).Should(Succeed())

				legacyPD := testparameters.NewParametersDefinitionFactory("legacy-log-params").
					SetConfigFile(legacyFile).
					Create(&testCtx).
					GetObject()
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(legacyPD), func(obj *parametersv1alpha1.ParametersDefinition) {
					obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
				})()).Should(Succeed())

				pcr := &parametersv1alpha1.ParamConfigRenderer{
					ObjectMeta: metav1.ObjectMeta{Name: pdcrName + "-mixed"},
					Spec: parametersv1alpha1.ParamConfigRendererSpec{
						ComponentDef:   compDefObj.Name,
						ServiceVersion: "8.0.30",
						ParametersDefs: []string{legacyPD.Name},
						Configs: []parametersv1alpha1.ComponentConfigDescription{{
							Name:         legacyFile,
							TemplateName: configSpecName,
							FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
								Format: parametersv1alpha1.Properties,
							},
						}},
					},
				}
				Expect(testCtx.CreateObj(testCtx.Ctx, pcr)).Should(Succeed())

				By("wait until the controller cache resolves both new and legacy parameter bindings")
				Eventually(func(g Gomega) {
					cachedTemplate := &corev1.ConfigMap{}
					g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(templateObj), cachedTemplate)).Should(Succeed())
					g.Expect(cachedTemplate.Data).Should(HaveKeyWithValue(legacyFile, "slow_query_log=1\n"))

					cachedCompDef := &appsv1.ComponentDefinition{}
					g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(compDefObj), cachedCompDef)).Should(Succeed())
					configDescs, paramsDefs, err := parameters.ResolveCmpdParametersDefs(testCtx.Ctx, testCtx.Cli, cachedCompDef)
					g.Expect(err).ShouldNot(HaveOccurred())
					g.Expect(configDescs).Should(HaveLen(2))
					g.Expect(paramsDefs).Should(HaveLen(2))
					g.Expect(configDescs).Should(ContainElement(HaveField("Name", legacyFile)))
					g.Expect(paramsDefs).Should(ContainElement(HaveField("Name", legacyPD.Name)))
				}).Should(Succeed())
			})

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
				g.Expect(parameters.GetConfigTemplateItem(&cfg.Spec, configSpecName)).ShouldNot(BeNil())
			})).Should(Succeed())

			configKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GetComponentCfgName(clusterName, defaultCompName, configSpecName),
			}
			Eventually(testapps.CheckObj(&testCtx, configKey, func(g Gomega, cfg *corev1.ConfigMap) {
				g.Expect(cfg.Data).Should(HaveKey(testparameters.MysqlConfigFile))
				g.Expect(cfg.Data).Should(HaveKeyWithValue(legacyFile, "slow_query_log=1\n"))
			})).Should(Succeed())
		})
	})
})

func TestComponentParameterReconcileRetriesMissingService(t *testing.T) {
	const namespace = "service-retry"
	template := testparameters.NewComponentTemplateFactory(configSpecName, namespace).
		AddConfigFile(testparameters.MysqlConfigFile, "[mysqld]\netcd_host={{ $.component.serviceReferences.etcd.spec.host.value }}\n").
		GetObject()
	compDef := testapps.NewComponentDefinitionFactory(compDefName).
		SetDefaultSpec().
		AddConfigTemplate(configSpecName, template.Name, namespace, configVolumeName, true).
		AddServiceRef("etcd", "etcd", "3.5").
		GetObject()
	compDef.Status.Phase = appsv1.AvailablePhase
	paramsDef := testparameters.NewParametersDefinitionFactory(paramsDefName).
		SetComponentDefinition(compDef.Name).
		SetTemplateName(configSpecName).
		GetObject()
	paramsDef.Status.Phase = parametersv1alpha1.PDAvailablePhase
	serviceRef := appsv1.ServiceRef{
		Name: "etcd",
		ClusterServiceSelector: &appsv1.ServiceRefClusterSelector{
			Cluster: "etcd",
			Service: &appsv1.ServiceRefServiceSelector{Service: "client", Port: "client"},
		},
	}
	cluster := testapps.NewClusterFactory(namespace, clusterName, "").
		AddComponent(defaultCompName, compDef.Name).
		SetReplicas(1).
		SetServiceRefs([]appsv1.ServiceRef{serviceRef}).
		GetObject()
	comp := testapps.NewComponentFactory(namespace, constant.GenerateClusterComponentName(clusterName, defaultCompName), compDef.Name).
		AddLabels(constant.AppInstanceLabelKey, clusterName).
		AddAnnotations(constant.KBAppClusterUIDKey, "cluster-uid").
		SetReplicas(1).
		GetObject()
	comp.Spec.ServiceRefs = []appsv1.ServiceRef{serviceRef}
	compParam := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{
			Name:       core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			Namespace:  namespace,
			UID:        "parameter-uid",
			Generation: 1,
		},
		Spec: parametersv1alpha1.ComponentParameterSpec{ClusterName: clusterName, ComponentName: defaultCompName},
	}
	reconciler := newComponentParameterRetryReconciler(t, template, compDef, paramsDef, cluster, comp, compParam)
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(compParam)}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("initialize config items: %v", err)
	}
	if err := reconciler.Get(ctx, req.NamespacedName, compParam); err != nil {
		t.Fatal(err)
	}
	if len(compParam.Spec.ConfigItemDetails) != 1 {
		t.Fatalf("expected initialized config item, got %#v", compParam.Spec.ConfigItemDetails)
	}
	_, err := reconciler.Reconcile(ctx, req)
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected missing referenced Service to return a retryable NotFound error, got %v", err)
	}
	configKey := client.ObjectKey{Namespace: namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
	if err := reconciler.Get(ctx, configKey, &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected no rendered config before the referenced Service exists, got %v", err)
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: constant.GenerateClusterServiceName("etcd", "client")},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "client", Port: 2379}}},
	}
	if err := reconciler.Create(ctx, service); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("retry after referenced Service creation: %v", err)
	}
	rendered := &corev1.ConfigMap{}
	if err := reconciler.Get(ctx, configKey, rendered); err != nil {
		t.Fatalf("get rendered config after retry: %v", err)
	}
	if got, want := rendered.Data[testparameters.MysqlConfigFile], "[mysqld]\netcd_host=etcd-client\n"; got != want {
		t.Fatalf("rendered config = %q, want %q", got, want)
	}
}

func TestComponentParameterReconcileMissingTarget(t *testing.T) {
	reconciler := newComponentParameterRetryReconciler(t)
	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: "service-retry", Name: "deleted-component-parameter"},
	})
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("missing ComponentParameter should not retry, got result %v and error %v", result, err)
	}
}

func newComponentParameterRetryReconciler(t *testing.T, objects ...client.Object) *ComponentParameterReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme} {
		if err := addToScheme(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&parametersv1alpha1.ComponentParameter{}).
		WithObjects(objects...).Build()
	return &ComponentParameterReconciler{Client: cli, Scheme: scheme}
}
