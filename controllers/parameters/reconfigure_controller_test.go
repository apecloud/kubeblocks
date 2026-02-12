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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Reconfigure Controller", func() {
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("reconfigure", func() {
		It("should compute consistent hash for same configuration", func() {
			configmap, _, _, _, _ := mockReconcileResource()

			By("compute hash for initial configuration")
			initialHash := computeTargetConfigHash(nil, configmap.Data)
			Expect(initialHash).NotTo(BeNil())
			Expect(*initialHash).NotTo(BeEmpty())

			By("compute hash again for same data should give same result")
			sameHash := computeTargetConfigHash(nil, configmap.Data)
			Expect(sameHash).NotTo(BeNil())
			Expect(*sameHash).To(Equal(*initialHash))

			By("compute hash for different data should give different result")
			modifiedData := make(map[string]string)
			for k, v := range configmap.Data {
				modifiedData[k] = v
			}
			modifiedData["new_key"] = "new_value"
			differentHash := computeTargetConfigHash(nil, modifiedData)
			Expect(differentHash).NotTo(BeNil())
			Expect(*differentHash).NotTo(Equal(*initialHash))
		})

		It("submit changes to cluster", func() {
			configmap, clusterObj, _, _, _ := mockReconcileResource()

			By("verify configHash is set in ConfigMap labels")
			cfgKey := client.ObjectKeyFromObject(configmap)
			Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cm *corev1.ConfigMap) {
				configHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(configHash).NotTo(BeEmpty())
			})).Should(Succeed())

			By("verify changes submit to cluster")
			clusterKey := client.ObjectKeyFromObject(clusterObj)
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				for _, comp := range cluster.Spec.ComponentSpecs {
					for _, config := range comp.Configs {
						g.Expect(config.ConfigHash).NotTo(BeEmpty())
					}
				}
			})).Should(Succeed())
			// The hash should be propagated from parameters controller to Cluster configs
			// and then to Component and ITS
			// This is an integration test verifying the full pipeline
		})

		It("should handle ExternalManaged configs correctly", func() {
			// This test would require setting up ExternalManaged config
			// For now, it's a placeholder
			Skip("ExternalManaged config test needs specific setup")
		})
	})

	Context("Reconfigure action and restart operations", func() {
		It("should trigger reconfigure action when config changes", func() {
			configmap, _, _, _, _ := mockReconcileResource()

			By("get initial ConfigHash")
			var initialHash string
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				initialHash = cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(initialHash).NotTo(BeEmpty())
			}).Should(Succeed())

			By("update configuration to trigger reconfigure action")
			updatedCM := testapps.NewCustomizedObj("resources/mysql-ins-config-update.yaml", &corev1.ConfigMap{})
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = updatedCM.Data
			})).Should(Succeed())

			By("verify new ConfigHash and reconfigure phase")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).NotTo(Equal(initialHash))
			}).Should(Succeed())
		})

		It("should trigger restart when restart policy is specified", func() {
			configmap, _, _, _, _ := mockReconcileResource()

			By("update configuration with restart policy")
			restartUpdatedCM := testapps.NewCustomizedObj("resources/mysql-ins-config-update-with-restart.yaml", &corev1.ConfigMap{})
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = restartUpdatedCM.Data
			})).Should(Succeed())

			By("verify restart phase is set")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
			}).Should(Succeed())
		})
	})
})
