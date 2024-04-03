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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ClusterDefinition Controller", func() {
	const (
		clusterDefName      = "test-clusterdef"
		clusterVersionName  = "test-clusterversion"
		compDefinitionName  = "test-component-definition"
		statefulCompDefName = "replicasets"

		configVolumeName = "mysql-config"

		cmName = "mysql-tree-node-template-8.0"
	)

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ClusterVersionSignature, true, ml)
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

	assureCfgTplConfigMapObj := func() *corev1.ConfigMap {
		By("Create a configmap and config template obj")
		cm := testapps.CreateCustomizedObj(&testCtx, "config/config-template.yaml", &corev1.ConfigMap{},
			testCtx.UseDefaultNamespace())

		cfgTpl := testapps.CreateCustomizedObj(&testCtx, "config/config-constraint.yaml",
			&appsv1beta1.ConfigConstraint{})
		Expect(testapps.ChangeObjStatus(&testCtx, cfgTpl, func() {
			cfgTpl.Status.Phase = appsv1beta1.CCAvailablePhase
		})).Should(Succeed())
		return cm
	}

	Context("with no ConfigSpec", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(statefulCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("should update status of clusterVersion at the same time when updating clusterDefinition", func() {
			By("Check reconciled finalizer and status of ClusterDefinition")
			var cdGen int64
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
					g.Expect(cd.Finalizers).NotTo(BeEmpty())
					g.Expect(cd.Status.ObservedGeneration).To(BeEquivalentTo(1))
					cdGen = cd.Status.ObservedGeneration
				})).Should(Succeed())

			By("Check reconciled finalizer and status of ClusterVersion")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersionObj),
				func(g Gomega, cv *appsv1alpha1.ClusterVersion) {
					g.Expect(cv.Finalizers).NotTo(BeEmpty())
					g.Expect(cv.Status.ObservedGeneration).To(BeEquivalentTo(1))
					g.Expect(cv.Status.ClusterDefGeneration).To(Equal(cdGen))
				})).Should(Succeed())

			By("updating clusterDefinition's spec which then update clusterVersion's status")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(cd *appsv1alpha1.ClusterDefinition) {
					cd.Spec.ConnectionCredential["root"] = "password"
				})).Should(Succeed())

			By("Check ClusterVersion.Status as updated")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersionObj),
				func(g Gomega, cv *appsv1alpha1.ClusterVersion) {
					g.Expect(cv.Status.Phase).To(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cv.Status.Message).To(Equal(""))
					g.Expect(cv.Status.ClusterDefGeneration > cdGen).To(BeTrue())
				})).Should(Succeed())

			// TODO: update components to break @validateClusterVersion, and transit ClusterVersion.Status.Phase to UnavailablePhase
		})
	})

	Context("with ConfigSpec", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
				AddConfigTemplate(cmName, cmName, cmName, testCtx.DefaultNamespace, configVolumeName).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(statefulCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				Create(&testCtx).GetObject()
		})

		It("should stop proceeding the status of clusterDefinition if configmap is invalid or doesn't exist", func() {
			By("check the reconciler set the status phase as unavailable if configmap doesn't exist.")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
					g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
					g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())

			assureCfgTplConfigMapObj()

			By("check the reconciler update Status.ObservedGeneration after configmap is created.")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(g Gomega, cd *appsv1alpha1.ClusterDefinition) {
					g.Expect(cd.Status.ObservedGeneration).Should(Equal(cd.Generation))
					g.Expect(cd.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))

					// check labels and finalizers
					g.Expect(cd.Finalizers).ShouldNot(BeEmpty())
					configCMLabel := cfgcore.GenerateTPLUniqLabelKeyWithConfig(cmName)
					configConstraintLabel := cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(cmName)
					g.Expect(cd.Labels[configCMLabel]).Should(BeEquivalentTo(cmName))
					g.Expect(cd.Labels[configConstraintLabel]).Should(BeEquivalentTo(cmName))
				})).Should(Succeed())

			By("check the reconciler update configmap.Finalizer after configmap is created.")
			cmKey := types.NamespacedName{
				Namespace: testCtx.DefaultNamespace,
				Name:      cmName,
			}
			Eventually(testapps.CheckObj(&testCtx, cmKey, func(g Gomega, cmObj *corev1.ConfigMap) {
				g.Expect(controllerutil.ContainsFinalizer(cmObj, constant.ConfigFinalizerName)).To(BeTrue())
			})).Should(Succeed())
		})
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
