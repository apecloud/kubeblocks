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

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("SidecarDefinition Controller", func() {
	const (
		hostingCompDefName = "test-compdef-hosting"
		compDefName        = "test-compdef"
		sidecarDefName     = "test-sidecardef"
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ComponentDefinitionSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.SidecarDefinitionSignature, true, ml)

		// namespaced
	}

	BeforeEach(func() {
		cleanEnv()

		By("create the hosting ComponentDefinition obj")
		testapps.NewComponentDefinitionFactory(hostingCompDefName).
			SetRuntime(nil).
			Create(&testCtx)

		By("create the owner ComponentDefinition obj")
		testapps.NewComponentDefinitionFactory(compDefName).
			SetRuntime(nil).
			Create(&testCtx)
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("provision", func() {
		It("ok", func() {
			By("create a SidecarDefinition obj")
			sidecarDefObj := testapps.NewSidecarDefinitionFactory(sidecarDefName, compDefName, []string{hostingCompDefName}).
				AddContainer(nil).
				Create(&testCtx).
				GetObject()

			By("checking the default object reconciled")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj),
				func(g Gomega, sidecarDef *appsv1.SidecarDefinition) {
					g.Expect(sidecarDef.Finalizers).ShouldNot(BeEmpty())
					g.Expect(sidecarDef.Status.ObservedGeneration).Should(Equal(sidecarDef.GetGeneration()))
					g.Expect(sidecarDef.Status.Phase).Should(Equal(appsv1.AvailablePhase))
				})).Should(Succeed())
		})

		It("owner not exist", func() {
			By("create a SidecarDefinition obj")
			sidecarDefObj := testapps.NewSidecarDefinitionFactory(sidecarDefName, compDefName+"-not-exist", []string{hostingCompDefName}).
				AddContainer(nil).
				Create(&testCtx).
				GetObject()

			By("checking the object reconciled")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj),
				func(g Gomega, sidecarDef *appsv1.SidecarDefinition) {
					g.Expect(sidecarDef.Finalizers).ShouldNot(BeEmpty())
					g.Expect(sidecarDef.Status.ObservedGeneration).Should(Equal(sidecarDef.GetGeneration()))
					g.Expect(sidecarDef.Status.Phase).Should(Equal(appsv1.UnavailablePhase))
				})).Should(Succeed())
		})

		// It("cmpd hosting the sidecar not exist", func() {
		//	By("create a SidecarDefinition obj")
		//	sidecarDefObj := testapps.NewSidecarDefinitionFactory(sidecarDefName, compDefName, []string{hostingCompDefName + "-not-exist"}).
		//		AddContainer(nil).
		//		Create(&testCtx).
		//		GetObject()
		//
		//	By("checking the object reconciled")
		//	Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj),
		//		func(g Gomega, sidecarDef *appsv1.SidecarDefinition) {
		//			g.Expect(sidecarDef.Finalizers).ShouldNot(BeEmpty())
		//			g.Expect(sidecarDef.Status.ObservedGeneration).Should(Equal(sidecarDef.GetGeneration()))
		//			g.Expect(sidecarDef.Status.Phase).Should(Equal(appsv1.UnavailablePhase))
		//		})).Should(Succeed())
		// })
	})

	Context("immutable", func() {
		var (
			newHostingCompDefObj *appsv1.ComponentDefinition
		)

		BeforeEach(func() {
			By("create a new hosting ComponentDefinition obj")
			newHostingCompDefObj = testapps.NewComponentDefinitionFactory(hostingCompDefName + "-new").
				SetRuntime(nil).
				Create(&testCtx).
				GetObject()
		})

		checkObjectStatus := func(obj *appsv1.SidecarDefinition, expectedPhase appsv1.Phase) {
			By(fmt.Sprintf("checking the object as %s", strings.ToLower(string(expectedPhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(obj),
				func(g Gomega, sidecarDef *appsv1.SidecarDefinition) {
					g.Expect(sidecarDef.Status.ObservedGeneration).Should(Equal(sidecarDef.GetGeneration()))
					g.Expect(sidecarDef.Status.Phase).Should(Equal(expectedPhase))
				})).Should(Succeed())
		}

		newSidecarDefinitionFn := func(processor func(factory *testapps.MockSidecarDefinitionFactory)) *appsv1.SidecarDefinition {
			By("create a SidecarDefinition obj")
			builder := testapps.NewSidecarDefinitionFactory(sidecarDefName, compDefName, []string{hostingCompDefName}).
				AddContainer(nil)
			if processor != nil {
				processor(builder)
			}
			obj := builder.Create(&testCtx).GetObject()
			checkObjectStatus(obj, appsv1.AvailablePhase)
			return obj
		}

		newSidecarDefinition := func() *appsv1.SidecarDefinition {
			return newSidecarDefinitionFn(nil)
		}

		newSidecarDefinitionSkipImmutableCheck := func() *appsv1.SidecarDefinition {
			return newSidecarDefinitionFn(func(f *testapps.MockSidecarDefinitionFactory) {
				f.AddAnnotations(constant.SkipImmutableCheckAnnotationKey, "true")
			})
		}

		It("update immutable fields - w/ skip annotation", func() {
			sidecarDefObj := newSidecarDefinitionSkipImmutableCheck()

			By("update immutable fields")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj), func(sidecarDef *appsv1.SidecarDefinition) {
				sidecarDef.Spec.Selectors = append(sidecarDef.Spec.Selectors, newHostingCompDefObj.GetName())
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(appsv1.AvailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj),
				func(g Gomega, sidecarDef *appsv1.SidecarDefinition) {
					g.Expect(sidecarDef.Status.ObservedGeneration).Should(Equal(sidecarDef.GetGeneration()))
					g.Expect(sidecarDef.Status.Phase).Should(Equal(appsv1.AvailablePhase))
					g.Expect(sidecarDef.Spec.Selectors).Should(ContainElements(newHostingCompDefObj.GetName()))
				})).Should(Succeed())
		})

		It("update immutable fields - w/o skip annotation", func() {
			sidecarDefObj := newSidecarDefinition()

			By("update immutable fields")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj), func(sidecarDef *appsv1.SidecarDefinition) {
				sidecarDef.Spec.Selectors = append(sidecarDef.Spec.Selectors, newHostingCompDefObj.GetName())
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(appsv1.UnavailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj),
				func(g Gomega, sidecarDef *appsv1.SidecarDefinition) {
					g.Expect(sidecarDef.Status.ObservedGeneration).Should(Equal(sidecarDef.GetGeneration()))
					g.Expect(sidecarDef.Status.Phase).Should(Equal(appsv1.UnavailablePhase))
					g.Expect(sidecarDef.Spec.Selectors).Should(ContainElements(newHostingCompDefObj.GetName()))
				})).Should(Succeed())

			By("revert the change to immutable fields back")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj), func(sidecarDef *appsv1.SidecarDefinition) {
				sidecarDef.Spec.Selectors = sidecarDef.Spec.Selectors[:1]
			})()).Should(Succeed())

			By(fmt.Sprintf("checking the updated object as %s", strings.ToLower(string(appsv1.AvailablePhase))))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(sidecarDefObj),
				func(g Gomega, sidecarDef *appsv1.SidecarDefinition) {
					g.Expect(sidecarDef.Status.ObservedGeneration).Should(Equal(sidecarDef.GetGeneration()))
					g.Expect(sidecarDef.Status.Phase).Should(Equal(appsv1.AvailablePhase))
					g.Expect(sidecarDef.Spec.Selectors).ShouldNot(ContainElements(newHostingCompDefObj.GetName()))
				})).Should(Succeed())
		})
	})
})
