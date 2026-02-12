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

	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
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
				g.Expect(status.UpdateRevision).Should(BeEquivalentTo("2"))
				g.Expect(status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CUpgradingPhase))
			})).Should(Succeed())

			By("mock the reconfigure done")
			mockReconfigureDone(itsObj.Namespace, itsObj.Name, configSpecName, configHash3)

			By("check component parameter status is updated to Finished")
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *parametersv1alpha1.ComponentParameter) {
				status := parameters.GetItemStatus(&cfg.Status, configSpecName)
				g.Expect(status).ShouldNot(BeNil())
				g.Expect(status.UpdateRevision).Should(BeEquivalentTo("2"))
				g.Expect(status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		})
	})
})
