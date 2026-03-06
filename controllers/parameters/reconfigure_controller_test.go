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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("Reconfigure Controller", func() {
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("reconfigure policy", func() {
		// TODO: impl
	})

	Context("reconfigure", func() {
		var (
			configmap                    *corev1.ConfigMap
			clusterObj                   *appsv1.Cluster
			synthesizedComp              *component.SynthesizedComponent
			clusterKey, compParameterKey types.NamespacedName
		)

		BeforeEach(func() {
			configmap, clusterObj, _, synthesizedComp, _ = mockReconcileResource()

			clusterKey = client.ObjectKeyFromObject(clusterObj)

			compParameterKey = types.NamespacedName{
				Namespace: synthesizedComp.Namespace,
				Name:      parameterscore.GenerateComponentConfigurationName(synthesizedComp.ClusterName, synthesizedComp.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, compParameterKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
				g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
				g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(1)))
			})).Should(Succeed())
		})

		It("compute config hash", func() {
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
			Expect(*differentHash).NotTo(BeEmpty())
			Expect(*differentHash).NotTo(Equal(*initialHash))
		})

		It("submit changes to cluster", func() {
			By("submit a parameter update request")
			key := testapps.GetRandomizedKey(synthesizedComp.Namespace, synthesizedComp.FullCompName)
			testparameters.NewParameterFactory(key.Name, key.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name).
				AddParameters("innodb_buffer_pool_size", "1024M").
				AddParameters("max_connections", "100").
				Create(&testCtx).
				GetObject()

			By("verify changes submit to cluster")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				for _, comp := range cluster.Spec.ComponentSpecs {
					for _, config := range comp.Configs {
						// g.Expect(config.Variables).Should(HaveKeyWithValue("innodb_buffer_pool_size", "1024M"))
						// g.Expect(config.Variables).Should(HaveKeyWithValue("max_connections", "100"))
						g.Expect(config.Variables).Should(BeNil())
						g.Expect(config.ConfigHash).ShouldNot(BeNil())
						g.Expect(*config.ConfigHash).Should(Equal(configHash1))
						g.Expect(config.Restart).ShouldNot(BeNil())
						g.Expect(*config.Restart).Should(BeTrue())
						g.Expect(config.Reconfigure).Should(BeNil())
					}
				}
			})).Should(Succeed())
		})

		It("restart", func() {
			By("mock parameters definition")
			pdKey := types.NamespacedName{
				Namespace: "",
				Name:      paramsDefName,
			}
			Expect(testapps.GetAndChangeObj(&testCtx, pdKey, func(pd *parametersv1alpha1.ParametersDefinition) {
				pd.Spec.ReloadAction = nil // restart
			})()).Should(Succeed())

			By("submit a parameter update request")
			key := testapps.GetRandomizedKey(synthesizedComp.Namespace, synthesizedComp.FullCompName)
			testparameters.NewParameterFactory(key.Name, key.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name).
				AddParameters("innodb_buffer_pool_size", "1024M").
				AddParameters("max_connections", "100").
				Create(&testCtx).
				GetObject()

			By("verify changes submit to cluster")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				for _, comp := range cluster.Spec.ComponentSpecs {
					for _, config := range comp.Configs {
						// g.Expect(config.Variables).Should(HaveKeyWithValue("innodb_buffer_pool_size", "1024M"))
						// g.Expect(config.Variables).Should(HaveKeyWithValue("max_connections", "100"))
						g.Expect(config.Variables).Should(BeNil())
						g.Expect(config.ConfigHash).ShouldNot(BeNil())
						g.Expect(*config.ConfigHash).Should(Equal(configHash1))
						g.Expect(config.Restart).ShouldNot(BeNil())
						g.Expect(*config.Restart).Should(BeTrue())
						g.Expect(config.Reconfigure).Should(BeNil())
					}
				}
			})).Should(Succeed())
		})
	})

	Context("phase", func() {
		// TODO: impl
	})
})
