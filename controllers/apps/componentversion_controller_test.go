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
		releaseId = func(r string) string {
			return fmt.Sprintf("%s-%s", releasePrefix, r)
		}
		serviceVersion = func(r string) string {
			return fmt.Sprintf("%s-%s", serviceVersionPrefix, r)
		}

		// compDefinitionObjs []*appsv1alpha1.ComponentDefinition
		compVersionObj *appsv1alpha1.ComponentVersion

		compDefNames    = []string{compDefName("v1.0"), compDefName("v1.1"), compDefName("v2.0"), compDefName("v3.0")}
		serviceVersions = []string{serviceVersion("v0"), serviceVersion("v1"), serviceVersion("v2")}
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
				SetServiceVersion(serviceVersion("")) // use empty revision as init service version
			for _, app := range []string{appName, appNameSamePrefix} {
				// use empty revision as init image tag
				f = f.SetRuntime(&corev1.Container{Name: app, Image: appImage(name, releaseId(""))})
			}
			objs = append(objs, f.Create(&testCtx).GetObject())
		}
		return objs
	}

	createCompVersionObj := func() *appsv1alpha1.ComponentVersion {
		By("create a default ComponentVersion obj with multiple releases")
		return testapps.NewComponentVersionFactory(compVersionName).
			SetSpec(appsv1alpha1.ComponentVersionSpec{
				CompatibilityRules: []appsv1alpha1.ComponentVersionCompatibilityRule{
					{
						// use prefix
						CompDefs: []string{compDefName("v1"), compDefName("v2")},
						Releases: []string{releaseId("r0"), releaseId("r1"), releaseId("r2"), releaseId("r3"), releaseId("r4")},
					},
					{
						// use prefix
						CompDefs: []string{compDefName("v3")},
						Releases: []string{releaseId("r5")},
					},
				},
				Releases: []appsv1alpha1.ComponentVersionRelease{
					{
						Name:           releaseId("r0"),
						Changes:        "init release",
						ServiceVersion: serviceVersion("v0"),
						Images: map[string]string{
							appName:           appImage(appName, releaseId("r0")),
							appNameSamePrefix: appImage(appNameSamePrefix, releaseId("r0")),
						},
					},
					{
						Name:           releaseId("r1"),
						Changes:        "update app image",
						ServiceVersion: serviceVersion("v0"),
						Images: map[string]string{
							appName: appImage(appName, releaseId("r1")),
						},
					},
					{
						Name:           releaseId("r2"),
						Changes:        "publish a new service version",
						ServiceVersion: serviceVersion("v1"),
						Images: map[string]string{
							appName:           appImage(appName, releaseId("r2")),
							appNameSamePrefix: appImage(appNameSamePrefix, releaseId("r2")),
						},
					},
					{
						Name:           releaseId("r3"),
						Changes:        "update app image",
						ServiceVersion: serviceVersion("v1"),
						Images: map[string]string{
							appName: appImage(appName, releaseId("r3")),
						},
					},
					{
						Name:           releaseId("r4"),
						Changes:        "update all app images for previous service version",
						ServiceVersion: serviceVersion("v0"),
						Images: map[string]string{
							appName:           appImage(appName, releaseId("r4")),
							appNameSamePrefix: appImage(appNameSamePrefix, releaseId("r4")),
						},
					},
					{
						Name:           releaseId("r5"),
						Changes:        "publish a new service version",
						ServiceVersion: serviceVersion("v2"),
						Images: map[string]string{
							appName: appImage(appName, releaseId("r5")),
							// appNameSamePrefix: appImage(appNameSamePrefix, releaseId("r5")),
						},
					},
				},
			}).
			Create(&testCtx).
			GetObject()
	}

	Context("provision & termination", func() {
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
					g.Expect(cmpv.Labels).Should(HaveLen(len(compDefNames)))
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

			isV1CompDefinition := func(s string) bool {
				return strings.HasPrefix(s, compDefName("v1"))
			}
			v1CompDefinitionCnt := generics.CountFunc(compDefNames, isV1CompDefinition)

			By("checking the object available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
				func(g Gomega, cmpv *appsv1alpha1.ComponentVersion) {
					g.Expect(cmpv.Status.ObservedGeneration).Should(Equal(cmpv.GetGeneration()))
					g.Expect(cmpv.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join(serviceVersions, ",")))
					g.Expect(cmpv.Labels).Should(HaveLen(len(compDefNames) - v1CompDefinitionCnt))
					for i := 0; i < len(compDefNames); i++ {
						if isV1CompDefinition(compDefNames[i]) {
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
					g.Expect(cmpv.Labels).Should(HaveLen(len(compDefNames) - v1CompDefinitionCnt))
					for i := 0; i < len(compDefNames); i++ {
						if isV1CompDefinition(compDefNames[i]) {
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
					g.Expect(cmpv.Labels).Should(HaveLen(len(compDefNames)))
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
					if release.ServiceVersion == serviceVersion("v1") {
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
					g.Expect(cmpv.Status.ServiceVersions).Should(Equal(strings.Join([]string{serviceVersion("v0"), serviceVersion("v2")}, ",")))
					g.Expect(cmpv.Labels).Should(HaveLen(len(compDefNames)))
					for i := 0; i < len(compDefNames); i++ {
						g.Expect(cmpv.Labels).Should(HaveKeyWithValue(compDefNames[i], compDefNames[i]))
					}
				})).Should(Succeed())
		})
	})

	FContext("component definition and service version", func() {
		BeforeEach(func() {
			// compDefinitionObjs = createCompDefinitionObjs()
			createCompDefinitionObjs()
			compVersionObj = createCompVersionObj()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("match from definition", func() {
			// use empty revision of service version
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.0"), serviceVersion(""))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			// empty revision of image tag
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId(""))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId(""))))
		})

		It("exact match", func() {
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.0"), serviceVersion("v0"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v0")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r4"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r4"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.1"), serviceVersion("v0"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v0")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r4"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r4"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2.0"), serviceVersion("v0"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v0")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r4"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r4"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.0"), serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.1"), serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2.0"), serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v3.0"), serviceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r5"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r5"))))
		})

		It("w/o service version", func() {
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1.1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v3.0"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r5"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r5"))))
		})

		It("prefix match definition", func() {
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefinitionName, serviceVersion("v0"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v0")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r4"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r4"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefinitionName, serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))

			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefinitionName, serviceVersion("v2"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r5"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r5"))))

			// v1 prefix
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1"), serviceVersion("v0"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v0")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r4"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r4"))))

			// v2 prefix
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2"), serviceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))
		})

		It("prefix match definition and w/o service version", func() {
			compDef, resolvedServiceVersion, err := resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefinitionName, "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v3.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v2")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r5"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r5"))))

			// v1 prefix
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v1"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v1.1")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))

			// v2 prefix
			compDef, resolvedServiceVersion, err = resolveCompDefinitionNServiceVersion(testCtx.Ctx, testCtx.Cli, compDefName("v2"), "")
			Expect(err).Should(Succeed())
			Expect(compDef.Name).Should(Equal(compDefName("v2.0")))
			Expect(resolvedServiceVersion).Should(Equal(serviceVersion("v1")))
			Expect(compDef.Spec.Runtime.Containers).Should(HaveLen(2))
			Expect(compDef.Spec.Runtime.Containers[0].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[0].Name, releaseId("r3"))))
			Expect(compDef.Spec.Runtime.Containers[1].Image).Should(Equal(appImage(compDef.Spec.Runtime.Containers[1].Name, releaseId("r2"))))
		})
	})

	Context("component definition images", func() {
		BeforeEach(func() {
			// compDefinitionObjs = createCompDefinitionObjs()
			createCompDefinitionObjs()
			compVersionObj = createCompVersionObj()
		})

		AfterEach(func() {
			cleanEnv()
		})
	})
})
