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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	configcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
	"github.com/apecloud/kubeblocks/pkg/unstructured"
)

var _ = Describe("Parameter Controller", func() {

	var compParamKey types.NamespacedName
	var comp *component.SynthesizedComponent
	var clusterObj *appsv1.Cluster

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	prepareTestEnv := func() {
		_, _, clusterObj, _, comp = mockReconcileResource()
		compParamKey = types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      configcore.GenerateComponentConfigurationName(comp.ClusterName, comp.Name),
		}

		Eventually(testapps.CheckObj(&testCtx, compParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
			g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(1)))
		})).Should(Succeed())
	}

	Context("parameter update", func() {
		It("Should reconcile success", func() {
			prepareTestEnv()

			By("submit the parameter update request")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddParameters("innodb_buffer_pool_size", "1024M").
				AddParameters("max_connections", "100").
				Create(&testCtx).
				GetObject()

			By("check component parameter status")
			Eventually(testapps.CheckObj(&testCtx, compParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
				g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(2)))
				g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			By("check component parameters cr")
			Eventually(testapps.CheckObj(&testCtx, compParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&compParameter.Spec, configSpecName)
				Expect(item).ShouldNot(BeNil())
				Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("max_connections", pointer.String("100")))
				Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("innodb_buffer_pool_size", pointer.String("1024M")))
			})).Should(Succeed())

			By("the second update parameters")
			key = testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj = testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddParameters("max_connections", "2000").
				AddParameters("gtid_mode", "OFF").
				Create(&testCtx).
				GetObject()

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			By("check component parameter")
			Eventually(testapps.CheckObj(&testCtx, compParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&compParameter.Spec, configSpecName)
				Expect(item).ShouldNot(BeNil())
				Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("max_connections", pointer.String("2000")))
				Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("gtid_mode", pointer.String("OFF")))
			})).Should(Succeed())
		})

		It("ignores null parameter entries without discarding other updates", func() {
			prepareTestEnv()

			submit := func(entries []parametersv1alpha1.ComponentParametersSpec, phase parametersv1alpha1.ParameterPhase) {
				GinkgoHelper()
				key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
				request := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).GetObject()
				request.Spec.ComponentParameters = entries
				Expect(testCtx.CreateObj(testCtx.Ctx, request)).To(Succeed())
				Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, parameter *parametersv1alpha1.Parameter) {
					g.Expect(parameter.Status.ObservedGeneration).To(Equal(parameter.Generation))
					g.Expect(parameter.Status.Phase).To(Equal(phase), "parameter status: %s", parameter.Status.Message)
				})).Should(Succeed())
			}

			const ignored = "default_time_zone"
			const valid = "max_connections"
			configMapKey := client.ObjectKey{
				Namespace: comp.Namespace,
				Name:      configcore.GetComponentCfgName(comp.ClusterName, comp.Name, configSpecName),
			}
			expectValues := func(maxConnections string) {
				GinkgoHelper()
				Eventually(testapps.CheckObj(&testCtx, configMapKey, func(g Gomega, configMap *corev1.ConfigMap) {
					config, err := unstructured.LoadConfig(testparameters.MysqlConfigFile, configMap.Data[testparameters.MysqlConfigFile], parametersv1alpha1.Ini)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(config.Get("mysqld." + ignored)).To(Equal("+08:00"))
					g.Expect(config.Get("mysqld." + valid)).To(Equal(maxConnections))
				})).Should(Succeed())
			}

			submit([]parametersv1alpha1.ComponentParametersSpec{{
				ComponentName: comp.Name,
				Parameters:    parametersv1alpha1.ComponentParameters{ignored: pointer.String("+08:00"), valid: pointer.String("100")},
			}}, parametersv1alpha1.CFinishedPhase)
			expectValues("100")

			By("accepting an update that only supplies null")
			before := &corev1.ConfigMap{}
			Expect(k8sClient.Get(testCtx.Ctx, configMapKey, before)).To(Succeed())
			nullOnly := parametersv1alpha1.ComponentParameters{ignored: nil}
			submit([]parametersv1alpha1.ComponentParametersSpec{{ComponentName: comp.Name, Parameters: nullOnly}}, parametersv1alpha1.CFinishedPhase)
			submit([]parametersv1alpha1.ComponentParametersSpec{{ComponentName: comp.Name}}, parametersv1alpha1.CFinishedPhase)
			after := &corev1.ConfigMap{}
			Expect(k8sClient.Get(testCtx.Ctx, configMapKey, after)).To(Succeed())
			Expect(after.Data).To(Equal(before.Data))
			Expect(after.Annotations[constant.ConfigurationRevision]).To(Equal(before.Annotations[constant.ConfigurationRevision]))

			By("applying a valid update alongside null in the same component")
			submit([]parametersv1alpha1.ComponentParametersSpec{{
				ComponentName: comp.Name,
				Parameters:    parametersv1alpha1.ComponentParameters{ignored: nil, valid: pointer.String("200")},
			}}, parametersv1alpha1.CFinishedPhase)
			expectValues("200")

			By("applying an update for another component when one component only supplies null")
			submit([]parametersv1alpha1.ComponentParametersSpec{
				{ComponentName: shardingCompName, Parameters: nullOnly},
				{ComponentName: comp.Name, Parameters: parametersv1alpha1.ComponentParameters{valid: pointer.String("300")}},
			}, parametersv1alpha1.CFinishedPhase)
			expectValues("300")

			By("rejecting an unknown component even if it only supplies null")
			submit([]parametersv1alpha1.ComponentParametersSpec{{
				ComponentName: "unknown-component", Parameters: nullOnly,
			}}, parametersv1alpha1.CMergeFailedPhase)
		})

		const stringParameter = "default_time_zone"
		DescribeTable("applies explicit empty string values",
			func(updates []map[string]string) {
				prepareTestEnv()
				file := testparameters.MysqlConfigFile
				configMapKey := client.ObjectKey{
					Namespace: comp.Namespace,
					Name:      configcore.GetComponentCfgName(comp.ClusterName, comp.Name, configSpecName),
				}
				Eventually(testapps.CheckObj(&testCtx, configMapKey, func(g Gomega, configMap *corev1.ConfigMap) {
					config, err := unstructured.LoadConfig(file, configMap.Data[file], parametersv1alpha1.Ini)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(config.Get("mysqld." + stringParameter)).To(BeNil())
				})).Should(Succeed())

				expected := make(map[string]string)
				for _, update := range updates {
					key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
					request := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name)
					for name, value := range update {
						request.AddParameters(name, value)
						expected[name] = value
					}
					request.Create(&testCtx)

					Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, parameter *parametersv1alpha1.Parameter) {
						g.Expect(parameter.Status.Phase).To(Equal(parametersv1alpha1.CFinishedPhase))
					})).Should(Succeed())
					Eventually(testapps.CheckObj(&testCtx, configMapKey, func(g Gomega, configMap *corev1.ConfigMap) {
						config, err := unstructured.LoadConfig(file, configMap.Data[file], parametersv1alpha1.Ini)
						g.Expect(err).NotTo(HaveOccurred())
						for name, value := range expected {
							g.Expect(config.Get("mysqld."+name)).To(Equal(value), "rendered value for %s", name)
						}
					})).Should(Succeed())
				}
			},
			Entry("in standalone requests", []map[string]string{
				{stringParameter: ""},
				{stringParameter: "+08:00"},
				{stringParameter: ""},
			}),
			Entry("with other parameter updates", []map[string]string{
				{stringParameter: "", "max_connections": "100"},
				{stringParameter: "+08:00"},
				{stringParameter: "", "max_connections": "200"},
			}),
		)

		It("parameters validate fails", func() {
			prepareTestEnv()

			By("submit the parameter update request with invalid max_connection")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddParameters("max_connections", "-100").
				Create(&testCtx).
				GetObject()

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CMergeFailedPhase))
			})).Should(Succeed())
		})
	})

	Context("custom template update", func() {
		It("update user template", func() {
			prepareTestEnv()

			By("create custom template object")
			configmap := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
				WithRandomName().
				Create(&testCtx).
				GetObject()

			By("submit the custom template request")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddCustomTemplate(configSpecName, configmap.Name, configmap.Namespace).
				Create(&testCtx).
				GetObject()

			By("check component parameter status")
			Eventually(testapps.CheckObj(&testCtx, compParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
				g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(2)))
				g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			}), time.Second*10).Should(Succeed())

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		})

		It("custom template failed", func() {
			prepareTestEnv()

			By("submit the custom template request")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddCustomTemplate(configSpecName, "not-exist-tpl", testCtx.DefaultNamespace).
				Create(&testCtx).
				GetObject()

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CMergeFailedPhase))
			})).Should(Succeed())
		})
	})

	Context("sharding component parameter update", func() {
		It("Should reconcile success", func() {
			prepareTestEnv()

			By("Create sharding component objs")
			shardingCompSpecList, err := sharding.GenShardingCompSpecList4Test(testCtx.Ctx, k8sClient, clusterObj, &clusterObj.Spec.Shardings[0])
			Expect(err).ShouldNot(HaveOccurred())
			for _, spec := range shardingCompSpecList {
				shardingLabels := map[string]string{
					constant.AppInstanceLabelKey:       comp.ClusterName,
					constant.KBAppShardingNameLabelKey: shardingCompName,
				}
				By("create a sharding component: " + spec.Name)
				comp, err := component.BuildComponent(clusterObj, spec, shardingLabels, nil)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(testCtx.Create(testCtx.Ctx, comp)).Should(Succeed())

				shardingCompParamKey := types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      configcore.GenerateComponentConfigurationName(clusterObj.Name, spec.Name),
				}

				By("check ComponentParameters cr for sharding component : " + spec.Name)
				Eventually(testapps.CheckObj(&testCtx, shardingCompParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
					g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
					g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(1)))
				})).Should(Succeed())
			}

			By("submit the parameter update request")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, shardingCompName).
				AddParameters("innodb_buffer_pool_size", "1024M").
				AddParameters("max_connections", "100").
				Create(&testCtx).
				GetObject()

			for _, spec := range shardingCompSpecList {
				shardingCompParamKey := types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      configcore.GenerateComponentConfigurationName(clusterObj.Name, spec.Name),
				}
				By("check component parameter status")
				Eventually(testapps.CheckObj(&testCtx, shardingCompParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
					g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(2)))
					g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
				}), time.Second*10).Should(Succeed())

				By("check parameter status")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
					g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
				})).Should(Succeed())
			}
		})

		It("component name validate fails", func() {
			prepareTestEnv()

			By("submit the parameter update request with invalid max_connection")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, "invalid component").
				AddParameters("max_connections", "100").
				Create(&testCtx).
				GetObject()

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CMergeFailedPhase))
			})).Should(Succeed())
		})
	})

})
