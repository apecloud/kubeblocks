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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("file templates transformer test", func() {
	const (
		compDefName = "test-compdef"
		clusterName = "test-cluster"
		compName    = "comp"
	)

	var (
		reader                  *appsutil.MockReader
		dag                     *graph.DAG
		transCtx                *componentTransformContext
		logConfCM, serverConfCM *corev1.ConfigMap

		newDAG = func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
			d := graph.NewDAG()
			graphCli.Root(d, comp, comp, model.ActionStatusPtr())
			return d
		}
	)

	BeforeEach(func() {
		compDef := &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: compDefName,
			},
			Spec: appsv1.ComponentDefinitionSpec{},
		}
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "current-component",
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: appsv1.ComponentSpec{},
		}
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      fmt.Sprintf("%s-%s", clusterName, compName),
			},
		}

		logConfCM = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "logConf",
			},
			Data: map[string]string{},
		}
		serverConfCM = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "serverConf",
			},
			Data: map[string]string{},
		}

		reader = &appsutil.MockReader{
			Objects: []client.Object{its, logConfCM, serverConfCM},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)

		transCtx = &componentTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			CompDef:       compDef,
			Component:     comp,
			ComponentOrig: comp.DeepCopy(),
			SynthesizeComponent: &component.SynthesizedComponent{
				Namespace:    testCtx.DefaultNamespace,
				ClusterName:  clusterName,
				Name:         compName,
				FullCompName: fmt.Sprintf("%s-%s", clusterName, compName),
				Annotations: map[string]string{
					constant.KBAppMultiClusterPlacementKey: "member-1",
				},
				EnableInstanceAPI: ptr.To(true),
				PodSpec: &corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
						},
					},
				},
				FileTemplates: []component.SynthesizedFileTemplate{
					{
						ComponentFileTemplate: appsv1.ComponentFileTemplate{
							Name:       "logConf",
							Template:   logConfCM.Name,
							Namespace:  logConfCM.Namespace,
							VolumeName: "logConf",
						},
					},
					{
						ComponentFileTemplate: appsv1.ComponentFileTemplate{
							Name:       "serverConf",
							Template:   serverConfCM.Name,
							Namespace:  serverConfCM.Namespace,
							VolumeName: "serverConf",
						},
					},
				},
			},
		}
	})

	checkConfigTemplateExtension := func(tplName string) bool {
		for _, config := range transCtx.SynthesizeComponent.FileTemplates {
			if config.Name == tplName && isExternalManaged(config) {
				return true
			}
		}
		return false
	}

	checkTemplateObjects := func(tpls []string) {
		graphCli := transCtx.Client.(model.GraphClient)
		objs := graphCli.FindAll(dag, &corev1.ConfigMap{})

		mobjs := make(map[string]client.Object)
		for i, obj := range objs {
			mobjs[obj.GetName()] = objs[i]
		}

		for _, tpl := range tpls {
			objName := fileTemplateObjectName(transCtx.SynthesizeComponent, tpl)
			if !checkConfigTemplateExtension(tpl) {
				Expect(mobjs).Should(HaveKey(objName))
			}
		}
	}

	checkTemplateObject := func(tplName string, f func(configMap *corev1.ConfigMap)) {
		graphCli := transCtx.Client.(model.GraphClient)
		objs := graphCli.FindAll(dag, &corev1.ConfigMap{})

		mobjs := make(map[string]client.Object)
		for i, obj := range objs {
			mobjs[obj.GetName()] = objs[i]
		}

		objName := fileTemplateObjectName(transCtx.SynthesizeComponent, tplName)
		Expect(mobjs).Should(HaveKey(objName))
		if f != nil {
			f(mobjs[objName].(*corev1.ConfigMap))
		}
	}

	newVolume := func(tplName string) corev1.Volume {
		return corev1.Volume{
			Name: tplName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fileTemplateObjectName(transCtx.SynthesizeComponent, tplName),
					},
					DefaultMode: ptr.To[int32](0444),
				},
			},
		}
	}

	checkVolumes := func(tpls []string) {
		podSpec := transCtx.SynthesizeComponent.PodSpec
		for _, tpl := range tpls {
			if !checkConfigTemplateExtension(tpl) {
				Expect(podSpec.Volumes).Should(ContainElement(newVolume(tpl)))
			}
		}
	}

	checkAssistantObject := func(kind, namespace, name string) {
		Expect(transCtx.SynthesizeComponent.InstanceAssistantObjects).Should(ContainElement(corev1.ObjectReference{
			Kind:      kind,
			Namespace: namespace,
			Name:      name,
		}))
	}

	// checkEnvWithAction := func(action string) {
	//	podSpec := transCtx.SynthesizeComponent.PodSpec
	//	for _, c := range podSpec.Containers {
	//		found := false
	//		for _, e := range c.Env {
	//			if strings.Contains(e.Value, action) {
	//				found = true
	//				break
	//			}
	//		}
	//		Expect(found).Should(BeTrue())
	//	}
	// }

	instanceConfigMaps := func() []*corev1.ConfigMap {
		var configs []*corev1.ConfigMap
		for _, obj := range transCtx.Client.(model.GraphClient).FindAll(dag, &corev1.ConfigMap{}) {
			cm := obj.(*corev1.ConfigMap)
			if cm.Labels[instanceConfigMapLabelKey] == "true" {
				configs = append(configs, cm)
			}
		}
		return configs
	}

	Context("provision", func() {
		It("uses the same resolved variable for the Component file and environment", func() {
			transCtx.CompDef.Spec.Vars = []appsv1.EnvVar{{Name: "TOKEN", Expression: ptr.To("{{ randAlphaNum 64 }}")}}
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			logConfCM.Data["server.conf"] = "{{ .TOKEN }}"

			Expect((&componentVarsTransformer{}).Transform(transCtx, dag)).To(Succeed())
			Expect((&componentFileTemplateTransformer{}).Transform(transCtx, dag)).To(Succeed())

			graphCli := transCtx.Client.(model.GraphClient)
			envMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Namespace: transCtx.Component.Namespace,
				Name:      constant.GenerateClusterComponentEnvPattern(clusterName, compName),
			}}
			vertex := graphCli.FindMatchedVertex(dag, envMap)
			Expect(vertex).NotTo(BeNil())
			envValue := vertex.(*model.ObjectVertex).Obj.(*corev1.ConfigMap).Data["TOKEN"]
			Expect(envValue).To(HaveLen(64))
			checkTemplateObject("logConf", func(obj *corev1.ConfigMap) {
				Expect(obj.Data["server.conf"]).To(Equal(envValue))
			})
		})

		It("reuses the Component render for an InstanceTemplate with no overrides", func() {
			transCtx.CompDef.Spec.Vars = []appsv1.EnvVar{{Name: "TOKEN", Expression: ptr.To("{{ randAlphaNum 64 }}")}}
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			logConfCM.Data["server.conf"] = "{{ .TOKEN }}/{{ randAlphaNum 64 }}"
			transCtx.SynthesizeComponent.Instances = []appsv1.InstanceTemplate{
				{Name: "same"},
				{Name: "empty-resources", Resources: &corev1.ResourceRequirements{}},
			}

			Expect((&componentVarsTransformer{}).Transform(transCtx, dag)).To(Succeed())
			Expect((&componentFileTemplateTransformer{}).Transform(transCtx, dag)).To(Succeed())

			checkTemplateObject("logConf", func(obj *corev1.ConfigMap) {
				Expect(obj.Data["server.conf"]).To(HaveLen(129))
			})
			Expect(instanceConfigMaps()).To(BeEmpty())
			Expect(transCtx.SynthesizeComponent.InstanceConfigs["same"]).To(BeEmpty())
			Expect(transCtx.SynthesizeComponent.InstanceConfigs["empty-resources"]).To(BeEmpty())
		})

		It("renders named templates with effective resource and service version variables", func() {
			required := ptr.To(appsv1.VarRequired)
			transCtx.CompDef.Spec.Vars = []appsv1.EnvVar{
				{Name: "MEMORY", ValueFrom: &appsv1.VarSource{ResourceVarRef: &appsv1.ResourceVarSelector{ResourceVars: appsv1.ResourceVars{Memory: required}}}},
				{Name: "VERSION", ValueFrom: &appsv1.VarSource{ComponentVarRef: &appsv1.ComponentVarSelector{ComponentVars: appsv1.ComponentVars{ServiceVersion: required}}}},
			}
			transCtx.SynthesizeComponent.SidecarVars = []appsv1.EnvVar{{Name: "SIDECAR", Value: "sidecar"}}
			transCtx.Component.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")}
			transCtx.Component.Spec.ServiceVersion = "1.0"
			reader.Objects = append(reader.Objects, transCtx.Component)
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			logConfCM.Data["server.conf"] = "{{.MEMORY}}/{{.VERSION}}/{{.SIDECAR}}"
			transCtx.SynthesizeComponent.Instances = []appsv1.InstanceTemplate{{
				Name: "large", ServiceVersion: "2.0",
				Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")}},
			}}

			Expect((&componentVarsTransformer{}).Transform(transCtx, dag)).To(Succeed())
			Expect((&componentFileTemplateTransformer{}).Transform(transCtx, dag)).To(Succeed())
			checkTemplateObject("logConf", func(obj *corev1.ConfigMap) {
				Expect(obj.Data["server.conf"]).To(Equal("2147483648/1.0/sidecar"))
			})
			Expect(instanceConfigMaps()).To(HaveLen(1))
			obj := instanceConfigMaps()[0]
			Expect(obj.Data["server.conf"]).To(Equal("8589934592/2.0/sidecar"))
			Expect(obj.Labels[instanceConfigMapLabelKey]).To(Equal("true"))
			Expect(transCtx.SynthesizeComponent.InstanceConfigs["large"][0].Volume).To(Equal(&workloads.ConfigVolumeBinding{VolumeName: "logConf", ConfigMapName: obj.Name}))
			Expect(transCtx.SynthesizeComponent.Configs[0].Volume).To(Equal(&workloads.ConfigVolumeBinding{VolumeName: "logConf", ConfigMapName: fileTemplateObjectName(transCtx.SynthesizeComponent, "logConf")}))
			Expect(transCtx.SynthesizeComponent.TemplateVars["MEMORY"]).To(Equal("2147483648"))
		})

		It("publishes rendered configs independently of Instance assistants in a provider", func() {
			transCtx.Component.UID = "component-uid"
			transCtx.Component.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "a,missing"}
			transCtx.SynthesizeComponent.EnableInstanceAPI = ptr.To(true)
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true

			Expect((&componentFileTemplateTransformer{configProviders: []string{"a"}}).Transform(transCtx, dag)).To(Succeed())
			checkTemplateObject("logConf", func(obj *corev1.ConfigMap) {
				Expect(obj.Labels[constant.KBAppConfigMapOwnerLabelKey]).To(Equal(string(transCtx.Component.UID)))
				Expect(obj.Annotations[instanceConfigMapPlacementsAnnotationKey]).To(Equal("a"))
				Expect(transCtx.SynthesizeComponent.Configs[0].Volume.ConfigMapName).To(Equal(obj.Name))
				Expect(transCtx.SynthesizeComponent.InstanceAssistantObjects).NotTo(ContainElement(corev1.ObjectReference{
					Kind: "ConfigMap", Namespace: obj.Namespace, Name: obj.Name,
				}))
			})
			checkAssistantObject("ConfigMap", transCtx.Component.Namespace, fileTemplateObjectName(transCtx.SynthesizeComponent, "serverConf"))
		})

		It("reuses the Component render when merged file variables match", func() {
			transCtx.CompDef.Spec.Vars = []appsv1.EnvVar{{
				Name: "MEMORY", ValueFrom: &appsv1.VarSource{ResourceVarRef: &appsv1.ResourceVarSelector{
					ResourceVars: appsv1.ResourceVars{Memory: ptr.To(appsv1.VarRequired)},
				}},
			}}
			transCtx.Component.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")}
			reader.Objects = append(reader.Objects, transCtx.Component)
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			logConfCM.Data["server.conf"] = "{{ .MEMORY }}/{{ randAlphaNum 64 }}"
			transCtx.SynthesizeComponent.Instances = []appsv1.InstanceTemplate{{
				Name:      "large",
				Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("8Gi")}},
				Configs: []appsv1.ClusterComponentConfig{{
					Name: ptr.To("logConf"), Variables: map[string]string{"MEMORY": "2147483648"},
				}},
			}}

			Expect((&componentVarsTransformer{}).Transform(transCtx, dag)).To(Succeed())
			Expect((&componentFileTemplateTransformer{}).Transform(transCtx, dag)).To(Succeed())

			Expect(instanceConfigMaps()).To(BeEmpty())
			config := transCtx.SynthesizeComponent.InstanceConfigs["large"][0]
			Expect(config.Parameters).To(HaveKeyWithValue("MEMORY", "2147483648"))
			Expect(config.Volume.ConfigMapName).To(Equal(fileTemplateObjectName(transCtx.SynthesizeComponent, "logConf")))
		})

		It("inherits variables and accepts a named template source override", func() {
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			transCtx.SynthesizeComponent.FileTemplates[0].Variables = map[string]string{"PORT": "1000", "MODE": "normal"}
			logConfCM.Data["server.conf"] = "{{.PORT}}/{{.MODE}}"
			alternate := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: transCtx.Component.Namespace, Name: "alternate"}, Data: map[string]string{"server.conf": "alternate/{{.PORT}}/{{.MODE}}"}}
			reader.Objects = append(reader.Objects, alternate)
			transCtx.SynthesizeComponent.Instances = []appsv1.InstanceTemplate{{Name: "large", Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("logConf"), ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: alternate.Name}}}, Variables: map[string]string{"PORT": "2000"}}}}}

			Expect((&componentFileTemplateTransformer{}).Transform(transCtx, dag)).To(Succeed())
			Expect(instanceConfigMaps()).To(HaveLen(1))
			Expect(instanceConfigMaps()[0].Data["server.conf"]).To(Equal("alternate/2000/normal"))
			Expect(transCtx.SynthesizeComponent.InstanceConfigs["large"][0].Parameters).To(HaveKeyWithValue("MODE", "normal"))
		})

		It("shares identical rendered content while preserving different action parameters", func() {
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			transCtx.SynthesizeComponent.FileTemplates[0].Variables = map[string]string{"PORT": "1000"}
			logConfCM.Data["server.conf"] = "constant"
			transCtx.SynthesizeComponent.Instances = []appsv1.InstanceTemplate{
				{Name: "same"},
				{Name: "different", Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("logConf"), Variables: map[string]string{"PORT": "2000"}}}},
				{Name: "disabled", Replicas: ptr.To(int32(0)), Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("logConf"), Variables: map[string]string{"PORT": "3000"}}}},
			}

			Expect((&componentFileTemplateTransformer{}).Transform(transCtx, dag)).To(Succeed())
			Expect(instanceConfigMaps()).To(BeEmpty())
			Expect(transCtx.SynthesizeComponent.InstanceConfigs["same"]).To(BeEmpty())
			Expect(transCtx.SynthesizeComponent.InstanceConfigs["disabled"]).To(BeEmpty())
			config := transCtx.SynthesizeComponent.InstanceConfigs["different"][0]
			Expect(config.Parameters).To(HaveKeyWithValue("PORT", "2000"))
			Expect(config.Volume.ConfigMapName).To(Equal(fileTemplateObjectName(transCtx.SynthesizeComponent, "logConf")))
		})

		It("validates every rendered template before scheduling ConfigMap changes", func() {
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			logConfCM.Data["server.conf"] = "valid"
			invalid := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: transCtx.Component.Namespace, Name: "invalid"}, Data: map[string]string{"server.conf": "{{.MISSING}}"}}
			reader.Objects = append(reader.Objects, invalid)
			transCtx.SynthesizeComponent.Instances = []appsv1.InstanceTemplate{
				{Name: "first", Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("logConf")}}},
				{Name: "second", Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("logConf"), ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: invalid.Name}}}}}},
			}

			Expect((&componentFileTemplateTransformer{}).Transform(transCtx, dag)).NotTo(Succeed())
			Expect(transCtx.Client.(model.GraphClient).FindAll(dag, &corev1.ConfigMap{})).To(BeEmpty())
		})

		It("replays file changes after the ConfigMap update has persisted", func() {
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			transCtx.SynthesizeComponent.FileTemplates[0].Reconfigure = &appsv1.Action{Exec: &appsv1.ExecAction{Command: []string{"reload"}}}
			transCtx.SynthesizeComponent.FileTemplates[0].Variables = map[string]string{"PORT": "1000"}
			transCtx.SynthesizeComponent.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "logConf", MountPath: "/etc/server"}}
			logConfCM.Data["server.conf"] = "port={{.PORT}}"
			transCtx.SynthesizeComponent.Instances = []appsv1.InstanceTemplate{{Name: "large", Configs: []appsv1.ClusterComponentConfig{{Name: ptr.To("logConf"), Variables: map[string]string{"PORT": "2000"}}}}}
			transformer := &componentFileTemplateTransformer{}
			persist := func() {
				for _, obj := range transCtx.Client.(model.GraphClient).FindAll(dag, &corev1.ConfigMap{}) {
					found := false
					for i, existing := range reader.Objects {
						if client.ObjectKeyFromObject(obj) == client.ObjectKeyFromObject(existing) {
							reader.Objects[i] = obj.DeepCopyObject().(client.Object)
							found = true
							break
						}
					}
					if !found {
						reader.Objects = append(reader.Objects, obj.DeepCopyObject().(client.Object))
					}
				}
				dag = newDAG(transCtx.Client.(model.GraphClient), transCtx.Component)
				transCtx.SynthesizeComponent.Configs = nil
			}
			Expect(transformer.Transform(transCtx, dag)).To(Succeed())
			persist()
			transCtx.SynthesizeComponent.Instances[0].Configs[0].Variables["PORT"] = "3000"
			Expect(transformer.Transform(transCtx, dag)).To(Succeed())
			before := transCtx.SynthesizeComponent.InstanceConfigs["large"][0].DeepCopy()
			Expect(before.Parameters).To(HaveKeyWithValue("KB_CONFIG_FILES_UPDATED", ContainSubstring("/etc/server/server.conf:")))
			Expect(instanceConfigMaps()[0].Annotations).To(HaveKey(fileTemplateChangesAnnotationKey))
			persist()
			Expect(transformer.Transform(transCtx, dag)).To(Succeed())
			Expect(transCtx.SynthesizeComponent.InstanceConfigs["large"][0]).To(Equal(*before))
		})

		It("keeps large restart-only ConfigMaps writable without durable reload changes", func() {
			tpl := &transCtx.SynthesizeComponent.FileTemplates[0]
			tpl.Config = true
			tpl.RestartOnFileChange = ptr.To(true)
			transCtx.SynthesizeComponent.PodSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: tpl.VolumeName, MountPath: "/etc/" + strings.Repeat("directory/", 20)}}
			for i := 0; i < 2000; i++ {
				logConfCM.Data[fmt.Sprintf("file-%04d.conf", i)] = "old"
			}
			transformer := &componentFileTemplateTransformer{}
			running, err := transformer.buildFileTemplateObject(transCtx, *tpl)
			Expect(err).NotTo(HaveOccurred())
			reader.Objects = append(reader.Objects, running)
			for file := range logConfCM.Data {
				logConfCM.Data[file] = "new"
			}

			Expect(transformer.Transform(transCtx, dag)).To(Succeed())
			vertex := transCtx.Client.(model.GraphClient).FindMatchedVertex(dag, running).(*model.ObjectVertex)
			updated := vertex.Obj.(*corev1.ConfigMap)
			Expect(updated.Data).To(Equal(logConfCM.Data))
			Expect(updated.Annotations).NotTo(HaveKey(fileTemplateChangesAnnotationKey))
			Expect(apivalidation.ValidateAnnotationsSize(updated.Annotations)).To(Succeed())
			Expect(transCtx.SynthesizeComponent.Configs[0].Restart).To(Equal(ptr.To(true)))
			Expect(transCtx.SynthesizeComponent.Configs[0].ConfigHash).NotTo(BeNil())
		})

		It("keeps stable tuple names within the ConfigMap name limit", func() {
			synth := *transCtx.SynthesizeComponent
			synth.FullCompName = strings.Repeat("component", 30)
			first := instanceFileTemplateObjectName(&synth, "a-b", "c")
			second := instanceFileTemplateObjectName(&synth, "a", "b-c")
			Expect(first).To(HaveLen(247))
			Expect(first).NotTo(Equal(second))
			Expect(first).To(Equal(instanceFileTemplateObjectName(&synth, "a-b", "c")))
		})

		It("ok", func() {
			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			checkVolumes([]string{"logConf", "serverConf"})
			checkTemplateObjects([]string{"logConf", "serverConf"})
		})

		It("variables - w/o", func() {
			logConfCM.Data["level"] = "{{- if (index $ \"LOG_LEVEL\") }}\n\t{{- .LOG_LEVEL }}\n{{- else }}\n\t{{- \"info\" }}\n{{- end }}"

			transformer := &componentFileTemplateTransformer{}
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())

			checkVolumes([]string{"logConf", "serverConf"})
			checkTemplateObjects([]string{"logConf", "serverConf"})
			checkTemplateObject("logConf", func(obj *corev1.ConfigMap) {
				Expect(obj.Data).Should(HaveKeyWithValue("level", "info"))
			})

		})

		It("variables - w/", func() {
			logConfCM.Data["level"] = "{{- if (index $ \"LOG_LEVEL\") }}\n\t{{- .LOG_LEVEL }}\n{{- else }}\n\t{{- \"info\" }}\n{{- end }}"
			transCtx.SynthesizeComponent.FileTemplates[0].Variables = map[string]string{
				"LOG_LEVEL": "debug",
			}

			transformer := &componentFileTemplateTransformer{}
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())

			checkVolumes([]string{"logConf", "serverConf"})
			checkTemplateObjects([]string{"logConf", "serverConf"})
			checkTemplateObject("logConf", func(obj *corev1.ConfigMap) {
				Expect(obj.Data).Should(HaveKeyWithValue("level", "debug"))
			})
		})

		It("keeps empty template file content empty", func() {
			logConfCM.Data["member_leave.sh"] = "member_leave"
			logConfCM.Data["role_probe.sh"] = "role_probe"
			logConfCM.Data["switchover.sh"] = ""

			transformer := &componentFileTemplateTransformer{}
			Expect(transformer.Transform(transCtx, dag)).Should(BeNil())

			checkTemplateObject("logConf", func(obj *corev1.ConfigMap) {
				Expect(obj.Labels).Should(HaveKeyWithValue(kubeBlockFileTemplateLabelKey, "true"))
				Expect(obj.Data).Should(HaveKeyWithValue("member_leave.sh", "member_leave"))
				Expect(obj.Data).Should(HaveKeyWithValue("role_probe.sh", "role_probe"))
				Expect(obj.Data).Should(HaveKeyWithValue("switchover.sh", ""))
			})
		})

		It("udf reconfigure", func() {
			transCtx.SynthesizeComponent.FileTemplates[0].ReconfigureRequired = ptr.To(true)
			transCtx.SynthesizeComponent.FileTemplates[0].ReconfigureAction = &appsv1.Action{
				Exec: &appsv1.ExecAction{
					Command: []string{"echo", "reconfigure"},
				},
			}

			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			checkVolumes([]string{"logConf", "serverConf"})
			checkTemplateObjects([]string{"logConf", "serverConf"})
			// checkEnvWithAction(component.UDFReconfigureActionName(transCtx.SynthesizeComponent.FileTemplates[0]))
		})

		It("reconfigure args", func() {
			transCtx.SynthesizeComponent.FileTemplates[0].Config = true
			transCtx.SynthesizeComponent.FileTemplates[0].ReconfigureRequired = ptr.To(true)
			transCtx.SynthesizeComponent.FileTemplates[0].ReconfigureAction = &appsv1.Action{
				HTTP: &appsv1.HTTPAction{
					Port: "8080",
					Path: "/reload",
				},
			}
			transCtx.SynthesizeComponent.FileTemplates[0].ReconfigureArgs = [][]string{{"maxmemory", "1gb"}, {"timeout", "30"}}

			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			Expect(transCtx.SynthesizeComponent.Configs).Should(HaveLen(1))
			Expect(transCtx.SynthesizeComponent.Configs[0].Name).Should(Equal("logConf"))
			Expect(transCtx.SynthesizeComponent.Configs[0].ReconfigureActionName).Should(Equal(component.UserReconfigureActionName(transCtx.SynthesizeComponent.FileTemplates[0])))
			Expect(transCtx.SynthesizeComponent.Configs[0].Reconfigure).Should(Equal(transCtx.SynthesizeComponent.FileTemplates[0].ReconfigureAction))
			Expect(transCtx.SynthesizeComponent.Configs[0].ReconfigureArgs).Should(Equal(transCtx.SynthesizeComponent.FileTemplates[0].ReconfigureArgs))
		})

		It("external managed", func() {
			transCtx.SynthesizeComponent.FileTemplates[1].ReconfigureRequired = ptr.To(true)
			transCtx.SynthesizeComponent.FileTemplates[1].ReconfigureAction = &appsv1.Action{
				Exec: &appsv1.ExecAction{
					Command: []string{"echo", "reconfigure"},
				},
			}
			transCtx.SynthesizeComponent.FileTemplates[1].ExternalManaged = ptr.To(true)

			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			checkVolumes([]string{"logConf", "serverConf"})
			checkTemplateObjects([]string{"logConf", "serverConf"})
			checkAssistantObject("ConfigMap", serverConfCM.Namespace, serverConfCM.Name)
			// checkEnvWithAction(component.UDFReconfigureActionName(transCtx.SynthesizeComponent.FileTemplates[1]))
		})

		It("external managed - w/o template", func() {
			transCtx.SynthesizeComponent.FileTemplates[1].Template = ""
			transCtx.SynthesizeComponent.FileTemplates[1].Namespace = ""
			transCtx.SynthesizeComponent.FileTemplates[1].Reconfigure = nil
			transCtx.SynthesizeComponent.FileTemplates[1].ExternalManaged = ptr.To(true)

			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("config/script template has no template specified"))
		})
	})
})
