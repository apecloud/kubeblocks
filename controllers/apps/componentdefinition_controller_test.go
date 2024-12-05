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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
		defaultActionHandler = &kbappsv1.Action{}
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

	checkObjectStatus := func(obj *kbappsv1.ComponentDefinition, expectedPhase kbappsv1.Phase) {
		By(fmt.Sprintf("checking the object as %s", strings.ToLower(string(expectedPhase))))
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(obj),
			func(g Gomega, cmpd *kbappsv1.ComponentDefinition) {
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
				func(g Gomega, cmpd *kbappsv1.ComponentDefinition) {
					g.Expect(cmpd.Finalizers).ShouldNot(BeEmpty())
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(kbappsv1.AvailablePhase))
				})).Should(Succeed())
		})
	})

	Context("volumes", func() {
		It("duplicate volumes", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddVolume("default", true, 0).
				AddVolume("default", true, 0).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})

		It("set volume high watermark", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddVolume("default", true, 85).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
		})
	})

	Context("host network", func() {
		It("ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddHostNetworkContainerPort(testapps.DefaultMySQLContainerName, []string{"mysql"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
		})

		It("duplicate containers", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddHostNetworkContainerPort(testapps.DefaultMySQLContainerName, []string{"mysql"}).
				AddHostNetworkContainerPort(testapps.DefaultMySQLContainerName, []string{"mysql"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})

		It("undefined container", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddHostNetworkContainerPort("non-exist-container", []string{"mysql"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})

		It("undefined container port", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddHostNetworkContainerPort(testapps.DefaultMySQLContainerName, []string{"non-exist-port"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})

		It("w/o port", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddServiceExt("default", "", corev1.ServiceSpec{}, "leader").
				AddRole("leader", true, true).
				AddRole("follower", true, false).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})
	})

	Context("system accounts", func() {
		It("provision accounts w/o actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(adminAccount, false, "create user").
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})

		It("w/ actions set", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(adminAccount, true, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
		})

		It("duplicate accounts", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddSystemAccount(adminAccount, true, "create user").
				AddSystemAccount(adminAccount, false, "create user").
				SetLifecycleAction("AccountProvision", defaultActionHandler).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
		})
	})

	Context("vars", func() {
		It("ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddVar(kbappsv1.EnvVar{
					Name:  "VAR1",
					Value: "value1",
				}).
				Create(&testCtx).GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
		})

		It("duplicate vars name", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddVar(kbappsv1.EnvVar{
					Name:  "VAR1",
					Value: "value1",
				}).
				AddVar(kbappsv1.EnvVar{
					Name:  "VAR1",
					Value: "value2",
				}).
				Create(&testCtx).GetObject()
			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})

		It("valid var component definition name", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddVar(kbappsv1.EnvVar{
					Name: "VAR1",
					ValueFrom: &kbappsv1.VarSource{
						ServiceRefVarRef: &kbappsv1.ServiceRefVarSelector{
							ClusterObjectReference: kbappsv1.ClusterObjectReference{
								Name:    "service",
								CompDef: "valid",
							},
						},
					},
				}).
				Create(&testCtx).GetObject()
			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
		})

		It("invalid var component definition name", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				AddVar(kbappsv1.EnvVar{
					Name: "VAR1",
					ValueFrom: &kbappsv1.VarSource{
						ServiceVarRef: &kbappsv1.ServiceVarSelector{
							ClusterObjectReference: kbappsv1.ClusterObjectReference{
								Name:    "service",
								CompDef: "(invalid",
							},
						},
					},
				}).
				Create(&testCtx).GetObject()
			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})
	})

	Context("available", func() {
		It("with phases - ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				SetAvailable(&kbappsv1.ComponentAvailable{
					WithPhases: pointer.String(fmt.Sprintf("%s,%s",
						string(kbappsv1.RunningComponentPhase), string(kbappsv1.UpdatingComponentPhase))),
				}).
				Create(&testCtx).
				GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
		})

		It("with phases - fail", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				SetAvailable(&kbappsv1.ComponentAvailable{
					// empty phase
					WithPhases: pointer.String(fmt.Sprintf("%s,%s", string(kbappsv1.RunningComponentPhase), "")),
				}).
				Create(&testCtx).
				GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})

		It("with probe - ok", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				SetAvailable(&kbappsv1.ComponentAvailable{
					WithProbe: &kbappsv1.ComponentAvailableWithProbe{},
				}).
				SetLifecycleAction("availableProbe", &kbappsv1.Probe{}).
				Create(&testCtx).
				GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
		})

		It("with probe - fail", func() {
			By("create a ComponentDefinition obj")
			componentDefObj := testapps.NewComponentDefinitionFactory(componentDefName).
				SetRuntime(nil).
				SetAvailable(&kbappsv1.ComponentAvailable{
					WithProbe: &kbappsv1.ComponentAvailableWithProbe{},
				}).
				// without available probe
				Create(&testCtx).
				GetObject()

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.AvailablePhase)
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

			checkObjectStatus(componentDefObj, kbappsv1.UnavailablePhase)
		})
	})

	Context("immutable", func() {
		newCmpdFn := func(processor func(*testapps.MockComponentDefinitionFactory)) *kbappsv1.ComponentDefinition {
			By("create a ComponentDefinition obj")
			builder := testapps.NewComponentDefinitionFactory(componentDefName).
				SetDescription("v0.0.1").
				SetRuntime(&corev1.Container{
					Name:    "container",
					Image:   "image:v0.0.1",
					Command: []string{"command"},
				}).
				SetUpdateStrategy(nil).
				SetPodManagementPolicy(nil)
			if processor != nil {
				processor(builder)
			}
			obj := builder.Create(&testCtx).GetObject()
			checkObjectStatus(obj, kbappsv1.AvailablePhase)
			return obj
		}

		newCmpd := func() *kbappsv1.ComponentDefinition {
			return newCmpdFn(nil)
		}

		newCmpdSkipImmutableCheck := func() *kbappsv1.ComponentDefinition {
			return newCmpdFn(func(f *testapps.MockComponentDefinitionFactory) {
				f.AddAnnotations(constant.SkipImmutableCheckAnnotationKey, "true")
			})
		}

		It("update mutable fields", func() {
			componentDefObj := newCmpd()

			By("update mutable fields")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentDefObj), func(cmpd *kbappsv1.ComponentDefinition) {
				cmpd.Spec.Description = "v0.0.2"
				parallel := appsv1.ParallelPodManagement
				cmpd.Spec.PodManagementPolicy = &parallel
				cmpd.Spec.MinReadySeconds = 10
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(kbappsv1.AvailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *kbappsv1.ComponentDefinition) {
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(kbappsv1.AvailablePhase))
					g.Expect(cmpd.Spec.Description).Should(Equal("v0.0.2"))
					g.Expect(cmpd.Spec.PodManagementPolicy).ShouldNot(BeNil())
					g.Expect(*cmpd.Spec.PodManagementPolicy).Should(Equal(appsv1.ParallelPodManagement))
				})).Should(Succeed())
		})

		It("update immutable fields - w/ skip annotation", func() {
			componentDefObj := newCmpdSkipImmutableCheck()

			By("update mutable & immutable fields")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentDefObj), func(cmpd *kbappsv1.ComponentDefinition) {
				cmpd.Spec.Description = "v0.0.2"
				cmpd.Spec.Runtime.Containers[0].Image = "image:v0.0.2"
				parallel := kbappsv1.ParallelConcurrency
				cmpd.Spec.UpdateStrategyConstraint = &parallel
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(kbappsv1.AvailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *kbappsv1.ComponentDefinition) {
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(kbappsv1.AvailablePhase))
					g.Expect(cmpd.Spec.Description).Should(Equal("v0.0.2"))
					c := corev1.Container{
						Name:    "container",
						Image:   "image:v0.0.2",
						Command: []string{"command"},
					}
					g.Expect(cmpd.Spec.Runtime.Containers[0]).Should(BeEquivalentTo(c))
					g.Expect(cmpd.Spec.UpdateStrategyConstraint).ShouldNot(BeNil())
					g.Expect(*cmpd.Spec.UpdateStrategyConstraint).Should(Equal(kbappsv1.ParallelConcurrency))
				})).Should(Succeed())
		})

		It("update immutable fields - w/o skip annotation", func() {
			componentDefObj := newCmpd()

			By("update mutable & immutable fields")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentDefObj), func(cmpd *kbappsv1.ComponentDefinition) {
				cmpd.Spec.Description = "v0.0.2"
				cmpd.Spec.Runtime.Containers[0].Image = "image:v0.0.2"
				parallel := kbappsv1.ParallelConcurrency
				cmpd.Spec.UpdateStrategyConstraint = &parallel
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(kbappsv1.UnavailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *kbappsv1.ComponentDefinition) {
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(kbappsv1.UnavailablePhase))
					g.Expect(cmpd.Spec.Description).Should(Equal("v0.0.2"))
					c := corev1.Container{
						Name:    "container",
						Image:   "image:v0.0.2",
						Command: []string{"command"},
					}
					g.Expect(cmpd.Spec.Runtime.Containers[0]).Should(BeEquivalentTo(c))
					g.Expect(cmpd.Spec.UpdateStrategyConstraint).ShouldNot(BeNil())
					g.Expect(*cmpd.Spec.UpdateStrategyConstraint).Should(Equal(kbappsv1.ParallelConcurrency))
				})).Should(Succeed())

			By("revert the change to immutable fields back")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentDefObj), func(cmpd *kbappsv1.ComponentDefinition) {
				cmpd.Spec.Runtime.Containers[0].Image = "image:v0.0.1"
				cmpd.Spec.UpdateStrategyConstraint = nil
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(kbappsv1.AvailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentDefObj),
				func(g Gomega, cmpd *kbappsv1.ComponentDefinition) {
					g.Expect(cmpd.Status.ObservedGeneration).Should(Equal(cmpd.GetGeneration()))
					g.Expect(cmpd.Status.Phase).Should(Equal(kbappsv1.AvailablePhase))
					g.Expect(cmpd.Spec.Description).Should(Equal("v0.0.2"))
					c := corev1.Container{
						Name:    "container",
						Image:   "image:v0.0.1",
						Command: []string{"command"},
					}
					g.Expect(cmpd.Spec.Runtime.Containers[0]).Should(BeEquivalentTo(c))
					g.Expect(cmpd.Spec.UpdateStrategyConstraint).ShouldNot(BeNil())
					g.Expect(*cmpd.Spec.UpdateStrategyConstraint).Should(Equal(kbappsv1.SerialConcurrency))
				})).Should(Succeed())
		})
	})
})
