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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var _ = Describe("synthesized component", func() {
	var (
		reqCtx = intctrlutil.RequestCtx{
			Ctx: testCtx.Ctx,
			Log: logger,
		}
		cli     client.Reader
		compDef *appsv1alpha1.ComponentDefinition
		comp    *appsv1alpha1.Component
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// inNS := client.InNamespace(testCtx.DefaultNamespace)
		// ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		// testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterDefinitionSignature, true, ml)

		// namespaced
		// testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("config template", func() {
		BeforeEach(func() {
			compDef = &appsv1alpha1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-compdef",
				},
				Spec: appsv1alpha1.ComponentDefinitionSpec{
					Configs: []appsv1alpha1.ComponentConfigSpec{
						{
							ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
								Name:        "app",
								TemplateRef: "app",
								VolumeName:  "app",
							},
						},
						{
							ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
								Name:       "external",
								VolumeName: "external",
							},
						},
					},
				},
			}
			comp = &appsv1alpha1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-comp",
					Labels: map[string]string{
						constant.AppInstanceLabelKey:     "test-cluster",
						constant.KBAppClusterUIDLabelKey: "uuid",
					},
					Annotations: map[string]string{
						constant.KubeBlocksGenerationKey: "1",
					},
				},
				Spec: appsv1alpha1.ComponentSpec{
					Configs: []appsv1alpha1.ClusterComponentConfig{},
				},
			}
		})

		It("comp def", func() {
			synthesizedComp, err := buildSynthesizedComponent(reqCtx, cli, compDef, comp, nil, nil, nil)
			Expect(err).Should(BeNil())

			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.ConfigTemplates).Should(BeEquivalentTo(compDef.Spec.Configs))
		})

		It("w/ comp override - ok", func() {
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1alpha1.ClusterComponentConfig{
				Name: func() *string { name := "external"; return &name }(),
				ClusterComponentConfigSource: appsv1alpha1.ClusterComponentConfigSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "external-cm",
						},
					},
				},
			})
			synthesizedComp, err := buildSynthesizedComponent(reqCtx, cli, compDef, comp, nil, nil, nil)
			Expect(err).Should(BeNil())

			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.ConfigTemplates[0]).Should(BeEquivalentTo(compDef.Spec.Configs[0]))

			expectExternalConfig := compDef.Spec.Configs[1]
			expectExternalConfig.TemplateRef = comp.Spec.Configs[0].ConfigMap.Name
			Expect(synthesizedComp.ConfigTemplates[1]).Should(BeEquivalentTo(expectExternalConfig))
		})

		It("w/ comp override - not defined", func() {
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1alpha1.ClusterComponentConfig{
				Name: func() *string { name := "not-defined"; return &name }(),
				ClusterComponentConfigSource: appsv1alpha1.ClusterComponentConfigSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "external-cm",
						},
					},
				},
			})
			_, err := buildSynthesizedComponent(reqCtx, cli, compDef, comp, nil, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("not defined in definition"))
		})

		It("w/ comp override - both specified", func() {
			compDef.Spec.Configs[1].TemplateRef = "external"
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1alpha1.ClusterComponentConfig{
				Name: func() *string { name := "external"; return &name }(),
				ClusterComponentConfigSource: appsv1alpha1.ClusterComponentConfigSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "external-cm",
						},
					},
				},
			})
			_, err := buildSynthesizedComponent(reqCtx, cli, compDef, comp, nil, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("partial overriding is not supported"))
		})

		It("w/ comp override - both not specified", func() {
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1alpha1.ClusterComponentConfig{
				Name:                         func() *string { name := "external"; return &name }(),
				ClusterComponentConfigSource: appsv1alpha1.ClusterComponentConfigSource{},
			})
			_, err := buildSynthesizedComponent(reqCtx, cli, compDef, comp, nil, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("there is no content provided for config template"))
		})
	})
})
