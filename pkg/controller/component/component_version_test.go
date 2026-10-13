/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Component Version", func() {

	Context("test component version function", func() {
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

		AfterEach(func() {
			cleanEnv()
		})

		It("checkNMergeImages", func() {
			appsInDef := map[string]appNameVersionImage{
				"action1": {name: "action1", required: false},
				"image1":  {name: "image1", version: "1.0.0", image: "wrong", required: true},
				"image2":  {name: "image2", version: "1.0.1", required: true}, // this image has different serviceVersion
			}
			appsInVer := map[string]appNameVersionImage{
				"image1": {name: "image1", version: "1.0.0", image: "right", required: true}, // this image overrides the one in appsInDef
			}
			apps := checkNMergeImages("1.0.0", appsInDef, appsInVer)
			Expect(apps["image1"].err).NotTo(HaveOccurred())
			Expect(apps["image1"].image).To(Equal("right"))

			Expect(apps["image2"].err).To(HaveOccurred())

			Expect(apps["action1"].err).NotTo(HaveOccurred())
		})

		It("resolve images before and after new release", func() {
			By("create new definition v4.0 with service version v4")
			compDefObj := testapps.NewComponentDefinitionFactory(testapps.CompDefName("v1")).
				SetServiceVersion(testapps.ServiceVersion("v0")).
				SetRuntime(&corev1.Container{Name: testapps.AppName,
					Image: testapps.AppImage(testapps.AppName, testapps.ReleaseID(""))}).
				GetObject()

			releases := []appsv1.ComponentVersionRelease{
				{
					Name:           testapps.ReleaseID("r0"),
					Changes:        "init release in v1",
					ServiceVersion: testapps.ServiceVersion("v1"),
					Images: map[string]string{
						testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r0")),
					},
				},
				{
					Name:           testapps.ReleaseID("r1"),
					Changes:        "new release in v2",
					ServiceVersion: testapps.ServiceVersion("v2"), // has different service version
					Images: map[string]string{
						testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r1")),
					},
				},
				{
					Name:           testapps.ReleaseID("r2"),
					Changes:        "new release in v1",
					ServiceVersion: testapps.ServiceVersion("v1"), // has same service version
					Images: map[string]string{
						testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r2")),
					},
				},
			}

			By("first release for the definition")
			compVersionObj := testapps.NewComponentVersionFactory(testapps.CompVersionName).
				SetSpec(appsv1.ComponentVersionSpec{
					CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
						{
							CompDefs: []string{testapps.CompDefName("v1")},
							Releases: []string{releases[0].Name},
						},
					},
					Releases: []appsv1.ComponentVersionRelease{releases[0]},
				}).
				GetObject()

			By("with app image in r0")
			err := resolveImagesWithCompVersions(compDefObj, []*appsv1.ComponentVersion{compVersionObj}, testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDefObj.Spec.Runtime.Containers[0].Image).Should(Equal(releases[0].Images[testapps.AppName]))

			By("publish a new release which has different service version")
			compVersionObj.Spec.Releases = append(compVersionObj.Spec.Releases, releases[1])
			compVersionObj.Spec.CompatibilityRules[0].Releases = append(compVersionObj.Spec.CompatibilityRules[0].Releases, releases[1].Name)

			By("with app image still in r0")
			err = resolveImagesWithCompVersions(compDefObj, []*appsv1.ComponentVersion{compVersionObj}, testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDefObj.Spec.Runtime.Containers[0].Image).Should(Equal(releases[0].Images[testapps.AppName]))

			By("publish a new release which has same service version")
			compVersionObj.Spec.Releases = append(compVersionObj.Spec.Releases, releases[2])
			compVersionObj.Spec.CompatibilityRules[0].Releases = append(compVersionObj.Spec.CompatibilityRules[0].Releases, releases[2].Name)

			By("with app image in r2")
			err = resolveImagesWithCompVersions(compDefObj, []*appsv1.ComponentVersion{compVersionObj}, testapps.ServiceVersion("v1"))
			Expect(err).Should(Succeed())
			Expect(compDefObj.Spec.Runtime.Containers[0].Image).Should(Equal(releases[2].Images[testapps.AppName]))
		})

		It("exact matched service version", func() {
			compDefObj := testapps.NewComponentDefinitionFactory(testapps.CompDefName("v1")).
				SetRuntime(&corev1.Container{Name: testapps.AppName}).
				GetObject()

			releases := []appsv1.ComponentVersionRelease{
				{
					Name:           testapps.ReleaseID("r0"),    // v0.0.1-r0
					ServiceVersion: testapps.ServiceVersion(""), // 8.0.30
					Images: map[string]string{
						testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r0")),
					},
				},
				{
					Name:           testapps.ReleaseID("r0-opt"),   // // v0.0.1-r0-opt, has a newer release name
					ServiceVersion: testapps.ServiceVersion("opt"), // 8.0.30-opt
					Images: map[string]string{
						testapps.AppName: testapps.AppImage(testapps.AppName, testapps.ReleaseID("r0-opt")),
					},
				},
			}

			compVersionObj := testapps.NewComponentVersionFactory(testapps.CompVersionName).
				SetSpec(appsv1.ComponentVersionSpec{
					CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
						{
							CompDefs: []string{compDefObj.Name},
							Releases: []string{releases[0].Name, releases[1].Name},
						},
					},
					Releases: []appsv1.ComponentVersionRelease{releases[0], releases[1]},
				}).
				GetObject()

			By("resolve images with service version 8.0.30")
			err := resolveImagesWithCompVersions(compDefObj, []*appsv1.ComponentVersion{compVersionObj}, testapps.ServiceVersion(""))
			Expect(err).Should(Succeed())
			Expect(compDefObj.Spec.Runtime.Containers[0].Image).Should(Equal(releases[0].Images[testapps.AppName]))
		})

		It("matched from different service versions", func() {
			var (
				app1, app2 = "app1", "app2"
			)

			compDefObj := testapps.NewComponentDefinitionFactory(testapps.CompDefName("v1")).
				SetRuntime(&corev1.Container{Name: app1}).
				SetRuntime(&corev1.Container{Name: app2}).
				GetObject()

			releases := []appsv1.ComponentVersionRelease{
				{
					Name:           testapps.ReleaseID("r0"),    // v0.0.1-r0
					ServiceVersion: testapps.ServiceVersion(""), // 8.0.30
					Images: map[string]string{
						app1: testapps.AppImage(app1, testapps.ReleaseID("r0")),
					},
				},
				{
					Name:           testapps.ReleaseID("r0-opt"),   // // v0.0.1-r0-opt, has a newer release name
					ServiceVersion: testapps.ServiceVersion("opt"), // 8.0.30-opt
					Images: map[string]string{
						app1: testapps.AppImage(app1, testapps.ReleaseID("r0-opt")),
						app2: testapps.AppImage(app2, testapps.ReleaseID("r0-opt")),
					},
				},
			}

			compVersionObj := testapps.NewComponentVersionFactory(testapps.CompVersionName).
				SetSpec(appsv1.ComponentVersionSpec{
					CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
						{
							CompDefs: []string{compDefObj.Name},
							Releases: []string{releases[0].Name, releases[1].Name},
						},
					},
					Releases: []appsv1.ComponentVersionRelease{releases[0], releases[1]},
				}).
				GetObject()

			By("resolve images with service version 8.0.30")
			err := resolveImagesWithCompVersions(compDefObj, []*appsv1.ComponentVersion{compVersionObj}, testapps.ServiceVersion(""))
			Expect(err).ShouldNot(BeNil())
		})
	})
})

