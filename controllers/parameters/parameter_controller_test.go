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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
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
			}), time.Second*10).Should(Succeed())

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		})

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
			shardingCompSpecList, err := intctrlutil.GenShardingCompSpecList(testCtx.Ctx, k8sClient, clusterObj, &clusterObj.Spec.Shardings[0])
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
