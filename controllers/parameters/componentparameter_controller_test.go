/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				cfg.Spec.Desired = &parametersv1alpha1.ParameterValues{
					Parameters: parametersv1alpha1.ParameterValueMap{
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

		It("should project init first and let desired override it", func() {
			_, _, _, _, _ = mockReconcileResource()

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				cfg.Spec.Init = &parametersv1alpha1.ParameterValues{
					Parameters: parametersv1alpha1.ParameterValueMap{
						"max_connections": cfgutil.ToPointer("1000"),
					},
				}
				cfg.Spec.Desired = &parametersv1alpha1.ParameterValues{
					Parameters: parametersv1alpha1.ParameterValueMap{
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

		It("should render both new PD and legacy PCR files in mixed mode", func() {
			templateObj, _, compObj, _, _ := mockReconcileResource()

			cfgKey := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				g.Expect(cfg.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			By("add a legacy-only file to the component template")
			const legacyFile = "log.conf"
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(templateObj), func(tpl *corev1.ConfigMap) {
				tpl.Data[legacyFile] = "slow_query_log=1\n"
			})).Should(Succeed())

			By("create a legacy-only ParametersDefinition and ParamConfigRenderer binding")
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
					ComponentDef:   compObj.Spec.CompDef,
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

			By("touch the component to regenerate ComponentParameter from mixed sources")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compObj), func(comp *appsv1.Component) {
				if comp.Annotations == nil {
					comp.Annotations = map[string]string{}
				}
				comp.Annotations["parameters.kubeblocks.io/mixed-mode-test"] = "true"
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
