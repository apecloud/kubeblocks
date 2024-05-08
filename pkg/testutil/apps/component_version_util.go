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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/testutil"
)

const (
	CompDefinitionName = "test-component-definition"
	CompVersionName    = "test-component-version"

	AppName           = "app"
	AppNameSamePrefix = "app-same-prefix"

	ReleasePrefix        = "v0.0.1"
	ServiceVersionPrefix = "8.0.30"
)

var (

	// compDefinitionObjs []*appsv1alpha1.ComponentDefinition
	//compVersionObj *appsv1alpha1.ComponentVersion

	CompDefNames    = []string{CompDefName("v1.0"), CompDefName("v1.1"), CompDefName("v2.0"), CompDefName("v3.0")}
	ServiceVersions = []string{ServiceVersion("v1"), ServiceVersion("v2"), ServiceVersion("v3")}
)

func AppImage(app, tag string) string {
	return fmt.Sprintf("%s:%s", app, tag)
}
func CompDefName(r string) string {
	return fmt.Sprintf("%s-%s", CompDefinitionName, r)
}
func ReleaseID(r string) string {
	return fmt.Sprintf("%s-%s", ReleasePrefix, r)
}
func ServiceVersion(r string) string {
	if len(r) == 0 {
		return ServiceVersionPrefix
	}
	return fmt.Sprintf("%s-%s", ServiceVersionPrefix, r)
}

func CreateCompDefinitionObjs(testCtx *testutil.TestContext, checkGeneration bool) []*appsv1alpha1.ComponentDefinition {
	By("create default ComponentDefinition objs")
	objs := make([]*appsv1alpha1.ComponentDefinition, 0)
	for _, name := range CompDefNames {
		f := NewComponentDefinitionFactory(name).
			SetServiceVersion(ServiceVersion("v0")) // use v0 as init service version
		for _, app := range []string{AppName, AppNameSamePrefix} {
			// use empty revision as init image tag
			f = f.SetRuntime(&corev1.Container{Name: app, Image: AppImage(app, ReleaseID(""))})
		}
		objs = append(objs, f.Create(testCtx).GetObject())
	}
	if checkGeneration {
		for _, obj := range objs {
			Eventually(CheckObj(testCtx, client.ObjectKeyFromObject(obj),
				func(g Gomega, compDef *appsv1alpha1.ComponentDefinition) {
					g.Expect(compDef.Status.ObservedGeneration).Should(Equal(compDef.Generation))
				})).Should(Succeed())
		}
	}
	return objs
}

func CreateCompVersionObj(testCtx *testutil.TestContext, checkGeneration bool) *appsv1alpha1.ComponentVersion {
	By("create a default ComponentVersion obj with multiple releases")
	var obj = NewComponentVersionFactory(CompVersionName).
		SetSpec(appsv1alpha1.ComponentVersionSpec{
			CompatibilityRules: []appsv1alpha1.ComponentVersionCompatibilityRule{
				{
					// use prefix
					CompDefs: []string{CompDefName("v1"), CompDefName("v2")},
					Releases: []string{ReleaseID("r0"), ReleaseID("r1"), ReleaseID("r2"), ReleaseID("r3"), ReleaseID("r4")}, // sv: v1, v2
				},
				{
					// use prefix
					CompDefs: []string{CompDefName("v3")},
					Releases: []string{ReleaseID("r5")}, // sv: v3
				},
			},
			Releases: []appsv1alpha1.ComponentVersionRelease{
				{
					Name:           ReleaseID("r0"),
					Changes:        "init release",
					ServiceVersion: ServiceVersion("v1"),
					Images: map[string]string{
						AppName:           AppImage(AppName, ReleaseID("r0")),
						AppNameSamePrefix: AppImage(AppNameSamePrefix, ReleaseID("r0")),
					},
				},
				{
					Name:           ReleaseID("r1"),
					Changes:        "update app image",
					ServiceVersion: ServiceVersion("v1"),
					Images: map[string]string{
						AppName: AppImage(AppName, ReleaseID("r1")),
					},
				},
				{
					Name:           ReleaseID("r2"),
					Changes:        "publish a new service version",
					ServiceVersion: ServiceVersion("v2"),
					Images: map[string]string{
						AppName:           AppImage(AppName, ReleaseID("r2")),
						AppNameSamePrefix: AppImage(AppNameSamePrefix, ReleaseID("r2")),
					},
				},
				{
					Name:           ReleaseID("r3"),
					Changes:        "update app image",
					ServiceVersion: ServiceVersion("v2"),
					Images: map[string]string{
						AppName: AppImage(AppName, ReleaseID("r3")),
					},
				},
				{
					Name:           ReleaseID("r4"),
					Changes:        "update all app images for previous service version",
					ServiceVersion: ServiceVersion("v1"),
					Images: map[string]string{
						AppName:           AppImage(AppName, ReleaseID("r4")),
						AppNameSamePrefix: AppImage(AppNameSamePrefix, ReleaseID("r4")),
					},
				},
				{
					Name:           ReleaseID("r5"),
					Changes:        "publish a new service version",
					ServiceVersion: ServiceVersion("v3"),
					Images: map[string]string{
						AppName:           AppImage(AppName, ReleaseID("r5")),
						AppNameSamePrefix: AppImage(AppNameSamePrefix, ReleaseID("r5")),
					},
				},
			},
		}).
		Create(testCtx).
		GetObject()
	if checkGeneration {
		Eventually(CheckObj(testCtx, client.ObjectKeyFromObject(obj),
			func(g Gomega, compVersion *appsv1alpha1.ComponentVersion) {
				g.Expect(compVersion.Status.ObservedGeneration).Should(Equal(compVersion.Generation))
			})).Should(Succeed())
	}
	return obj
}
