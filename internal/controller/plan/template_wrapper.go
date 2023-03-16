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

func newTemplateRenderWrapper(cfgTplBuilder *configTemplateBuilder, cluster *appsv1alpha1.Cluster, params builder.BuilderParams, ctx context.Context, cli client.Client) renderWrapper {
	return renderWrapper{
		ctx:     ctx,
		cli:     cli,
		cluster: cluster,
		params:  params,

		templateBuilder:     cfgTplBuilder,
		templateAnnotations: make(map[string]string),
		volumes:             make(map[string]appsv1alpha1.ComponentTemplateSpec),
	}
}

func (wrapper *renderWrapper) renderConfigTemplate(task *intctrltypes.ReconcileTask, obj client.Object) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	for _, tpl := range task.Component.ConfigTemplates {
		cmName := cfgcore.GetComponentCfgName(task.Cluster.Name, task.Component.Name, tpl.VolumeName)

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(wrapper.templateBuilder, cmName, tpl.ConfigConstraintRef, tpl.ComponentTemplateSpec, wrapper.params, wrapper.ctx, wrapper.cli, func(m map[string]string) error {
			return validateRenderedData(m, tpl, wrapper.ctx, wrapper.cli)
		})
		if err != nil {
			return err
		}
		updateCMConfigSpecLabels(cm, tpl)

		if err := wrapper.addRenderObject(tpl.ComponentTemplateSpec, cm, scheme); err != nil {
			return err
		}
	}
	return nil
}

func (wrapper *renderWrapper) renderScriptTemplate(task *intctrltypes.ReconcileTask, obj client.Object) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	for _, tpl := range task.Component.ScriptTemplates {
		cmName := cfgcore.GetComponentCfgName(task.Cluster.Name, task.Component.Name, tpl.VolumeName)

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(wrapper.templateBuilder, cmName, "", tpl, wrapper.params, wrapper.ctx, wrapper.cli, nil)
		if err != nil {
			return err
		}
		if err := wrapper.addRenderObject(tpl, cm, scheme); err != nil {
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

func updateCMConfigSpecLabels(cm *corev1.ConfigMap, tpl appsv1alpha1.ComponentConfigSpec) {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}

	cm.Labels[constant.CMConfigurationSpecProviderLabelKey] = tpl.Name
	cm.Labels[constant.CMConfigurationTplNameLabelKey] = tpl.TemplateRef
	if tpl.ConfigConstraintRef != "" {
		cm.Labels[constant.CMConfigurationConstraintsNameLabelKey] = tpl.ConfigConstraintRef
	}

	if len(tpl.Keys) != 0 {
		cm.Labels[constant.CMConfigurationCMKeysLabelKey] = strings.Join(tpl.Keys, ",")
	}
}

// generateConfigMapFromTpl render config file by config template provided by provider.
func generateConfigMapFromTpl(tplBuilder *configTemplateBuilder,
	cmName string,
	configConstraintName string,
	tplCfg appsv1alpha1.ComponentTemplateSpec,
	params builder.BuilderParams,
	ctx context.Context,
	cli client.Client, dataValidator templateRenderValidator) (*corev1.ConfigMap, error) {
	// Render config template by TplEngine
	// The template namespace must be the same as the ClusterDefinition namespace
	configs, err := renderConfigMapTemplate(tplBuilder, tplCfg, ctx, cli)
	if err != nil {
		return nil, err
	}

	if dataValidator != nil {
		if err = dataValidator(configs); err != nil {
			return nil, err
		}
	}

	// Using ConfigMap cue template render to configmap of config
	return builder.BuildConfigMapWithTemplate(configs, params, cmName, configConstraintName, tplCfg)
}

// renderConfigMapTemplate render config file using template engine
func renderConfigMapTemplate(
	tplBuilder *configTemplateBuilder,
	tplCfg appsv1alpha1.ComponentTemplateSpec,
	ctx context.Context,
	cli client.Client) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: tplCfg.Namespace,
		Name:      tplCfg.TemplateRef,
	}, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	tplBuilder.setTplName(tplCfg.TemplateRef)
	renderedCfg, err := tplBuilder.render(cmObj.Data)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to render configmap")
	}
	return renderedCfg, nil
}

// validateRenderedData validate config file against constraint
func validateRenderedData(
	renderedCfg map[string]string,
	tplCfg appsv1alpha1.ComponentConfigSpec,
	ctx context.Context,
	cli client.Client) error {
	cfgTemplate := &appsv1alpha1.ConfigConstraint{}
	if tplCfg.ConfigConstraintRef == "" {
		return nil
	}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: "",
		Name:      tplCfg.ConfigConstraintRef,
	}, cfgTemplate); err != nil {
		return cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", tplCfg)
	}

	// NOTE: not require checker configuration template status
	cfgChecker := cfgcore.NewConfigValidator(&cfgTemplate.Spec, cfgcore.WithKeySelector(tplCfg.Keys))

	// NOTE: It is necessary to verify the correctness of the data
	if err := cfgChecker.Validate(renderedCfg); err != nil {
		return cfgcore.WrapError(err, "failed to validate configmap")
	}

	return nil
}
