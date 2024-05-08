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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ComponentVersion Controller", func() {
	const (
		compDefinitionName = "test-component-definition"
		compVersionName    = "test-component-version"

		appName           = "app"
		appNameSamePrefix = "app-same-prefix"

		releasePrefix        = "v0.0.1"
		serviceVersionPrefix = "8.0.30"
	)

	var (
		appImage = func(app, tag string) string {
			return fmt.Sprintf("%s:%s", app, tag)
		}
		compDefName = func(r string) string {
			return fmt.Sprintf("%s-%s", compDefinitionName, r)
		}
		releaseID = func(r string) string {
			return fmt.Sprintf("%s-%s", releasePrefix, r)
		}
		serviceVersion = func(r string) string {
			if len(r) == 0 {
				return serviceVersionPrefix
			}
			return fmt.Sprintf("%s-%s", serviceVersionPrefix, r)
		}

		// compDefinitionObjs []*appsv1alpha1.ComponentDefinition
		compVersionObj *appsv1alpha1.ComponentVersion

		compDefNames    = []string{compDefName("v1.0"), compDefName("v1.1"), compDefName("v2.0"), compDefName("v3.0")}
		serviceVersions = []string{serviceVersion("v1"), serviceVersion("v2"), serviceVersion("v3")}
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentDefinitionSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentVersionSignature, true, ml)

		// namespaced
	}

	BeforeEach(func() {
		cleanEnv()

	})

	AfterEach(func() {
		cleanEnv()
	})

	createCompDefinitionObjs := func() []*appsv1alpha1.ComponentDefinition {
		By("create default ComponentDefinition objs")
		objs := make([]*appsv1alpha1.ComponentDefinition, 0)
		for _, name := range compDefNames {
			f := testapps.NewComponentDefinitionFactory(name).
				SetServiceVersion(serviceVersion("v0")) // use v0 as init service version
			for _, app := range []string{appName, appNameSamePrefix} {
				// use empty revision as init image tag
				f = f.SetRuntime(&corev1.Container{Name: app, Image: appImage(app, releaseID(""))})
			}
			objs = append(objs, f.Create(&testCtx).GetObject())
		}
		for _, obj := range objs {
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(obj),
				func(g Gomega, compDef *appsv1alpha1.ComponentDefinition) {
					g.Expect(compDef.Status.ObservedGeneration).Should(Equal(compDef.Generation))
				})).Should(Succeed())
		}
		return objs
	}

	createCompVersionObj := func() *appsv1alpha1.ComponentVersion {
		By("create a default ComponentVersion obj with multiple releases")
		obj := testapps.NewComponentVersionFactory(compVersionName).
			SetSpec(appsv1alpha1.ComponentVersionSpec{
				CompatibilityRules: []appsv1alpha1.ComponentVersionCompatibilityRule{
					{
						// use prefix
						CompDefs: []string{compDefName("v1"), compDefName("v2")},
						Releases: []string{releaseID("r0"), releaseID("r1"), releaseID("r2"), releaseID("r3"), releaseID("r4")}, // sv: v1, v2
					},
					{
						// use prefix
						CompDefs: []string{compDefName("v3")},
						Releases: []string{releaseID("r5")}, // sv: v3
					},
				},
				Releases: []appsv1alpha1.ComponentVersionRelease{
					{
						Name:           releaseID("r0"),
						Changes:        "init release",
						ServiceVersion: serviceVersion("v1"),
						Images: map[string]string{
							appName:           appImage(appName, releaseID("r0")),
							appNameSamePrefix: appImage(appNameSamePrefix, releaseID("r0")),
						},
					},
					{
						Name:           releaseID("r1"),
						Changes:        "update app image",
						ServiceVersion: serviceVersion("v1"),
						Images: map[string]string{
							appName: appImage(appName, releaseID("r1")),
						},
					},
					{
						Name:           releaseID("r2"),
						Changes:        "publish a new service version",
						ServiceVersion: serviceVersion("v2"),
						Images: map[string]string{
							appName:           appImage(appName, releaseID("r2")),
							appNameSamePrefix: appImage(appNameSamePrefix, releaseID("r2")),
						},
					},
					{
						Name:           releaseID("r3"),
						Changes:        "update app image",
						ServiceVersion: serviceVersion("v2"),
						Images: map[string]string{
							appName: appImage(appName, releaseID("r3")),
						},
					},
					{
						Name:           releaseID("r4"),
						Changes:        "update all app images for previous service version",
						ServiceVersion: serviceVersion("v1"),
						Images: map[string]string{
							appName:           appImage(appName, releaseID("r4")),
							appNameSamePrefix: appImage(appNameSamePrefix, releaseID("r4")),
						},
					},
					{
						Name:           releaseID("r5"),
						Changes:        "publish a new service version",
						ServiceVersion: serviceVersion("v3"),
						Images: map[string]string{
							appName:           appImage(appName, releaseID("r5")),
							appNameSamePrefix: appImage(appNameSamePrefix, releaseID("r5")),
						},
					},
				},
			}).
			Create(&testCtx).
			GetObject()
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(obj),
			func(g Gomega, compVersion *appsv1alpha1.ComponentVersion) {
				g.Expect(compVersion.Status.ObservedGeneration).Should(Equal(compVersion.Generation))
			})).Should(Succeed())

		return obj
	}

	updateNCheckCompDefinitionImages := func(compDef *appsv1alpha1.ComponentDefinition, serviceVersion string, r0, r1 string) {
		Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseID(""))))
		Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseID(""))))
		Expect(updateCompDefinitionImages4ServiceVersion(testCtx.Ctx, testCtx.Cli, compDef, serviceVersion)).Should(Succeed())
		Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
		Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseID(r0))))
		Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseID(r1))))
	}

	Context("reconcile component version", func() {
		BeforeEach(func() {
			// compDefinitionObjs = createCompDefinitionObjs()
			createCompDefinitionObjs()
			compVersionObj = createCompVersionObj()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("ok", func() {
			By("checking the object available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Finalizers).ShouldNot(BeEmpty())
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join(serviceVersions, ",")))
					for i := 0; i < len(compDefNames); i++ {
						g.Expect(cmpv.Labels).Should(HaveKeyWithValue(compDefNames[i], compDefNames[i]))
					}
				})).Should(Succeed())
		})

		It("release has no supported component definitions", func() {
			By("delete v3.0 component definition, let release r5 has no available component definitions")
			compDefKey := types.NamespacedName{Name: compDefName("v3.0")}
			testapps.DeleteObject(&testCtx, compDefKey, &appsv1alpha1.ComponentDefinition{})
			Eventually(testapps.CheckObjExists(&testCtx, compDefKey, &appsv1alpha1.ComponentDefinition{}, false)).Should(Succeed())

			By("checking the object unavailable")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())
		})

		It("w/o container defined", func() {
			By("update component version to add a non-exist app")
			compVersionKey := client.ObjectKeyFromObject(compVersionObj)
			Eventually(testapps.GetAndChangeObj(&testCtx, compVersionKey, func(compVersion *appsv1alpha1.ComponentVersion) {
				compVersion.Spec.Releases[0].Images["app-non-exist"] = "app-image-non-exist"
			})).Should(Succeed())

			By("checking the object unavailable")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())
		})

		It("delete component definition", func() {
			By("update component version to delete definition v1.*")
			compVersionKey := client.ObjectKeyFromObject(compVersionObj)
			Eventually(testapps.GetAndChangeObj(&testCtx, compVersionKey, func(compVersion *appsv1alpha1.ComponentVersion) {
				compVersion.Spec.CompatibilityRules[0].CompDefs = []string{compDefName("v2")}
			})).Should(Succeed())

			By("checking the object available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join(serviceVersions, ",")))
					for i := 0; i < len(compDefNames); i++ {
						if strings.HasPrefix(compDefNames[i], compDefName("v1")) {
							g.Expect(cmpv.Labels).ShouldNot(HaveKey(compDefNames[i]))
						} else {
							g.Expect(cmpv.Labels).Should(HaveKeyWithValue(compDefNames[i], compDefNames[i]))
						}
					}
				})).Should(Succeed())

			By("delete v1.* component definitions")
			for _, name := range compDefNames {
				if !strings.HasPrefix(name, compDefName("v1")) {
					continue
				}
				compDefKey := types.NamespacedName{Name: name}
				testapps.DeleteObject(&testCtx, compDefKey, &appsv1alpha1.ComponentDefinition{})
				Eventually(testapps.CheckObjExists(&testCtx, compDefKey, &appsv1alpha1.ComponentDefinition{}, false)).Should(Succeed())
			}

			By("checking the object available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join(serviceVersions, ",")))
					for i := 0; i < len(compDefNames); i++ {
						if strings.HasPrefix(compDefNames[i], compDefName("v1")) {
							g.Expect(cmpv.Labels).ShouldNot(HaveKey(compDefNames[i]))
						} else {
							g.Expect(cmpv.Labels).Should(HaveKeyWithValue(compDefNames[i], compDefNames[i]))
						}
					}
				})).Should(Succeed())
		})

		It("delete a release", func() {
			By("update component version to delete first release")
			compVersionKey := client.ObjectKeyFromObject(compVersionObj)
			Eventually(testapps.GetAndChangeObj(&testCtx, compVersionKey, func(compVersion *appsv1alpha1.ComponentVersion) {
				compVersion.Spec.Releases = compVersion.Spec.Releases[1:]
				compVersion.Spec.CompatibilityRules[0].Releases = compVersion.Spec.CompatibilityRules[0].Releases[1:]
			})).Should(Succeed())

			By("checking the object available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join(serviceVersions, ",")))
					for i := 0; i < len(compDefNames); i++ {
						g.Expect(cmpv.Labels).Should(HaveKeyWithValue(compDefNames[i], compDefNames[i]))
					}
				})).Should(Succeed())
		})

		It("delete a service version", func() {
			By("update component version to delete releases for service version v1")
			compVersionKey := client.ObjectKeyFromObject(compVersionObj)
			Eventually(testapps.GetAndChangeObj(&testCtx, compVersionKey, func(compVersion *appsv1alpha1.ComponentVersion) {
				releaseToDelete := sets.New[string]()
				releases := make([]appsv1alpha1.ComponentVersionRelease, 0)
				for i, release := range compVersion.Spec.Releases {
					if release.ServiceVersion == serviceVersion("v2") {
						releaseToDelete.Insert(release.Name)
						continue
					}
					releases = append(releases, compVersion.Spec.Releases[i])
				}
				compVersion.Spec.Releases = releases

				for i, rule := range compVersion.Spec.CompatibilityRules {
					releaseNames := make([]string, 0)
					for j, release := range rule.Releases {
						if releaseToDelete.Has(release) {
							continue
						}
						releaseNames = append(releaseNames, rule.Releases[j])
					}
					compVersion.Spec.CompatibilityRules[i].Releases = releaseNames
				}
			})).Should(Succeed())

			By("checking the object available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join([]string{serviceVersion("v1"), serviceVersion("v3")}, ",")))
					for i := 0; i < len(compDefNames); i++ {
						g.Expect(cmpv.Labels).Should(HaveKeyWithValue(compDefNames[i], compDefNames[i]))
					}
				})).Should(Succeed())
		})
	})

	Context("resolve component definition, service version and images", func() {
		BeforeEach(func() {
			// compDefinitionObjs = createCompDefinitionObjs()
			createCompDefinitionObjs()
			compVersionObj = createCompVersionObj()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("full match", func() {
			By("with definition v1.0 and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.0"), serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v1.1 and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.1"), serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v2.0 and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2.0"), serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v1.0 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.0"), serviceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v1.1 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.1"), serviceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2.0 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2.0"), serviceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v3.0 and service version v2")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v3.0"), serviceVersion("v3"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")
		})

		It("w/o service version", func() {
			By("with definition v1.0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v1.1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2.0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v3.0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v3.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")
		})

		It("prefix match definition", func() {
			By("with definition prefix and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefinitionName, serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition prefix and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefinitionName, serviceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition prefix and service version v2")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefinitionName, serviceVersion("v3"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 prefix and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1"), serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v2 prefix and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2"), serviceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("prefix match definition and w/o service version", func() {
			By("with definition prefix")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefinitionName, "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 prefix")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2 prefix")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("match from definition", func() {
			By("with definition v1.0 and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.0"), serviceVersion("v0"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v0")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "", "") // empty revision of image tag
		})

		It("resolve images from definition and version", func() {
			By("create new definition v4.0 with service version v4")
			compDefObj := testapps.NewComponentDefinitionFactory(compDefName("v4.0")).
				SetServiceVersion(serviceVersion("v4")).
				SetRuntime(&corev1.Container{Name: appName, Image: appImage(appName, releaseID(""))}).
				SetRuntime(&corev1.Container{Name: appNameSamePrefix, Image: appImage(appNameSamePrefix, releaseID(""))}).
				Create(&testCtx).
				GetObject()
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compDefObj),
				func(g Gomega, compDef *appsv1alpha1.ComponentDefinition) {
					g.Expect(compDef.Status.ObservedGeneration).Should(Equal(compDef.Generation))
				})).Should(Succeed())

			By("new release for the definition")
			compVersionKey := client.ObjectKeyFromObject(compVersionObj)
			Eventually(testapps.GetAndChangeObj(&testCtx, compVersionKey, func(compVersion *appsv1alpha1.ComponentVersion) {
				release := appsv1alpha1.ComponentVersionRelease{
					Name:           releaseID("r6"),
					Changes:        "publish a new service version",
					ServiceVersion: serviceVersion("v4"),
					Images: map[string]string{
						appName: appImage(appName, releaseID("r6")),
						// not provide image for this app
						// appNameSamePrefix: appImage(appNameSamePrefix, releaseID("r6")),
					},
				}
				rule := appsv1alpha1.ComponentVersionCompatibilityRule{
					CompDefs: []string{compDefName("v4")}, // use prefix
					Releases: []string{releaseID("r6")},
				}
				compVersion.Spec.CompatibilityRules = append(compVersion.Spec.CompatibilityRules, rule)
				compVersion.Spec.Releases = append(compVersion.Spec.Releases, release)
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, compVersionKey, func(g Gomega, compVersion *appsv1alpha1.ComponentVersion) {
				g.Expect(compVersion.Status.ObservedGeneration).Should(Equal(compVersion.Generation))
			})).Should(Succeed())

			By("with definition v4.0 and service version v3")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v4.0"), serviceVersion("v4"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v4.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v4")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r6", "") // app is r6 and another one is ""
		})

		It("resolve images before and after new release", func() {
			By("create new definition v4.0 with service version v4")
			compDefObj := testapps.NewComponentDefinitionFactory(compDefName("v1")).
				SetRuntime(&corev1.Container{Name: appName}).
				GetObject()

			releases := []appsv1alpha1.ComponentVersionRelease{
				{
					Name:           releaseID("r0"),
					Changes:        "init release in v1",
					ServiceVersion: serviceVersion("v1"),
					Images: map[string]string{
						appName: appImage(appName, releaseID("r0")),
					},
				},
				{
					Name:           releaseID("r1"),
					Changes:        "new release in v2",
					ServiceVersion: serviceVersion("v2"), // has different service version
					Images: map[string]string{
						appName: appImage(appName, releaseID("r1")),
					},
				},
				{
					Name:           releaseID("r2"),
					Changes:        "new release in v1",
					ServiceVersion: serviceVersion("v1"), // has same service version
					Images: map[string]string{
						appName: appImage(appName, releaseID("r2")),
					},
				},
			}

			By("first release for the definition")
			compVersionObj := testapps.NewComponentVersionFactory(compVersionName).
				SetSpec(appsv1alpha1.ComponentVersionSpec{
					CompatibilityRules: []appsv1alpha1.ComponentVersionCompatibilityRule{
						{
							CompDefs: []string{compDefName("v1")},
							Releases: []string{releases[0].Name},
						},
					},
					Releases: []appsv1alpha1.ComponentVersionRelease{releases[0]},
				}).
				GetObject()

			By("with app image in r0")
			err := resolveImagesWithCompVersions(compDefObj, []*appsv1alpha1.ComponentVersion{compVersionObj}, serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDefObj.Spec.Runtime.Containers[0].Image).Should(Equal(releases[0].Images[appName]))

			By("publish a new release which has different service version")
			compVersionObj.Spec.Releases = append(compVersionObj.Spec.Releases, releases[1])
			compVersionObj.Spec.CompatibilityRules[0].Releases = append(compVersionObj.Spec.CompatibilityRules[0].Releases, releases[1].Name)

			By("with app image still in r0")
			err = resolveImagesWithCompVersions(compDefObj, []*appsv1alpha1.ComponentVersion{compVersionObj}, serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDefObj.Spec.Runtime.Containers[0].Image).Should(Equal(releases[0].Images[appName]))

			By("publish a new release which has same service version")
			compVersionObj.Spec.Releases = append(compVersionObj.Spec.Releases, releases[2])
			compVersionObj.Spec.CompatibilityRules[0].Releases = append(compVersionObj.Spec.CompatibilityRules[0].Releases, releases[2].Name)

			By("with app image in r2")
			err = resolveImagesWithCompVersions(compDefObj, []*appsv1alpha1.ComponentVersion{compVersionObj}, serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDefObj.Spec.Runtime.Containers[0].Image).Should(Equal(releases[2].Images[appName]))
		})
	})
})
