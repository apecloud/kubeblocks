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

	Context("When updating configuration", func() {
		It("Should reconcile success", func() {
			mockReconcileResource()

			cfgKey := client.ObjectKey{
				Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
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
})
