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

package apps

import (
	"math/rand"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ClusterDefinition Controller", func() {
	const (
		clusterDefName     = "test-clusterdef"
		compDefinitionName = "test-component-definition"
	)

	var (
		clusterDefObj *appsv1alpha1.ClusterDefinition
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// resources should be released in following order
		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ClusterDefinitionSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ComponentDefinitionSignature, true, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)

		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ConfigMapSignature, true, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("cluster topology", func() {
		var (
			singleCompTopology = appsv1alpha1.ClusterTopology{
				Name:    "topo1",
				Default: true,
				Components: []appsv1alpha1.ClusterTopologyComponent{
					{
						Name:    "server",
						CompDef: compDefinitionName,
					},
				},
				Orders: &appsv1alpha1.ClusterTopologyOrders{},
			}
			multipleCompsTopology = appsv1alpha1.ClusterTopology{
				Name:    "topo2",
				Default: false,
				Components: []appsv1alpha1.ClusterTopologyComponent{
					{
						Name:    "proxy",
						CompDef: compDefinitionName,
					},
					{
						Name:    "server",
						CompDef: compDefinitionName,
					},
					{
						Name:    "storage",
						CompDef: compDefinitionName,
					},
				},
				Orders: &appsv1alpha1.ClusterTopologyOrders{
					Provision: []string{"storage", "server", "proxy"},
					Update:    []string{"storage", "server", "proxy"},
				},
			}
		)

		BeforeEach(func() {
			By("create a ComponentDefinition obj")
			compDefObj := testapps.NewComponentDefinitionFactory(compDefinitionName).
				SetRuntime(nil).
				AddServiceRef("service-1", "service-1", "v1").
				AddServiceRef("service-2", "service-2", "v2").
				Create(&testCtx).
				GetObject()
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compDefObj), func(g Gomega, compDef *appsv1alpha1.ComponentDefinition) {
				g.Expect(compDef.Status.ObservedGeneration).Should(Equal(compDef.Generation))
				g.Expect(compDef.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
			})).Should(Succeed())

			By("Create a ClusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddClusterTopology(singleCompTopology).
				AddClusterTopology(multipleCompsTopology).
				Create(&testCtx).
				GetObject()
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
				g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
				g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
			})).Should(Succeed())
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("ok", func() {
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
				g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
				g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
			})).Should(Succeed())
		})

		It("duplicate topology", func() {
			By("update cd to add a topology with same name")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(cd *appsv1alpha1.ClusterDefinition) {
				cd.Spec.Topologies = append(cd.Spec.Topologies, singleCompTopology)
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
				g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
				g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				g.Expect(cd.Status.Message).Should(ContainSubstring("duplicate topology"))
			})).Should(Succeed())
		})

		It("multiple default topologies", func() {
			By("update cd to set all topologies as default")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(cd *appsv1alpha1.ClusterDefinition) {
				for i := range cd.Spec.Topologies {
					cd.Spec.Topologies[i].Default = true
				}
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
				g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
				g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				g.Expect(cd.Status.Message).Should(ContainSubstring("multiple default topologies"))
			})).Should(Succeed())
		})

		It("duplicate topology component", func() {
			By("update cd to set all component names as same")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(cd *appsv1alpha1.ClusterDefinition) {
				compName := cd.Spec.Topologies[0].Components[0].Name
				for i, topology := range cd.Spec.Topologies {
					for j := range topology.Components {
						cd.Spec.Topologies[i].Components[j].Name = compName
					}
				}
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
				g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
				g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				g.Expect(cd.Status.Message).Should(ContainSubstring("duplicate topology component"))
			})).Should(Succeed())
		})

		It("different components in topology orders", func() {
			By("update cd to add/remove components in orders")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(cd *appsv1alpha1.ClusterDefinition) {
				for i := range cd.Spec.Topologies {
					update := func(orders []string) []string {
						if len(orders) == 0 {
							return orders
						}
						rand.Shuffle(len(orders), func(m, n int) {
							orders[m], orders[n] = orders[n], orders[m]
						})
						return append(orders[1:], "comp-non-exist")
					}
					topology := cd.Spec.Topologies[i]
					if topology.Orders != nil {
						cd.Spec.Topologies[i].Orders.Provision = update(topology.Orders.Provision)
						cd.Spec.Topologies[i].Orders.Terminate = update(topology.Orders.Terminate)
						cd.Spec.Topologies[i].Orders.Update = update(topology.Orders.Update)
					}
				}
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
				g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
				g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				g.Expect(cd.Status.Message).Should(MatchRegexp("the components in provision|terminate|update orders are different from those in definition"))
			})).Should(Succeed())
		})

		It("topology component has no matched definitions", func() {
			By("update cd to set a non-exist compdef for the first topology and component")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(cd *appsv1alpha1.ClusterDefinition) {
				cd.Spec.Topologies[0].Components[0].CompDef = "compdef-non-exist"
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj), func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
				g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
				g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				g.Expect(cd.Status.Message).Should(ContainSubstring("there is no matched definitions found for the topology component"))
			})).Should(Succeed())
		})
	})
})
