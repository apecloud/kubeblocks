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

package parameters

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ComponentParameter Controller", func() {

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	updatePDCRForInjectEnv := func() {
		Eventually(testapps.GetAndChangeObj(&testCtx, types.NamespacedName{Name: pdcrName}, func(pdcr *parametersv1alpha1.ParameterDrivenConfigRender) {
			pdcr.Spec.Configs = append(pdcr.Spec.Configs, parametersv1alpha1.ComponentConfigDescription{
				Name:         envTestFileKey,
				TemplateName: configSpecName,
				InjectEnvTo:  []string{testapps.DefaultMySQLContainerName},
				FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
					Format: parametersv1alpha1.Properties,
				},
			})
		})).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Name: pdcrName}, func(g Gomega, pdcr *parametersv1alpha1.ParameterDrivenConfigRender) {
			g.Expect(pdcr.Spec.Configs).Should(HaveLen(2))
			g.Expect(pdcr.Spec.Configs[1].FileFormatConfig.Format).Should(BeEquivalentTo(parametersv1alpha1.Properties))
		})).Should(Succeed())

	}

	Context("When updating configuration", func() {
		It("Should reconcile success", func() {
			mockReconcileResource()

			cfgKey := client.ObjectKey{
				Name:      core.GenerateComponentParameterName(clusterName, defaultCompName),
				Namespace: testCtx.DefaultNamespace,
			}

			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, componentParameter *parametersv1alpha1.ComponentParameter) {
				g.Expect(componentParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
				itemStatus := intctrlutil.GetItemStatus(&componentParameter.Status, configSpecName)
				g.Expect(itemStatus.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			By("reconfiguring parameters.")
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				item := intctrlutil.GetConfigTemplateItem(&cfg.Spec, configSpecName)
				item.ConfigFileParams = map[string]parametersv1alpha1.ParametersInFile{
					"my.cnf": {
						Parameters: map[string]*string{
							"max_connections": cfgutil.ToPointer("1000"),
							"gtid_mode":       cfgutil.ToPointer("ON"),
						},
					},
				}
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				itemStatus := intctrlutil.GetItemStatus(&cfg.Status, configSpecName)
				g.Expect(itemStatus).ShouldNot(BeNil())
				g.Expect(itemStatus.UpdateRevision).Should(BeEquivalentTo("2"))
				g.Expect(itemStatus.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		})

	})

	Context("When updating configuration with injectEnvTo", func() {
		It("Should reconcile success", func() {
			mockReconcileResource()
			updatePDCRForInjectEnv()

			cfgKey := client.ObjectKey{
				Name:      core.GenerateComponentParameterName(clusterName, defaultCompName),
				Namespace: testCtx.DefaultNamespace,
			}
			envKey := client.ObjectKey{
				Name:      core.GenerateEnvFromName(core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)),
				Namespace: testCtx.DefaultNamespace,
			}

			envObj := &corev1.ConfigMap{}
			Eventually(testapps.CheckObjExists(&testCtx, envKey, envObj, true)).Should(Succeed())

			By("reconfiguring parameters.")
			Eventually(testapps.GetAndChangeObj(&testCtx, cfgKey, func(cfg *parametersv1alpha1.ComponentParameter) {
				item := intctrlutil.GetConfigTemplateItem(&cfg.Spec, configSpecName)
				item.ConfigFileParams = map[string]parametersv1alpha1.ParametersInFile{
					envTestFileKey: {
						Parameters: map[string]*string{
							"max_connections": cfgutil.ToPointer("1000"),
							"gtid_mode":       cfgutil.ToPointer("ON"),
						},
					},
				}
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				itemStatus := intctrlutil.GetItemStatus(&cfg.Status, configSpecName)
				g.Expect(itemStatus).ShouldNot(BeNil())
				g.Expect(itemStatus.UpdateRevision).Should(BeEquivalentTo("2"))
				g.Expect(itemStatus.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, envKey, func(g Gomega, envObj *corev1.ConfigMap) {
				g.Expect(envObj.Data).Should(HaveKeyWithValue("max_connections", "1000"))
				g.Expect(envObj.Data).Should(HaveKeyWithValue("gtid_mode", "ON"))
			})).Should(Succeed())
		})

	})
})
