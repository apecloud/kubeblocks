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
	"context"
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestInstanceFileTemplates(t *testing.T) {
	action := &appsv1.Action{Exec: &appsv1.ExecAction{Command: []string{"reload"}}}
	comp := &SynthesizedComponent{
		Namespace: "component-ns",
		FileTemplates: []SynthesizedFileTemplate{
			{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name: "server", Template: "default-source", Namespace: "definition-ns", VolumeName: "config",
					DefaultMode: ptr.To(int32(0440)), RestartOnFileChange: ptr.To(true), Reconfigure: action,
				},
				Config: true, Variables: map[string]string{"keep": "parent", "override": "parent"},
				ConfigHash: ptr.To("parent-hash"), ReconfigureRequired: ptr.To(true), ReconfigureAction: action,
				ReconfigureArgs: [][]string{{"--reload"}},
			},
			{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "script", Template: "script-source"}},
			{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "external", ExternalManaged: ptr.To(true)}, Config: true},
		},
	}
	instance := &appsv1.InstanceTemplate{Name: "large", Configs: []appsv1.ClusterComponentConfig{{
		Name: ptr.To("server"), Variables: map[string]string{"override": "instance", "add": "instance"},
		ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "instance-source"},
		}},
	}}}
	templates, err := InstanceFileTemplates(comp, instance)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 1 {
		t.Fatalf("got %d templates, want one managed config", len(templates))
	}
	want := comp.FileTemplates[0]
	want.Namespace, want.Template = comp.Namespace, "instance-source"
	want.Variables = map[string]string{"keep": "parent", "override": "instance", "add": "instance"}
	if !reflect.DeepEqual(templates[0], want) {
		t.Fatalf("effective template differs: got %#v, want %#v", templates[0], want)
	}
	templates[0].Variables["keep"] = "changed"
	templates[0].Variables["add"] = "changed"
	if comp.FileTemplates[0].Variables["keep"] != "parent" || instance.Configs[0].Variables["add"] != "instance" {
		t.Fatal("derived variables alias Component or instance inputs")
	}
	inherited, err := InstanceFileTemplates(comp, &appsv1.InstanceTemplate{Name: "inherit"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inherited) != 1 || !reflect.DeepEqual(inherited[0], comp.FileTemplates[0]) {
		t.Fatalf("empty overrides did not inherit managed configuration: %#v", inherited)
	}
	inherited[0].Variables["keep"] = "changed"
	if comp.FileTemplates[0].Variables["keep"] != "parent" {
		t.Fatal("inherited variables alias Component inputs")
	}
}

func TestInstanceFileTemplatesRejectsInvalidOverrides(t *testing.T) {
	config := func(name string) appsv1.ClusterComponentConfig {
		return appsv1.ClusterComponentConfig{Name: ptr.To(name)}
	}
	for _, tc := range []struct {
		name   string
		modify func(*appsv1.ClusterComponentConfig)
		extra  bool
		want   string
	}{
		{name: "missing name", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = nil }, want: "name is required"},
		{name: "empty name", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = ptr.To("") }, want: "name is required"},
		{name: "unknown name", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = ptr.To("unknown") }, want: "not defined"},
		{name: "script target", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = ptr.To("script") }, want: "not defined"},
		{name: "external target", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = ptr.To("external") }, want: "externally managed"},
		{name: "duplicate name", extra: true, want: "duplicated"},
		{name: "external managed", modify: func(c *appsv1.ClusterComponentConfig) { c.ExternalManaged = ptr.To(false) }, want: "accepts only"},
		{name: "config hash", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigHash = ptr.To("") }, want: "accepts only"},
		{name: "restart", modify: func(c *appsv1.ClusterComponentConfig) { c.Restart = ptr.To(false) }, want: "accepts only"},
		{name: "reconfigure", modify: func(c *appsv1.ClusterComponentConfig) { c.Reconfigure = ptr.To(false) }, want: "accepts only"},
		{name: "action", modify: func(c *appsv1.ClusterComponentConfig) { c.ReconfigureAction = &appsv1.Action{} }, want: "accepts only"},
		{name: "args", modify: func(c *appsv1.ClusterComponentConfig) { c.ReconfigureArgs = [][]string{} }, want: "accepts only"},
		{name: "empty source", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigMap = &corev1.ConfigMapVolumeSource{} }, want: "invalid configMap.name"},
		{name: "invalid source", modify: func(c *appsv1.ClusterComponentConfig) {
			c.ConfigMap = &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "Invalid/Source"}}
		}, want: "invalid configMap.name"},
		{name: "source items", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigMap.Items = []corev1.KeyToPath{} }, want: "accepts only configMap.name"},
		{name: "source mode", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigMap.DefaultMode = ptr.To(int32(0444)) }, want: "accepts only configMap.name"},
		{name: "source optional", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigMap.Optional = ptr.To(false) }, want: "accepts only configMap.name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			comp := &SynthesizedComponent{FileTemplates: []SynthesizedFileTemplate{
				{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "server", Template: "source"}, Config: true},
				{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "script"}},
				{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "external", ExternalManaged: ptr.To(true)}, Config: true},
			}}
			override := config("server")
			override.ConfigMap = &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "source"}}
			if tc.modify != nil {
				tc.modify(&override)
			}
			instance := &appsv1.InstanceTemplate{Name: "large", Configs: []appsv1.ClusterComponentConfig{override}}
			if tc.extra {
				instance.Configs = append(instance.Configs, config("server"))
			}
			_, err := InstanceFileTemplates(comp, instance)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got error %v, want %q", err, tc.want)
			}
		})
	}
}

