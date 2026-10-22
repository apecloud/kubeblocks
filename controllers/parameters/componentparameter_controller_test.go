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
	"encoding/json"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
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
			_, _, _, _, _ = mockReconcileResourceWithInitial(&parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{
					"max_connections": cfgutil.ToPointer("1000"),
				},
			})

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
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

var _ = Describe("Initial immutable parameters", func() {
	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	initialInputs := func() *parametersv1alpha1.ParameterInputs {
		return &parametersv1alpha1.ParameterInputs{
			Assignments: map[string]*string{"max_connections": cfgutil.ToPointer("1000")},
		}
	}
	parameterKey := func() client.ObjectKey {
		return client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}
	}
	configKey := func() client.ObjectKey {
		return client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      core.GetComponentCfgName(clusterName, defaultCompName, configSpecName),
		}
	}
	createInitial := func() (*appsv1.Component, string) {
		_, _, comp, _, its := mockReconcileResourceWithInitial(initialInputs(), func(_ *corev1.ConfigMap, _ *appsv1.ComponentDefinition) {
			key := client.ObjectKey{Name: paramsDefName}
			Eventually(testapps.GetAndChangeObj(&testCtx, key, func(pd *parametersv1alpha1.ParametersDefinition) {
				pd.Spec.ImmutableParameters = []string{"max_connections"}
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, pd *parametersv1alpha1.ParametersDefinition) {
				g.Expect(pd.Spec.ImmutableParameters).Should(ConsistOf("max_connections"))
			})).Should(Succeed())
		})
		Eventually(testapps.CheckObj(&testCtx, parameterKey(), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
			g.Expect(cp.Spec.Initial).Should(Equal(initialInputs()))
			g.Expect(cp.Status.Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
		})).Should(Succeed())
		waitRenderedConfigHash(comp.Namespace, clusterName, defaultCompName, configSpecName, "max_connections=1000")
		return comp, its.Name
	}

	It("accepts an initial immutable value and preserves it across rerender and ConfigMap recreation", func() {
		comp, _ := createInitial()
		By("rerender the same initial inputs for successive component generations")
		for _, replicas := range []int32{2, 3} {
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(obj *appsv1.Component) {
				obj.Spec.Replicas = replicas
			})).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), comp)).Should(Succeed())
			generation := strconv.FormatInt(comp.Generation, 10)
			Eventually(testapps.CheckObj(&testCtx, configKey(), func(g Gomega, cm *corev1.ConfigMap) {
				g.Expect(cm.Annotations[constant.ParametersAppliedComponentGenerationKey]).Should(Equal(generation))
				g.Expect(cm.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("max_connections=1000"))
			})).Should(Succeed())
		}

		By("recreate a deleted managed ConfigMap with the same initial immutable value")
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, configKey(), cm)).Should(Succeed())
		oldUID := cm.UID
		Expect(k8sClient.Delete(ctx, cm)).Should(Succeed())
		By("complete ConfigMap removal to exercise recovery from a missing rendered resource")
		Eventually(testapps.GetAndChangeObj(&testCtx, configKey(), func(deleting *corev1.ConfigMap) {
			deleting.Finalizers = nil
		})).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, configKey(), func(g Gomega, recreated *corev1.ConfigMap) {
			g.Expect(recreated.UID).ShouldNot(Equal(oldUID))
			g.Expect(recreated.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("max_connections=1000"))
		})).Should(Succeed())
	})

	It("preserves initial immutable values when desired updates another parameter in the same file", func() {
		comp, itsName := createInitial()
		for _, value := range []string{"ON", "OFF"} {
			Eventually(testapps.GetAndChangeObj(&testCtx, parameterKey(), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{"gtid_mode": cfgutil.ToPointer(value)},
				}
			})).Should(Succeed())
			hash := waitRenderedConfigHash(comp.Namespace, clusterName, defaultCompName, configSpecName,
				"max_connections=1000", "gtid_mode="+value)
			mockReconfigureDone(comp.Namespace, itsName, configSpecName, hash)
			Eventually(testapps.CheckObj(&testCtx, parameterKey(), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Status.ObservedGeneration).Should(Equal(cp.Generation))
				g.Expect(cp.Status.Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		}
	})

	It("rejects desired set, remove, and same-value writes to an initial immutable parameter", func() {
		createInitial()
		for _, desired := range []*parametersv1alpha1.ParameterInputs{
			{Assignments: map[string]*string{"max_connections": cfgutil.ToPointer("2000")}},
			{Updates: []parametersv1alpha1.ParameterUpdate{{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "max_connections"}}},
			{Assignments: map[string]*string{"max_connections": cfgutil.ToPointer("1000")}},
		} {
			Eventually(testapps.GetAndChangeObj(&testCtx, parameterKey(), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired = desired
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, parameterKey(), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Status.ObservedGeneration).Should(Equal(cp.Generation))
				g.Expect(cp.Status.Phase).Should(Equal(parametersv1alpha1.CMergeFailedPhase))
				g.Expect(cp.Status.Message).Should(ContainSubstring("max_connections"))
				g.Expect(cp.Status.Message).Should(ContainSubstring("immutable"))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, configKey(), func(g Gomega, cm *corev1.ConfigMap) {
				g.Expect(cm.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("max_connections=1000"))
				g.Expect(cm.Data[testparameters.MysqlConfigFile]).ShouldNot(ContainSubstring("max_connections=2000"))
			})).Should(Succeed())
		}
	})

	It("rejects adding, changing, or removing Initial after ComponentParameter creation at the API", func() {
		for _, tc := range []struct {
			name    string
			initial *parametersv1alpha1.ParameterInputs
			next    *parametersv1alpha1.ParameterInputs
		}{
			{name: "add", next: initialInputs()},
			{name: "add-empty", next: &parametersv1alpha1.ParameterInputs{}},
			{name: "change", initial: initialInputs(), next: &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"max_connections": cfgutil.ToPointer("2000")},
			}},
			{name: "remove", initial: initialInputs()},
			{name: "clear", initial: initialInputs(), next: &parametersv1alpha1.ParameterInputs{}},
		} {
			cp := &parametersv1alpha1.ComponentParameter{
				ObjectMeta: metav1.ObjectMeta{Name: "initial-validation-" + tc.name, Namespace: testCtx.DefaultNamespace},
				Spec: parametersv1alpha1.ComponentParameterSpec{
					ComponentName: "unbound-component", Initial: tc.initial,
				},
			}
			Expect(k8sClient.Create(ctx, cp)).Should(Succeed())
			patch, err := json.Marshal([]map[string]any{{"op": "add", "path": "/spec/initial", "value": tc.next}})
			Expect(err).ShouldNot(HaveOccurred())
			err = k8sClient.Patch(ctx, cp, client.RawPatch(types.JSONPatchType, patch))
			Expect(apierrors.IsInvalid(err)).Should(BeTrue(), "Initial %s must be rejected by API validation, got: %v", tc.name, err)
		}
	})
})
