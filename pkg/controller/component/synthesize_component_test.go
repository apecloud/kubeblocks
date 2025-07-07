/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("synthesized component", func() {
	var (
		cli     client.Reader
		compDef *appsv1.ComponentDefinition
		comp    *appsv1.Component
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("file templates", func() {
		BeforeEach(func() {
			compDef = &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-compdef",
				},
				Spec: appsv1.ComponentDefinitionSpec{
					Configs: []appsv1.ComponentFileTemplate{
						{
							Name:       "logConf",
							Template:   "logConf",
							VolumeName: "logConf",
						},
						{
							Name:       "serverConf",
							Template:   "serverConf",
							VolumeName: "serverConf",
						},
					},
				},
			}
			comp = &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "test-cluster-comp",
					Labels: map[string]string{
						constant.AppInstanceLabelKey: "test-cluster",
					},
					Annotations: map[string]string{
						constant.KBAppClusterUIDKey:      "uuid",
						constant.KubeBlocksGenerationKey: "1",
					},
				},
				Spec: appsv1.ComponentSpec{
					Configs: []appsv1.ClusterComponentConfig{},
				},
			}
		})

		It("ok", func() {
			synthesizedComp, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).Should(BeNil())

			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: compDef.Spec.Configs[0],
				Config:                true,
			}))
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: compDef.Spec.Configs[1],
				Config:                true,
			}))
		})

		It("override", func() {
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1.ClusterComponentConfig{
				Name: ptr.To(compDef.Spec.Configs[1].Name),
				ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "external-server-conf",
						},
					},
				},
				Reconfigure: &appsv1.Action{
					Exec: &appsv1.ExecAction{
						Command: []string{"echo", "external", "reconfigure"},
					},
				},
			})
			synthesizedComp, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).Should(BeNil())

			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: compDef.Spec.Configs[0],
				Config:                true,
			}))
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name:       compDef.Spec.Configs[1].Name,
					Template:   comp.Spec.Configs[0].ConfigMap.Name,
					Namespace:  comp.Namespace,
					VolumeName: compDef.Spec.Configs[1].VolumeName,
				},
				Config:      true,
				Reconfigure: comp.Spec.Configs[0].Reconfigure,
			}))
		})

		It("external managed", func() {
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1.ClusterComponentConfig{
				Name: ptr.To(compDef.Spec.Configs[1].Name),
				ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "external-server-conf",
						},
					},
				},
				Reconfigure: &appsv1.Action{
					Exec: &appsv1.ExecAction{
						Command: []string{"echo", "external", "reconfigure"},
					},
				},
				ExternalManaged: ptr.To(true),
			})
			synthesizedComp, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).Should(BeNil())

			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: compDef.Spec.Configs[0],
				Config:                true,
			}))
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name:            compDef.Spec.Configs[1].Name,
					Template:        comp.Spec.Configs[0].ConfigMap.Name,
					Namespace:       comp.Namespace,
					VolumeName:      compDef.Spec.Configs[1].VolumeName,
					ExternalManaged: comp.Spec.Configs[0].ExternalManaged,
				},
				Config:      true,
				Reconfigure: comp.Spec.Configs[0].Reconfigure,
			}))
		})

		It("external managed - lazy provision", func() {
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1.ClusterComponentConfig{
				Name:            ptr.To(compDef.Spec.Configs[1].Name),
				ExternalManaged: ptr.To(true),
			})
			synthesizedComp, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).Should(BeNil())

			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: compDef.Spec.Configs[0],
				Config:                true,
			}))
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name:            compDef.Spec.Configs[1].Name,
					Template:        "",
					Namespace:       "",
					VolumeName:      compDef.Spec.Configs[1].VolumeName,
					ExternalManaged: comp.Spec.Configs[0].ExternalManaged,
				},
				Config:      true,
				Reconfigure: comp.Spec.Configs[0].Reconfigure,
			}))
		})
	})

	Context("env", func() {
		BeforeEach(func() {
			compDef = &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-compdef",
				},
				Spec: appsv1.ComponentDefinitionSpec{
					Runtime: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "app",
								Env: []corev1.EnvVar{
									{
										Name:  "key",
										Value: "value",
									},
								},
							},
						},
					},
				},
			}
			comp = &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "test-cluster-comp",
					Labels: map[string]string{
						constant.AppInstanceLabelKey: "test-cluster",
					},
					Annotations: map[string]string{
						constant.KBAppClusterUIDKey:      "uuid",
						constant.KubeBlocksGenerationKey: "1",
					},
				},
				Spec: appsv1.ComponentSpec{
					Env: []corev1.EnvVar{
						{
							Name:  "ukey",
							Value: "uvalue",
						},
					},
				},
			}
		})

		It("duplicated", func() {
			comp.Spec.Env = append(comp.Spec.Env, comp.Spec.Env[0])

			_, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("duplicated user-defined env var"))
		})

		It("ok", func() {
			synthesizedComp, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).Should(BeNil())
			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.PodSpec.Containers[0].Env).Should(HaveLen(2))
			Expect(synthesizedComp.PodSpec.Containers[0].Env[1]).Should(BeEquivalentTo(comp.Spec.Env[0]))
		})
	})

	Context("volumes", func() {
		BeforeEach(func() {
			compDef = &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-compdef",
				},
				Spec: appsv1.ComponentDefinitionSpec{
					Runtime: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "config",
							},
						},
						Containers: []corev1.Container{
							{
								Name: "app",
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "config",
										MountPath: "/config",
									},
									{
										Name:      "data",
										MountPath: "/data",
									},
									{
										Name:      "log",
										MountPath: "/log",
									},
								},
							},
						},
					},
				},
			}
			comp = &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "test-cluster-comp",
					Labels: map[string]string{
						constant.AppInstanceLabelKey: "test-cluster",
					},
					Annotations: map[string]string{
						constant.KBAppClusterUIDKey:      "uuid",
						constant.KubeBlocksGenerationKey: "1",
					},
				},
				Spec: appsv1.ComponentSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
						},
						{
							Name: "log",
						},
						{
							Name: "not-defined",
						},
					},
				},
			}
		})

		It("duplicated", func() {
			comp.Spec.Volumes = append(comp.Spec.Volumes, comp.Spec.Volumes[0])

			_, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("duplicated volume"))
		})

		It("duplicated with definition", func() {
			comp.Spec.Volumes = append(comp.Spec.Volumes, compDef.Spec.Runtime.Volumes[0])

			_, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("duplicated volume"))
		})

		It("ok", func() {
			synthesizedComp, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).Should(BeNil())
			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.PodSpec.Volumes).Should(HaveLen(4))
			Expect(synthesizedComp.PodSpec.Volumes[0].Name).Should(Equal("config"))
			Expect(synthesizedComp.PodSpec.Volumes[1].Name).Should(Equal("data"))
			Expect(synthesizedComp.PodSpec.Volumes[2].Name).Should(Equal("log"))
			Expect(synthesizedComp.PodSpec.Volumes[3].Name).Should(Equal("not-defined"))
		})
	})

	Context("components and definitions", func() {
		var (
			reader       *mockReader
			cluster      *appsv1.Cluster
			comp1, comp2 *appsv1.Component
			shardComp1   *appsv1.Component
		)

		BeforeEach(func() {
			compDef = &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-compdef-a",
				},
				Spec: appsv1.ComponentDefinitionSpec{
					ServiceVersion: "8.0.30",
					Runtime: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "app",
							},
						},
					},
				},
			}
			cluster = &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "test-cluster",
				},
				Spec: appsv1.ClusterSpec{
					ComponentSpecs: []appsv1.ClusterComponentSpec{
						{
							Name:         "comp1",
							ComponentDef: "test-compdef-a",
						},
						{
							Name:         "comp2",
							ComponentDef: "test-compdef-b",
						},
					},
					Shardings: []appsv1.ClusterSharding{
						{
							Name:   "sharding1",
							Shards: 3,
							Template: appsv1.ClusterComponentSpec{
								ComponentDef: "test-compdef-a",
							},
						},
						{
							Name:   "sharding2",
							Shards: 5,
							Template: appsv1.ClusterComponentSpec{
								ComponentDef: "test-compdef-c",
							},
						},
					},
				},
			}
			comp1 = &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "test-cluster-comp1",
					Labels:    constant.GetClusterLabels("test-cluster"),
					Annotations: map[string]string{
						constant.KBAppClusterUIDKey:      "uuid",
						constant.KubeBlocksGenerationKey: "1",
					},
				},
				Spec: appsv1.ComponentSpec{
					CompDef:        "test-compdef-a",
					ServiceVersion: "8.0.30",
				},
			}
			comp2 = &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "test-cluster-comp2",
					Labels:    constant.GetClusterLabels("test-cluster"),
					Annotations: map[string]string{
						constant.KBAppClusterUIDKey:      "uuid",
						constant.KubeBlocksGenerationKey: "1",
					},
				},
				Spec: appsv1.ComponentSpec{
					CompDef:        "test-compdef-b",
					ServiceVersion: "8.0.30",
				},
			}
			shardComp1 = &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "test-cluster-sharding1-a",
					Labels: constant.GetClusterLabels("test-cluster",
						map[string]string{constant.KBAppShardingNameLabelKey: "sharding1"}),
					Annotations: map[string]string{
						constant.KBAppClusterUIDKey:      "uuid",
						constant.KubeBlocksGenerationKey: "1",
					},
				},
				Spec: appsv1.ComponentSpec{
					CompDef:        "test-compdef-a",
					ServiceVersion: "8.0.30",
				},
			}
			reader = &mockReader{
				cli: k8sClient,
			}
		})

		It("buildComp2CompDefs", func() {
			reader.objs = []client.Object{comp1, comp2}

			synthesizedComp, err := BuildSynthesizedComponent(ctx, reader, compDef, comp1)
			Expect(err).Should(BeNil())
			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.Comp2CompDefs).Should(HaveLen(2))
			Expect(synthesizedComp.Comp2CompDefs).Should(HaveKeyWithValue("comp1", "test-compdef-a"))
			Expect(synthesizedComp.Comp2CompDefs).Should(HaveKeyWithValue("comp2", "test-compdef-b"))
		})

		It("buildComp2CompDefs - with sharding comp", func() {
			reader.objs = []client.Object{comp1, comp2, shardComp1}

			synthesizedComp, err := BuildSynthesizedComponent(ctx, reader, compDef, comp1)
			Expect(err).Should(BeNil())
			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.Comp2CompDefs).Should(HaveLen(3))
			Expect(synthesizedComp.Comp2CompDefs).Should(HaveKeyWithValue("comp1", "test-compdef-a"))
			Expect(synthesizedComp.Comp2CompDefs).Should(HaveKeyWithValue("comp2", "test-compdef-b"))
			Expect(synthesizedComp.Comp2CompDefs).Should(HaveKeyWithValue("sharding1-a", "test-compdef-a"))
		})

		It("buildCompDef2CompCount", func() {
			reader.objs = []client.Object{cluster, comp1}

			synthesizedComp, err := BuildSynthesizedComponent(ctx, reader, compDef, comp1)
			Expect(err).Should(BeNil())
			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.CompDef2CompCnt).Should(HaveLen(3))
			Expect(synthesizedComp.CompDef2CompCnt).Should(HaveKeyWithValue("test-compdef-a", int32(4)))
			Expect(synthesizedComp.CompDef2CompCnt).Should(HaveKeyWithValue("test-compdef-b", int32(1)))
			Expect(synthesizedComp.CompDef2CompCnt).Should(HaveKeyWithValue("test-compdef-c", int32(5)))
		})

		It("roll back serviceaccount change", func() {
			lastCompDef := compDef.DeepCopy()
			lastCompDef.Name = "last-comp-def"
			comp1.Labels[constant.LastComponentDefinitionLabelKey] = lastCompDef.Name
			reader.objs = []client.Object{comp1, lastCompDef}

			synthesizedComp, err := BuildSynthesizedComponent(ctx, reader, compDef, comp1)
			Expect(err).Should(BeNil())
			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.ServiceAccountName).Should(Equal(constant.GenerateDefaultServiceAccountName(lastCompDef.Name)))
		})

		It("does not roll back serviceaccount change when roles change", func() {
			lastCompDef := compDef.DeepCopy()
			lastCompDef.Name = "last-comp-def"
			lastCompDef.Spec.LifecycleActions = &appsv1.ComponentLifecycleActions{
				RoleProbe: &appsv1.Probe{},
			}
			comp1.Labels[constant.LastComponentDefinitionLabelKey] = lastCompDef.Name
			reader.objs = []client.Object{comp1, lastCompDef}

			synthesizedComp, err := BuildSynthesizedComponent(ctx, reader, compDef, comp1)
			Expect(err).Should(BeNil())
			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.ServiceAccountName).Should(Equal(constant.GenerateDefaultServiceAccountName(compDef.Name)))
		})
	})
})