func TestBuildSynthesizedComponentValidatesInactiveInstanceConfigs(t *testing.T) {
	comp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-db", Namespace: "default",
			Labels:      map[string]string{constant.AppInstanceLabelKey: "cluster"},
			Annotations: map[string]string{constant.KBAppClusterUIDKey: "uid"},
		},
		Spec: appsv1.ComponentSpec{Instances: []appsv1.InstanceTemplate{{
			Name: "inactive", Replicas: ptr.To(int32(0)),
			Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("unknown")}},
		}}},
	}
	def := &appsv1.ComponentDefinition{Spec: appsv1.ComponentDefinitionSpec{
		Runtime: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:1"}}},
	}}
	_, err := BuildSynthesizedComponent(context.Background(), nil, def, comp)
	if err == nil || !strings.Contains(err.Error(), "instance template inactive: config template unknown is not defined") {
		t.Fatalf("got error %v, want invalid zero-replica config declaration", err)
	}
}

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
				Reconfigure: ptr.To(true),
				ReconfigureAction: &appsv1.Action{
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
				Config:              true,
				ReconfigureRequired: ptr.To(true),
				ReconfigureAction:   comp.Spec.Configs[0].ReconfigureAction,
			}))
		})

		It("action override does not imply reconfigure", func() {
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1.ClusterComponentConfig{
				Name: ptr.To(compDef.Spec.Configs[1].Name),
				ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "external-server-conf",
						},
					},
				},
				ReconfigureAction: &appsv1.Action{
					Exec: &appsv1.ExecAction{
						Command: []string{"echo", "external", "reconfigure"},
					},
				},
			})
			synthesizedComp, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).Should(BeNil())

			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name:       compDef.Spec.Configs[1].Name,
					Template:   comp.Spec.Configs[0].ConfigMap.Name,
					Namespace:  comp.Namespace,
					VolumeName: compDef.Spec.Configs[1].VolumeName,
				},
				Config:            true,
				ReconfigureAction: comp.Spec.Configs[0].ReconfigureAction,
			}))
		})

		It("restart does not disable default reconfigure", func() {
			compDef.Spec.Configs[1].Reconfigure = &appsv1.Action{
				Exec: &appsv1.ExecAction{
					Command: []string{"echo", "template", "reconfigure"},
				},
			}
			comp.Spec.Configs = append(comp.Spec.Configs, appsv1.ClusterComponentConfig{
				Name:    ptr.To(compDef.Spec.Configs[1].Name),
				Restart: ptr.To(true),
			})

			synthesizedComp, err := BuildSynthesizedComponent(ctx, cli, compDef, comp)
			Expect(err).Should(BeNil())

			Expect(synthesizedComp).ShouldNot(BeNil())
			Expect(synthesizedComp.FileTemplates).Should(ContainElement(SynthesizedFileTemplate{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name:                compDef.Spec.Configs[1].Name,
					Template:            compDef.Spec.Configs[1].Template,
					Namespace:           compDef.Spec.Configs[1].Namespace,
					VolumeName:          compDef.Spec.Configs[1].VolumeName,
					RestartOnFileChange: ptr.To(true),
					Reconfigure:         compDef.Spec.Configs[1].Reconfigure,
				},
				Config: true,
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
				Reconfigure: ptr.To(true),
				ReconfigureAction: &appsv1.Action{
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
				Config:              true,
				ReconfigureRequired: ptr.To(true),
				ReconfigureAction:   comp.Spec.Configs[0].ReconfigureAction,
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
				Config: true,
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
	})
})
