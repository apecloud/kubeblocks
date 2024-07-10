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
		adminAccount     = "kbadmin"
		probeAccount     = "kbprobe"
		monitorAccount   = "kbmonitoring"
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

	Context("host network", func() {
		It("ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddHostNetworkContainerPort(testapps.DefaultMySQLContainerName, []string{"mysql"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.AvailablePhase)
		})

		It("duplicate containers", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddHostNetworkContainerPort(testapps.DefaultMySQLContainerName, []string{"mysql"}).
				AddHostNetworkContainerPort(testapps.DefaultMySQLContainerName, []string{"mysql"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("undefined container", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddHostNetworkContainerPort("non-exist-container", []string{"mysql"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("undefined container port", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddHostNetworkContainerPort(testapps.DefaultMySQLContainerName, []string{"non-exist-port"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
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
				AddSystemAccount(adminAccount, false, "create user").
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("w/ actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(adminAccount, true, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.AvailablePhase)
		})

		It("duplicate accounts", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(adminAccount, true, "create user").
				AddSystemAccount(adminAccount, false, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.UnavailablePhase)
		})

		It("multiple init accounts", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(adminAccount, true, "create user").
				AddSystemAccount(probeAccount, true, "create user").
				AddSystemAccount(monitorAccount, false, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, appsv1alpha1.AvailablePhase)
		})

		It("multiple accounts should be ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(adminAccount, true, "create user").
				AddSystemAccount(probeAccount, false, "create user").
				AddSystemAccount(monitorAccount, false, "create user").
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
