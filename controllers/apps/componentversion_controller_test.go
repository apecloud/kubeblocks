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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ComponentVersion Controller", func() {
	var (
		// compDefinitionObjs []*appsv1alpha1.ComponentDefinition
		compVersionObj *appsv1alpha1.ComponentVersion

		compDefNames    = []string{testapps.CompDefName("v1.0"), testapps.CompDefName("v1.1"), testapps.CompDefName("v2.0"), testapps.CompDefName("v3.0")}
		serviceVersions = []string{testapps.ServiceVersion("v1"), testapps.ServiceVersion("v2"), testapps.ServiceVersion("v3")}
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
				SetServiceVersion(testapps.ServiceVersion("v0")) // use v0 as init service version
			for _, app := range []string{testapps.AppName, testapps.AppNameSamePrefix} {
				// use empty revision as init image tag
				f = f.SetRuntime(&corev1.Container{Name: app, Image: testapps.AppImage(app, testapps.ReleaseID(""))})
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
		obj := testapps.NewComponentVersionFactory(testapps.CompVersionName).
			SetSpec(appsv1alpha1.ComponentVersionSpec{
				CompatibilityRules: []appsv1alpha1.ComponentVersionCompatibilityRule{
					{
						// use prefix
						CompDefs: []string{testapps.CompDefName("v1"), testapps.CompDefName("v2")},
						Releases: []string{testapps.ReleaseID("r0"), testapps.ReleaseID("r1"), testapps.ReleaseID("r2"), testapps.ReleaseID("r3"), testapps.ReleaseID("r4")}, // sv: v1, v2
					},
					{
						// use prefix
						CompDefs: []string{testapps.CompDefName("v3")},
						Releases: []string{testapps.ReleaseID("r5")}, // sv: v3
					},
				},
				Releases: []appsv1alpha1.ComponentVersionRelease{
					{
						Name:           testapps.ReleaseID("r0"),
						Changes:        "init release",
						ServiceVersion: testapps.ServiceVersion("v1"),
						Images: map[string]string{
							testapps.AppName:           testapps.AppImage(testapps.AppName, testapps.ReleaseID("r0")),
							testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r0")),
						},
					},
					{
						Name:           testapps.ReleaseID("r1"),
						Changes:        "update app image",
						ServiceVersion: testapps.ServiceVersion("v1"),
						Images: map[string]string{
							testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r1")),
						},
					},
					{
						Name:           testapps.ReleaseID("r2"),
						Changes:        "publish a new service version",
						ServiceVersion: testapps.ServiceVersion("v2"),
						Images: map[string]string{
							testapps.AppName:           testapps.AppImage(testapps.AppName, testapps.ReleaseID("r2")),
							testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r2")),
						},
					},
					{
						Name:           testapps.ReleaseID("r3"),
						Changes:        "update app image",
						ServiceVersion: testapps.ServiceVersion("v2"),
						Images: map[string]string{
							testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r3")),
						},
					},
					{
						Name:           testapps.ReleaseID("r4"),
						Changes:        "update all app images for previous service version",
						ServiceVersion: testapps.ServiceVersion("v1"),
						Images: map[string]string{
							testapps.AppName:           testapps.AppImage(testapps.AppName, testapps.ReleaseID("r4")),
							testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r4")),
						},
					},
					{
						Name:           testapps.ReleaseID("r5"),
						Changes:        "publish a new service version",
						ServiceVersion: testapps.ServiceVersion("v3"),
						Images: map[string]string{
							testapps.AppName:           testapps.AppImage(testapps.AppName, testapps.ReleaseID("r5")),
							testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r5")),
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
		Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(testapps.AppImage(compDef.Spec.Runtime.Containers[0].Name, testapps.ReleaseID(""))))
		Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(testapps.AppImage(compDef.Spec.Runtime.Containers[1].Name, testapps.ReleaseID(""))))
		Expect(component.UpdateCompDefinitionImages4ServiceVersion(testCtx.Ctx, testCtx.Cli, compDef, serviceVersion)).Should(Succeed())
		Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
		Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(testapps.AppImage(compDef.Spec.Runtime.Containers[0].Name, testapps.ReleaseID(r0))))
		Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(testapps.AppImage(compDef.Spec.Runtime.Containers[1].Name, testapps.ReleaseID(r1))))
	}

	Context("reconcile component version", func() {
		BeforeEach(func() {
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
			compDefKey := types.NamespacedName{Name: testapps.CompDefName("v3.0")}
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

		It("update component definition with valid regexp", func() {
			By("update component version to reference a valid regexp component definition")
			compVersionKey := client.ObjectKeyFromObject(compVersionObj)
			Eventually(testapps.GetAndChangeObj(&testCtx, compVersionKey, func(compVersion *appsv1alpha1.ComponentVersion) {
				compVersion.Spec.CompatibilityRules[1].CompDefs = []string{testapps.CompDefName("^v3$")}
			})).Should(Succeed())

			By("checking the object available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})

		It("update component definition with invalid regexp", func() {
			By("update component version to reference an invalid regexp component definition")
			compVersionKey := client.ObjectKeyFromObject(compVersionObj)
			Eventually(testapps.GetAndChangeObj(&testCtx, compVersionKey, func(compVersion *appsv1alpha1.ComponentVersion) {
				compVersion.Spec.CompatibilityRules[1].CompDefs = []string{testapps.CompDefName("(invalid-v3")}
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
				compVersion.Spec.CompatibilityRules[0].CompDefs = []string{testapps.CompDefName("v2")}
			})).Should(Succeed())

			By("checking the object available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join(serviceVersions, ",")))
					for i := 0; i < len(compDefNames); i++ {
						if strings.HasPrefix(compDefNames[i], testapps.CompDefName("v1")) {
							g.Expect(cmpv.Labels).ShouldNot(HaveKey(compDefNames[i]))
						} else {
							g.Expect(cmpv.Labels).Should(HaveKeyWithValue(compDefNames[i], compDefNames[i]))
						}
					}
				})).Should(Succeed())

			By("delete v1.* component definitions")
			for _, name := range compDefNames {
				if !strings.HasPrefix(name, testapps.CompDefName("v1")) {
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
						if strings.HasPrefix(compDefNames[i], testapps.CompDefName("v1")) {
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
					if release.ServiceVersion == testapps.ServiceVersion("v2") {
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
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join([]string{testapps.ServiceVersion("v1"), testapps.ServiceVersion("v3")}, ",")))
					for i := 0; i < len(compDefNames); i++ {
						g.Expect(cmpv.Labels).Should(HaveKeyWithValue(compDefNames[i], compDefNames[i]))
					}
				})).Should(Succeed())
		})
	})

	Context("resolve component definition, service version and images", func() {
		BeforeEach(func() {
			createCompDefinitionObjs()
			compVersionObj = createCompVersionObj()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("full match", func() {
			By("with definition v1.0 and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v1.1 and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.1"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v2.0 and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2.0"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v1.0 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v1.1 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.1"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2.0 and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2.0"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v3.0 and service version v2")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v3.0"), testapps.ServiceVersion("v3"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")
		})

		It("w/o service version", func() {
			By("with definition v1.0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v1.1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2.0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v3.0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v3.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")
		})

		It("prefix match definition", func() {
			By("with definition prefix and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefinitionName, testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition prefix and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefinitionName, testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition prefix and service version v2")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefinitionName, testapps.ServiceVersion("v3"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 prefix and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v2 prefix and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("prefix match definition and w/o service version", func() {
			By("with definition prefix")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefinitionName, "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 prefix")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2 prefix")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v2"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("regular expression match definition", func() {
			By("with definition exact regex and service version 1")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithExactRegex("v2.0"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition exact regex and service version v2")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithExactRegex("v2.0"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition exact regex and service version v3")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithExactRegex("v3.0"), testapps.ServiceVersion("v3"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 fuzzy regex and service version v0")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithFuzzyRegex("v1"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")

			By("with definition v2 fuzzy regex and service version v1")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithFuzzyRegex("v2"), testapps.ServiceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("regular expression match definition and w/o service version", func() {
			By("with definition regex")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, "^"+testapps.CompDefinitionName, "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v3")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r5", "r5")

			By("with definition v1 regex")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithFuzzyRegex("v1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")

			By("with definition v2 regex")
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefNameWithFuzzyRegex("v2"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})

		It("match from definition", func() {
			By("with definition v1.0 and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), testapps.ServiceVersion("v0"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v0")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "", "") // empty revision of image tag
		})

		It("resolve images from definition and version", func() {
			By("create new definition v4.0 with service version v4")
			compDefObj := testapps.NewComponentDefinitionFactory(testapps.CompDefName("v4.0")).
				SetServiceVersion(testapps.ServiceVersion("v4")).
				SetRuntime(&corev1.Container{Name: testapps.AppName, Image: testapps.AppImage(testapps.AppName, testapps.ReleaseID(""))}).
				SetRuntime(&corev1.Container{Name: testapps.AppNameSamePrefix, Image: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID(""))}).
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
					Name:           testapps.ReleaseID("r6"),
					Changes:        "publish a new service version",
					ServiceVersion: testapps.ServiceVersion("v4"),
					Images: map[string]string{
						testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r6")),
						// not provide image for this app
						// testapps.AppNameSamePrefix: testapps.AppImage(testapps.AppNameSamePrefix, testapps.ReleaseID("r6")),
					},
				}
				rule := appsv1alpha1.ComponentVersionCompatibilityRule{
					CompDefs: []string{testapps.CompDefName("v4")}, // use prefix
					Releases: []string{testapps.ReleaseID("r6")},
				}
				compVersion.Spec.CompatibilityRules = append(compVersion.Spec.CompatibilityRules, rule)
				compVersion.Spec.Releases = append(compVersion.Spec.Releases, release)
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, compVersionKey, func(g Gomega, compVersion *appsv1alpha1.ComponentVersion) {
				g.Expect(compVersion.Status.ObservedGeneration).Should(Equal(compVersion.Generation))
			})).Should(Succeed())

			By("with definition v4.0 and service version v3")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v4.0"), testapps.ServiceVersion("v4"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v4.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v4")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r6", "") // app is r6 and another one is ""
		})
	})

	Context("resolve component definition, service version without serviceVersion in componentDefinition", func() {
		BeforeEach(func() {
			compDefs := createCompDefinitionObjs()
			for _, compDef := range compDefs {
				compDefKey := client.ObjectKeyFromObject(compDef)
				Eventually(testapps.GetAndChangeObj(&testCtx, compDefKey, func(compDef *appsv1alpha1.ComponentDefinition) {
					compDef.Spec.ServiceVersion = ""
				})).Should(Succeed())
			}
			compVersionObj = createCompVersionObj()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("full match", func() {
			By("with definition v1.0 and service version v0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v1")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r4", "r4")
		})

		It("w/o service version", func() {
			By("with definition v1.0")
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, testapps.CompDefName("v1.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(testapps.CompDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(testapps.ServiceVersion("v2")))
			updateNCheckCompDefinitionImages(compDef, resolvedServiceVersion, "r3", "r2")
		})
	})
})
