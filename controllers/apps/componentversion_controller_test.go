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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ComponentVersion Controller", func() {
	const (
		componentDefName     = "test-componentdef"
		componentVersionName = "test-componentversion"
	)

	var (
		appName           = "app"
		appWithSamePrefix = "app-same-prefix"

		initRelease    = "v0.0.1"
		serviceVersion = "8.0.30"

		appImage = func(name, tag string) string {
			return fmt.Sprintf("%s-image:%s", name, tag)
		}

		defaultContainers = map[string]string{
			appName:           appImage(appName, initRelease),
			appWithSamePrefix: appImage(appWithSamePrefix, initRelease),
		}
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ComponentDefinitionSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ComponentVersionSignature, true, ml)

		// namespaced
	}

	BeforeEach(func() {
		cleanEnv()

	})

	AfterEach(func() {
		cleanEnv()
	})

	newRelease := func(v, sv string, apps ...string) (string, string, string, map[string]string) {
		images := map[string]string{}
		for _, app := range apps {
			images[app] = appImage(app, v)
		}
		return v, "update", sv, images
	}

	createComponentDefObj := func(name string) *appsv1alpha1.ComponentDefinition {
		f := testapps.NewComponentDefinitionFactory(name)
		for _, name := range defaultContainers {
			f = f.SetRuntime(&corev1.Container{Name: name})
		}
		return f.Create(&testCtx).GetObject()
	}

	Context("provision & termination", func() {
		It("ok", func() {
			By("create a ComponentDefinition obj")
			createComponentDefObj(componentDefName)

			By("create a ComponentVersion obj")
			compVersionObj := testapps.NewComponentVersionFactory(componentVersionName).
				AddRelease(newRelease(initRelease, serviceVersion, maps.Keys(defaultContainers)...)).
				AddCompatibilityRule([]string{componentDefName}, []string{initRelease}).
				Create(&testCtx).
				GetObject()

			By("checking the object reconciled")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Finalizers).ShouldNot(BeEmpty())
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(serviceVersion))
					g.Expect(cmpv.Labels).Should(HaveKeyWithValue(componentDefName, componentDefName))
				})).Should(Succeed())
		})

		It("w/o component definition", func() {
			By("create a ComponentVersion obj")
			compVersionObj := testapps.NewComponentVersionFactory(componentVersionName).
				AddRelease(newRelease(initRelease, serviceVersion, maps.Keys(defaultContainers)...)).
				AddCompatibilityRule([]string{componentDefName}, []string{initRelease}).
				Create(&testCtx).
				GetObject()

			By("checking the object reconciled")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Finalizers).ShouldNot(BeEmpty())
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())
		})
	})
})