var _ = Describe("instance template image resolution", func() {
	It("resolves init containers and actions without changing the definition", func() {
		def := testapps.NewComponentDefinitionFactory("image-resolution").SetServiceVersion("1.0.0").
			SetRuntime(&corev1.Container{Name: "db", Image: "db:1"}).GetObject()
		def.Spec.Runtime.InitContainers = []corev1.Container{{Name: "setup", Image: "setup:1"}}
		def.Spec.LifecycleActions = &appsv1.ComponentLifecycleActions{PostProvision: &appsv1.Action{Exec: &appsv1.ExecAction{Image: "action:1"}}}
		ver := &appsv1.ComponentVersion{Spec: appsv1.ComponentVersionSpec{Releases: []appsv1.ComponentVersionRelease{
			{Name: "r2", ServiceVersion: "2.0.0", Images: map[string]string{"db": "db:2", "setup": "setup:2", "postProvision": "action:2"}},
		}}}
		before := def.DeepCopy()
		By("returning no overrides when no compatible ComponentVersion exists")
		images, err := ResolveInstanceTemplateImages4ServiceVersion(ctx, testCtx.Cli, def, "2.0.0")
		Expect(err).Should(Succeed())
		Expect(images).Should(BeNil())
		images, err = resolveImagesWithCompVersions4Template(def, []*appsv1.ComponentVersion{ver}, "2.0.0")
		Expect(err).Should(Succeed())
		Expect(images).Should(Equal(map[string]string{"db": "db:2", "setup": "setup:2", kbagent.ContainerName: "action:2", kbagent.ContainerName4Worker: "action:2"}))
		Expect(def).Should(Equal(before))
		By("using the configured tools image for actions without an explicit image")
		oldTools := viper.GetString(constant.KBToolsImage)
		defer viper.Set(constant.KBToolsImage, oldTools)
		viper.Set(constant.KBToolsImage, "tools:default")
		def.Spec.LifecycleActions.PostProvision.Exec.Image = ""
		delete(ver.Spec.Releases[0].Images, "postProvision")
		images, err = resolveImagesWithCompVersions4Template(def, []*appsv1.ComponentVersion{ver}, "2.0.0")
		Expect(err).Should(Succeed())
		Expect(images[kbagent.ContainerName]).Should(Equal("tools:default"))
		Expect(images[kbagent.ContainerName4Worker]).Should(Equal("tools:default"))
		By("retaining the definition image for an action absent from the release")
		def.Spec.LifecycleActions.PostProvision.Exec.Image = "action:fixed"
		images, err = resolveImagesWithCompVersions4Template(def, []*appsv1.ComponentVersion{ver}, "2.0.0")
		Expect(err).Should(Succeed())
		Expect(images[kbagent.ContainerName]).Should(Equal("action:fixed"))
	})
})
