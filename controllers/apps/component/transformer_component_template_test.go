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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	configcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
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

	claimServerConfByParametersDefinition := func() {
		transCtx.CompDef.Spec.Configs = []appsv1.ComponentFileTemplate{
			{
				Name:            "serverConf",
				Template:        "server-template-cm",
				ExternalManaged: ptr.To(true),
			},
		}
		reader.Objects = append(reader.Objects, &parametersv1alpha1.ParametersDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "server-conf-params"},
			Spec: parametersv1alpha1.ParametersDefinitionSpec{
				ComponentDef:   compDefName,
				TemplateName:   "serverConf",
				FileName:       "server.conf",
				ServiceVersion: "*",
				FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
					Format: parametersv1alpha1.Properties,
				},
			},
			Status: parametersv1alpha1.ParametersDefinitionStatus{Phase: parametersv1alpha1.PDAvailablePhase},
		})
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

	Context("provision", func() {
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

		It("external managed - w/o template waits when parameters definition claims the config before component parameter exists", func() {
			transCtx.SynthesizeComponent.FileTemplates[1].Template = ""
			transCtx.SynthesizeComponent.FileTemplates[1].Namespace = ""
			transCtx.SynthesizeComponent.FileTemplates[1].ExternalManaged = ptr.To(true)
			claimServerConfByParametersDefinition()

			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(intctrlutil.IsRequeueError(err)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("waiting for parameters controller to backfill config template: serverConf"))
			Expect(err.Error()).ShouldNot(ContainSubstring("config/script template has no template specified"))
		})

		It("external managed - w/o template waits when component parameter claims the config", func() {
			transCtx.SynthesizeComponent.FileTemplates[1].Template = ""
			transCtx.SynthesizeComponent.FileTemplates[1].Namespace = ""
			transCtx.SynthesizeComponent.FileTemplates[1].ExternalManaged = ptr.To(true)
			reader.Objects = append(reader.Objects, &parametersv1alpha1.ComponentParameter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      configcore.GenerateComponentConfigurationName(clusterName, compName),
				},
				Spec: parametersv1alpha1.ComponentParameterSpec{
					ClusterName:   clusterName,
					ComponentName: compName,
					ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{
						{
							Name: "serverConf",
							ConfigSpec: &appsv1.ComponentFileTemplate{
								Name:            "serverConf",
								ExternalManaged: ptr.To(true),
							},
						},
					},
				},
			})

			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(intctrlutil.IsRequeueError(err)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("waiting for parameters controller to backfill config template: serverConf"))
			Expect(err.Error()).ShouldNot(ContainSubstring("config/script template has no template specified"))
		})

		It("external managed - w/o template still fails when parameters controller did not claim the config", func() {
			transCtx.SynthesizeComponent.FileTemplates[1].Template = ""
			transCtx.SynthesizeComponent.FileTemplates[1].Namespace = ""
			transCtx.SynthesizeComponent.FileTemplates[1].ExternalManaged = ptr.To(true)
			reader.Objects = append(reader.Objects, &parametersv1alpha1.ComponentParameter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      configcore.GenerateComponentConfigurationName(clusterName, compName),
				},
				Spec: parametersv1alpha1.ComponentParameterSpec{
					ClusterName:   clusterName,
					ComponentName: compName,
					ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{
						{
							Name: "otherConf",
							ConfigSpec: &appsv1.ComponentFileTemplate{
								Name:            "otherConf",
								ExternalManaged: ptr.To(true),
							},
						},
					},
				},
			})

			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(intctrlutil.IsRequeueError(err)).Should(BeFalse())
			Expect(err.Error()).Should(ContainSubstring("config/script template has no template specified"))
		})

		It("external managed - w/o template fails loudly when claimed parameter config failed", func() {
			transCtx.SynthesizeComponent.FileTemplates[1].Template = ""
			transCtx.SynthesizeComponent.FileTemplates[1].Namespace = ""
			transCtx.SynthesizeComponent.FileTemplates[1].ExternalManaged = ptr.To(true)
			message := "render failed"
			reader.Objects = append(reader.Objects, &parametersv1alpha1.ComponentParameter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      configcore.GenerateComponentConfigurationName(clusterName, compName),
				},
				Spec: parametersv1alpha1.ComponentParameterSpec{
					ClusterName:   clusterName,
					ComponentName: compName,
					ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{
						{
							Name: "serverConf",
							ConfigSpec: &appsv1.ComponentFileTemplate{
								Name:            "serverConf",
								ExternalManaged: ptr.To(true),
							},
						},
					},
				},
				Status: parametersv1alpha1.ComponentParameterStatus{
					ConfigurationItemStatus: []parametersv1alpha1.ConfigTemplateItemDetailStatus{
						{
							Name:    "serverConf",
							Phase:   parametersv1alpha1.CMergeFailedPhase,
							Message: &message,
						},
					},
				},
			})

			transformer := &componentFileTemplateTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).ShouldNot(BeNil())
			Expect(intctrlutil.IsRequeueError(err)).Should(BeFalse())
			Expect(err.Error()).Should(ContainSubstring("parameters controller failed to backfill config template serverConf: render failed"))
		})
	})
})
