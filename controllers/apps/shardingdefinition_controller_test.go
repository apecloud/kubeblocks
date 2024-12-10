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

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ShardingDefinition Controller", func() {
	const (
		shardingDefName  = "test-shardingdef"
		componentDefName = "test-componentdef"
		adminAccount     = "admin"
	)

	var (
		compDefObj *appsv1.ComponentDefinition
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ShardingDefinitionSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ComponentDefinitionSignature, true, ml)

		// namespaced
	}

	checkObjectStatus := func(obj *appsv1.ShardingDefinition, expectedPhase appsv1.Phase) {
		By(fmt.Sprintf("checking the object as %s", strings.ToLower(string(expectedPhase))))
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(obj),
			func(g Gomega, sdd *appsv1.ShardingDefinition) {
				g.Expect(sdd.Status.ObservedGeneration).Should(Equal(sdd.GetGeneration()))
				g.Expect(sdd.Status.Phase).Should(Equal(expectedPhase))
			})).Should(Succeed())
	}

	BeforeEach(func() {
		cleanEnv()

		By("create a ComponentDefinition obj")
		compDefObj = testapps.NewComponentDefinitionFactory(componentDefName).
			SetRuntime(nil).
			AddSystemAccount(adminAccount, false, "create user").
			Create(&testCtx).GetObject()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("default", func() {
		It("reconcile empty object", func() {
			By("create a ShardingDefinition obj")
			shardingDefObj := testapps.NewShardingDefinitionFactory(shardingDefName, compDefObj.GetName()).
				Create(&testCtx).GetObject()

			By("checking the object reconciled")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(shardingDefObj),
				func(g Gomega, sdd *appsv1.ShardingDefinition) {
					g.Expect(sdd.Finalizers).ShouldNot(BeEmpty())
					g.Expect(sdd.Status.ObservedGeneration).Should(Equal(sdd.GetGeneration()))
					g.Expect(sdd.Status.Phase).Should(Equal(appsv1.AvailablePhase))
				})).Should(Succeed())
		})
	})

	Context("template", func() {
		It("the component definition object doesn't exist", func() {
			By("create a ShardingDefinition obj")
			shardingDefObj := testapps.NewShardingDefinitionFactory(shardingDefName, "test-compdef-not-exist").
				Create(&testCtx).GetObject()

			checkObjectStatus(shardingDefObj, appsv1.UnavailablePhase)
		})
	})

	Context("provision & update strategy", func() {
		It("ok", func() {
			By("create a ShardingDefinition obj")
			shardingDefObj := testapps.NewShardingDefinitionFactory(shardingDefName, compDefObj.GetName()).
				SetProvisionStrategy(appsv1.SerialConcurrency).
				SetUpdateStrategy(appsv1.ParallelConcurrency).
				Create(&testCtx).GetObject()

			checkObjectStatus(shardingDefObj, appsv1.AvailablePhase)
		})

		It("unsupported strategy", func() {
			By("create a ShardingDefinition obj")
			shardingDefObj := testapps.NewShardingDefinitionFactory(shardingDefName, compDefObj.GetName()).
				SetProvisionStrategy(appsv1.BestEffortParallelConcurrency).
				SetUpdateStrategy(appsv1.BestEffortParallelConcurrency).
				Create(&testCtx).GetObject()

			checkObjectStatus(shardingDefObj, appsv1.UnavailablePhase)
		})
	})

	Context("system accounts", func() {
		It("ok", func() {
			By("create a ShardingDefinition obj")
			shardingDefObj := testapps.NewShardingDefinitionFactory(shardingDefName, compDefObj.GetName()).
				AddSystemAccount(appsv1.ShardingSystemAccount{Name: adminAccount, Shared: pointer.Bool(true)}).
				Create(&testCtx).GetObject()

			checkObjectStatus(shardingDefObj, appsv1.AvailablePhase)
		})

		It("duplicate accounts", func() {
			By("create a ShardingDefinition obj")
			shardingDefObj := testapps.NewShardingDefinitionFactory(shardingDefName, compDefObj.GetName()).
				AddSystemAccount(appsv1.ShardingSystemAccount{Name: adminAccount}).
				AddSystemAccount(appsv1.ShardingSystemAccount{Name: adminAccount, Shared: pointer.Bool(true)}).
				Create(&testCtx).GetObject()

			checkObjectStatus(shardingDefObj, appsv1.UnavailablePhase)
		})

		It("account not defined", func() {
			By("create a ShardingDefinition obj")
			shardingDefObj := testapps.NewShardingDefinitionFactory(shardingDefName, compDefObj.GetName()).
				AddSystemAccount(appsv1.ShardingSystemAccount{Name: "account-not-defined"}).
				Create(&testCtx).GetObject()

			checkObjectStatus(shardingDefObj, appsv1.UnavailablePhase)
		})
	})

	Context("immutable", func() {
		var (
			newCompDefObj *appsv1.ComponentDefinition
		)

		BeforeEach(func() {
			By("create a new ComponentDefinition obj")
			newCompDefObj = testapps.NewComponentDefinitionFactory(componentDefName + "-new").
				SetRuntime(nil).
				Create(&testCtx).GetObject()
		})

		newSddFn := func(processor func(factory *testapps.MockShardingDefinitionFactory)) *appsv1.ShardingDefinition {
			By("create a ShardingDefinition obj")
			builder := testapps.NewShardingDefinitionFactory(shardingDefName, compDefObj.GetName())
			if processor != nil {
				processor(builder)
			}
			obj := builder.Create(&testCtx).GetObject()
			checkObjectStatus(obj, appsv1.AvailablePhase)
			return obj
		}

		newSdd := func() *appsv1.ShardingDefinition {
			return newSddFn(nil)
		}

		newSddSkipImmutableCheck := func() *appsv1.ShardingDefinition {
			return newSddFn(func(f *testapps.MockShardingDefinitionFactory) {
				f.AddAnnotations(constant.SkipImmutableCheckAnnotationKey, "true")
			})
		}

		It("update immutable fields - w/ skip annotation", func() {
			shardingDefObj := newSddSkipImmutableCheck()

			By("update immutable fields")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(shardingDefObj), func(sdd *appsv1.ShardingDefinition) {
				sdd.Spec.Template.CompDef = newCompDefObj.GetName()
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(appsv1.AvailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(shardingDefObj),
				func(g Gomega, sdd *appsv1.ShardingDefinition) {
					g.Expect(sdd.Status.ObservedGeneration).Should(Equal(sdd.GetGeneration()))
					g.Expect(sdd.Status.Phase).Should(Equal(appsv1.AvailablePhase))
					g.Expect(sdd.Spec.Template.CompDef).Should(Equal(newCompDefObj.GetName()))
				})).Should(Succeed())
		})

		It("update immutable fields - w/o skip annotation", func() {
			shardingDefObj := newSdd()

			By("update immutable fields")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(shardingDefObj), func(sdd *appsv1.ShardingDefinition) {
				sdd.Spec.Template.CompDef = newCompDefObj.GetName()
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(appsv1.UnavailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(shardingDefObj),
				func(g Gomega, sdd *appsv1.ShardingDefinition) {
					g.Expect(sdd.Status.ObservedGeneration).Should(Equal(sdd.GetGeneration()))
					g.Expect(sdd.Status.Phase).Should(Equal(appsv1.UnavailablePhase))
					g.Expect(sdd.Spec.Template.CompDef).Should(Equal(newCompDefObj.GetName()))
				})).Should(Succeed())

			By("revert the change to immutable fields back")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(shardingDefObj), func(sdd *appsv1.ShardingDefinition) {
				sdd.Spec.Template.CompDef = compDefObj.GetName()
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(appsv1.AvailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(shardingDefObj),
				func(g Gomega, sdd *appsv1.ShardingDefinition) {
					g.Expect(sdd.Status.ObservedGeneration).Should(Equal(sdd.GetGeneration()))
					g.Expect(sdd.Status.Phase).Should(Equal(appsv1.AvailablePhase))
					g.Expect(sdd.Spec.Template.CompDef).Should(Equal(compDefObj.GetName()))
				})).Should(Succeed())
		})
	})
})
