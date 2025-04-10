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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ParameterExtension Controller", func() {

	matchConfig := func(comp *appsv1.ComponentSpec, name string) *appsv1.ClusterComponentConfig {
		for i, config := range comp.Configs {
			if config.Name != nil && *config.Name == name {
				return &comp.Configs[i]
			}
		}
		return nil
	}

	Context("When updating cluster configs", func() {
		BeforeEach(cleanEnv)

		AfterEach(cleanEnv)

		It("Should reconcile success", func() {
			_, _, clusterObj, compObj, _ := mockReconcileResource()

			By("check cm resource")
			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKey{Name: configcore.GetComponentCfgName(clusterObj.Name, defaultCompName, configSpecName), Namespace: clusterObj.Namespace}, &corev1.ConfigMap{}, true)).Should(Succeed())

			By("set external managed")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compObj), func(comp *appsv1.Component) {
				comp.Spec.Configs = []appsv1.ClusterComponentConfig{{
					Name:            pointer.String(configSpecName),
					ExternalManaged: pointer.Bool(true),
				}}
			})).Should(Succeed())

			By("check component external cm object")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compObj), func(g Gomega, component *appsv1.Component) {
				config := matchConfig(&component.Spec, configSpecName)
				g.Expect(config).ShouldNot(BeNil())
				g.Expect(config.ConfigMap).ShouldNot(BeNil())
				g.Expect(config.ConfigMap.Name).Should(BeEquivalentTo(configcore.GetComponentCfgName(clusterName, defaultCompName, configSpecName)))
			})).Should(Succeed())
		})

		It("Should reconcile success for sharding component", func() {
			_, _, clusterObj, _, _ := mockReconcileResource()

			By("Create sharding component objs")
			shardingCompSpecList, err := intctrlutil.GenShardingCompSpecList(testCtx.Ctx, k8sClient, clusterObj, &clusterObj.Spec.Shardings[0])
			Expect(err).ShouldNot(HaveOccurred())
			for _, spec := range shardingCompSpecList {
				shardingLabels := map[string]string{
					constant.AppInstanceLabelKey:       clusterObj.Name,
					constant.KBAppShardingNameLabelKey: shardingCompName,
				}
				By("create a sharding component: " + spec.Name)
				comp, err := component.BuildComponent(clusterObj, spec, shardingLabels, nil)
				comp.Spec.Configs = []appsv1.ClusterComponentConfig{{
					Name:            pointer.String(configSpecName),
					ExternalManaged: pointer.Bool(true),
				}}
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

				By("check cm resource")
				Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKey{Name: configcore.GetComponentCfgName(clusterObj.Name, spec.Name, configSpecName), Namespace: clusterObj.Namespace}, &corev1.ConfigMap{}, true)).Should(Succeed())
			}

			By("check component external cm object")
			for _, spec := range shardingCompSpecList {
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKey{Name: component.FullName(clusterName, spec.Name), Namespace: clusterObj.Namespace}, func(g Gomega, component *appsv1.Component) {
					config := matchConfig(&component.Spec, configSpecName)
					g.Expect(config).ShouldNot(BeNil())
					g.Expect(config.ConfigMap).ShouldNot(BeNil())
					g.Expect(config.ConfigMap.Name).Should(BeEquivalentTo(configcore.GetComponentCfgName(clusterName, spec.Name, configSpecName)))
				})).Should(Succeed())
			}
		})

	})
})
