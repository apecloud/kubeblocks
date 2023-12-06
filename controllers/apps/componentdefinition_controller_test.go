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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ComponentDefinition Controller", func() {
	const (
		componentDefName = "test-componentdef"

		// configVolumeName = "mysql-config"
		// cmName           = "mysql-tree-node-template-8.0"
	)

	var (
		defaultActionHandler = &appsv1alpha1.LifecycleActionHandler{}
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

	checkObjectStatus := func(obj *appsv1alpha1.ComponentDefinition, expectedPhase appsv1alpha1.Phase) {
		By(fmt.Sprintf("checking the object as %s", strings.ToLower(string(expectedPhase))))
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(obj),
			func(g Gomega, cmpd *appsv1alpha1.ComponentDefinition) {
				g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
				g.Expect(cmpd.Status.Phase).Should(Equal(expectedPhase))
			})).Should(Succeed())
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
		It("reconcile empty object", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				Create(&testCtx).GetObject()

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
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddVolume("default", true, 85).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("enable volume protection w/ actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddVolume("default", true, 85).
				SetLifecycleAction("Readonly", defaultActionHandler).
				SetLifecycleAction("Readwrite", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.AvailablePhase)
		})
	})

	Context("services", func() {
		It("ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddService("default", "", 3306, "leader").
				AddService("readonly", "readonly", 3306, "follower").
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.AvailablePhase)
		})

		It("duplicate names", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddService("default", "", 3306, "leader").
				AddService("default", "readonly", 3306, "follower").
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("duplicate service names", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddService("default", "default", 3306, "leader").
				AddService("readonly", "default", 3306, "follower").
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("multiple default service names", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddService("default", "", 3306, "leader").
				AddService("readonly", "", 3306, "follower").
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("w/o port", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddServiceExt("default", "", corev1.ServiceSpec{}, "leader").
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("undefined role selector", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddService("default", "", 3306, "leader").
				AddService("readonly", "readonly", 3306, "follower").
				AddService("undefined", "undefined", 3306, "undefined").
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})
	})

	Context("system accounts", func() {
		It("provision accounts w/o actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(string(appsv1alpha1.AdminAccount), true, "create user").
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("w/ actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(string(appsv1alpha1.AdminAccount), true, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.AvailablePhase)
		})

		It("duplicate accounts", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(string(appsv1alpha1.AdminAccount), true, "create user").
				AddSystemAccount(string(appsv1alpha1.AdminAccount), false, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("multiple init accounts", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(string(appsv1alpha1.AdminAccount), true, "create user").
				AddSystemAccount(string(appsv1alpha1.ProbeAccount), true, "create user").
				AddSystemAccount(string(appsv1alpha1.MonitorAccount), false, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("multiple accounts should be ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(string(appsv1alpha1.AdminAccount), true, "create user").
				AddSystemAccount(string(appsv1alpha1.ProbeAccount), false, "create user").
				AddSystemAccount(string(appsv1alpha1.MonitorAccount), false, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.AvailablePhase)
		})
	})

	Context("replica roles", func() {
		It("ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				AddRole("learner", false, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.AvailablePhase)
		})

		It("duplicate roles", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				AddRole("learner", false, false).
				AddRole("learner", true, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})
	})
})
