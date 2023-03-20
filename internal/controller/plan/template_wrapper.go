/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plan

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

type templateRenderValidator = func(map[string]string) error

type renderWrapper struct {
	templateBuilder *configTemplateBuilder

	volumes             map[string]appsv1alpha1.ComponentTemplateSpec
	templateAnnotations map[string]string
	renderedObjs        []client.Object

	ctx     context.Context
	cli     client.Client
	cluster *appsv1alpha1.Cluster
	params  builder.BuilderParams
}

func newTemplateRenderWrapper(templateBuilder *configTemplateBuilder, cluster *appsv1alpha1.Cluster, params builder.BuilderParams, ctx context.Context, cli client.Client) renderWrapper {
	return renderWrapper{
		ctx:     ctx,
		cli:     cli,
		cluster: cluster,
		params:  params,

		templateBuilder:     templateBuilder,
		templateAnnotations: make(map[string]string),
		volumes:             make(map[string]appsv1alpha1.ComponentTemplateSpec),
	}
}

func (wrapper *renderWrapper) renderConfigTemplate(task *intctrltypes.ReconcileTask) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	for _, configSpec := range task.Component.ConfigTemplates {
		cmName := cfgcore.GetComponentCfgName(task.Cluster.Name, task.Component.Name, configSpec.VolumeName)

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(wrapper.templateBuilder, cmName, configSpec.ConfigConstraintRef, configSpec.ComponentTemplateSpec, wrapper.params, wrapper.ctx, wrapper.cli, func(m map[string]string) error {
			return validateRenderedData(m, configSpec, wrapper.ctx, wrapper.cli)
		})
		if err != nil {
			return err
		}
		updateCMConfigSpecLabels(cm, configSpec)

		if err := wrapper.addRenderObject(configSpec.ComponentTemplateSpec, cm, scheme); err != nil {
			return err
		}
	}
	return nil
}

func (wrapper *renderWrapper) renderScriptTemplate(task *intctrltypes.ReconcileTask) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	for _, templateSpec := range task.Component.ScriptTemplates {
		cmName := cfgcore.GetComponentCfgName(task.Cluster.Name, task.Component.Name, templateSpec.VolumeName)

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(wrapper.templateBuilder, cmName, "", templateSpec, wrapper.params, wrapper.ctx, wrapper.cli, nil)
		if err != nil {
			return err
		}
		if err := wrapper.addRenderObject(templateSpec, cm, scheme); err != nil {
			return err
		}
	}
	return nil
}

func (wrapper *renderWrapper) addRenderObject(tpl appsv1alpha1.ComponentTemplateSpec, cm *corev1.ConfigMap, scheme *runtime.Scheme) error {
	// The owner of the configmap object is a cluster of users,
	// in order to manage the life cycle of configmap
	if err := controllerutil.SetOwnerReference(wrapper.cluster, cm, scheme); err != nil {
		return err
	}

	cmName := cm.Name
	wrapper.volumes[cmName] = tpl
	wrapper.renderedObjs = append(wrapper.renderedObjs, cm)
	wrapper.templateAnnotations[cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)] = cmName
	return nil
}

func updateCMConfigSpecLabels(cm *corev1.ConfigMap, configSpec appsv1alpha1.ComponentConfigSpec) {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}

	cm.Labels[constant.CMConfigurationSpecProviderLabelKey] = configSpec.Name
	cm.Labels[constant.CMConfigurationTemplateNameLabelKey] = configSpec.TemplateRef
	if configSpec.ConfigConstraintRef != "" {
		cm.Labels[constant.CMConfigurationConstraintsNameLabelKey] = configSpec.ConfigConstraintRef
	}

	if len(configSpec.Keys) != 0 {
		cm.Labels[constant.CMConfigurationCMKeysLabelKey] = strings.Join(configSpec.Keys, ",")
	}
}

// generateConfigMapFromTpl render config file by config template provided by provider.
func generateConfigMapFromTpl(tplBuilder *configTemplateBuilder,
	cmName string,
	configConstraintName string,
	templateSpec appsv1alpha1.ComponentTemplateSpec,
	params builder.BuilderParams,
	ctx context.Context,
	cli client.Client, dataValidator templateRenderValidator) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := renderConfigMapTemplate(tplBuilder, templateSpec, ctx, cli)
	if err != nil {
		return nil, err
	}

	if dataValidator != nil {
		if err = dataValidator(configs); err != nil {
			return nil, err
		}
	}

	// Using ConfigMap cue template render to configmap of config
	return builder.BuildConfigMapWithTemplate(configs, params, cmName, configConstraintName, templateSpec)
}

// renderConfigMapTemplate render config file using template engine
func renderConfigMapTemplate(
	templateBuilder *configTemplateBuilder,
	templateSpec appsv1alpha1.ComponentTemplateSpec,
	ctx context.Context,
	cli client.Client) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: templateSpec.Namespace,
		Name:      templateSpec.TemplateRef,
	}, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	templateBuilder.setTemplateName(templateSpec.TemplateRef)
	renderedData, err := templateBuilder.render(cmObj.Data)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to render configmap")
	}
	return renderedData, nil
}

// validateRenderedData validate config file against constraint
func validateRenderedData(
	renderedData map[string]string,
	configSpec appsv1alpha1.ComponentConfigSpec,
	ctx context.Context,
	cli client.Client) error {
	configConstraint := &appsv1alpha1.ConfigConstraint{}
	if configSpec.ConfigConstraintRef == "" {
		return nil
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: "",
		Name:      configSpec.ConfigConstraintRef,
	}, configConstraint); err != nil {
		return cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", configSpec)
	}

	// NOTE: not require checker configuration template status
	configChecker := cfgcore.NewConfigValidator(&configConstraint.Spec, cfgcore.WithKeySelector(configSpec.Keys))

	// NOTE: It is necessary to verify the correctness of the data
	if err := configChecker.Validate(renderedData); err != nil {
		return cfgcore.WrapError(err, "failed to validate configmap")
	}

	return nil
}
