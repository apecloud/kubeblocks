/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("ComponentDefinition Controller", func() {
	const (
		componentDefName = "test-componentdef"

		// configVolumeName = "mysql-config"
		// cmName           = "mysql-tree-node-template-8.0"
	)

	var (
		componentDefObj *appsv1alpha1.ComponentDefinition

		defaultAction = &appsv1alpha1.Action{}
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ComponentDefinitionSignature, true, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)

		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ConfigMapSignature, true, inNS, ml)
	}

	// assureCfgTplConfigMapObj := func() *corev1.ConfigMap {
	//	By("Create a configmap and config template obj")
	//	cm := testapps.CreateCustomizedObj(&testCtx, "config/config-template.yaml", &corev1.ConfigMap{}, testCtx.UseDefaultNamespace())
	//
	//	cfgTpl := testapps.CreateCustomizedObj(&testCtx, "config/config-constraint.yaml",
	//		&appsv1alpha1.ConfigConstraint{})
	//	Expect(testapps.ChangeObjStatus(&testCtx, cfgTpl, func() {
	//		cfgTpl.Status.Phase = appsv1alpha1.CCAvailablePhase
	//	})).Should(Succeed())
	//	return cm
	// }

	BeforeEach(func() {
		cleanEnv()

	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("default", func() {
		BeforeEach(func() {
			By("create a ComponentDefinition obj")
			componentDefObj = testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				Create(&testCtx).GetObject()
		})

		It("reconcile empty object", func() {
			By("checking the object reconciled")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *appsv1alpha1.ComponentDefinition) {
					g.Expect(cmpd.Finalizers).ShouldNot(BeEmpty())
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})

	// Context("with config spec", func() {
	//	BeforeEach(func() {
	//		By("create a ComponentDefinition obj")
	//		componentDefObj = testapps.NewComponentDefinitionFactory(componentDefName).
	//			SetRuntime(nil).
	//			SetConfigTemplate(cmName, cmName, cmName, testCtx.DefaultNamespace, configVolumeName).
	//			Create(&testCtx).GetObject()
	//	})
	//
	//	It("should stop proceeding the status of clusterDefinition if configmap is invalid or doesn't exist", func() {
	//		By("check the reconciler won't update Status.ObservedGeneration if configmap doesn't exist.")
	//		// should use Consistently here, since cd.Status.ObservedGeneration is initialized to be zero,
	//		// we must watch the value for a while to tell it's not changed by the reconciler.
	//		Consistently(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
	//			func(g Gomega, cmpd *appsv1alpha1.ComponentDefinition) {
	//				g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(int64(0)))
	//			})).Should(Succeed())
	//
	//		assureCfgTplConfigMapObj()
	//
	//		By("check the reconciler update Status.ObservedGeneration after configmap is created.")
	//		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
	//			func(g Gomega, cmpd *appsv1alpha1.ComponentDefinition) {
	//				g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(int64(1)))
	//				// check labels and finalizers
	//				g.Expect(cmpd.Finalizers).ShouldNot(BeEmpty())
	//				configCMLabel := cfgcore.GenerateTPLUniqLabelKeyWithConfig(cmName)
	//				configConstraintLabel := cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(cmName)
	//				g.Expect(cmpd.Labels[configCMLabel]).Should(Equal(cmName))
	//				g.Expect(cmpd.Labels[configConstraintLabel]).Should(Equal(cmName))
	//			})).Should(Succeed())
	//
	//		By("check the reconciler update configmap.Finalizer after configmap is created.")
	//		cmKey := types.NamespacedName{
	//			Namespace: testCtx.DefaultNamespace,
	//			Name:      cmName,
	//		}
	//		Eventually(testapps.CheckObj(&testCtx, cmKey, func(g Gomega, cmObj *corev1.ConfigMap) {
	//			g.Expect(controllerutil.ContainsFinalizer(cmObj, constant.ConfigurationTemplateFinalizerName)).Should(BeTrue())
	//		})).Should(Succeed())
	//	})
	// })

	Context("volumes", func() {
		It("enable volume protection w/o actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj = testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				SetVolume("default", true, 85).
				Create(&testCtx).GetObject()

			By("checking the object as unavailable")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *appsv1alpha1.ComponentDefinition) {
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())
		})

		It("enable volume protection w/ actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj = testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				SetVolume("default", true, 85).
				SetLifecycleAction("Readonly", defaultAction).
				SetLifecycleAction("Readwrite", defaultAction).
				Create(&testCtx).GetObject()

			By("checking the object as available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *appsv1alpha1.ComponentDefinition) {
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})

	Context("system accounts", func() {
		It("provision accounts w/o actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj = testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount("kubeblocks").
				Create(&testCtx).GetObject()

			By("checking the object as unavailable")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *appsv1alpha1.ComponentDefinition) {
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())
		})

		It("provision accounts w/ actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj = testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount("kubeblocks").
				SetLifecycleAction("AccountProvision", defaultAction).
				Create(&testCtx).GetObject()

			By("checking the object as available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *appsv1alpha1.ComponentDefinition) {
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})
})
