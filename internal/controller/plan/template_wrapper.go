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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type templateRenderValidator = func(map[string]string) error

type renderWrapper struct {
	templateBuilder *configTemplateBuilder

	volumes        map[string]appsv1alpha1.ComponentTemplateSpec
	templateLabels map[string]string
	renderedObjs   []client.Object

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

		templateBuilder: cfgTplBuilder,
		templateLabels:  make(map[string]string),
		volumes:         make(map[string]appsv1alpha1.ComponentTemplateSpec),
	}
}

func (task *renderWrapper) renderConfigTemplate(configTemplates []appsv1alpha1.ComponentConfigSpec, obj client.Object) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	for _, tpl := range configTemplates {
		cmName := cfgcore.GetInstanceCMName(obj, &tpl.ComponentTemplateSpec)

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(task.templateBuilder, cmName, tpl.ConfigConstraintRef, tpl.ComponentTemplateSpec, task.params, task.ctx, task.cli, func(m map[string]string) error {
			return validateConfigMap(m, tpl, task.ctx, task.cli)
		})
		if err != nil {
			return err
		}
		updateCMConfigSelectorLabels(cm, tpl)

		if err := task.addRenderObject(tpl.ComponentTemplateSpec, cm, scheme); err != nil {
			return err
		}
	}
	return nil
}

func (task *renderWrapper) renderScriptTemplate(scriptTemplates []appsv1alpha1.ComponentTemplateSpec, obj client.Object) error {
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	for _, tpl := range scriptTemplates {
		cmName := cfgcore.GetInstanceCMName(obj, &tpl)

		// Generate ConfigMap objects for config files
		cm, err := generateConfigMapFromTpl(task.templateBuilder, cmName, "", tpl, task.params, task.ctx, task.cli, nil)
		if err != nil {
			return err
		}
		if err := task.addRenderObject(tpl, cm, scheme); err != nil {
			return err
		}
	}
	return nil
}

func (task *renderWrapper) addRenderObject(tpl appsv1alpha1.ComponentTemplateSpec, cm *corev1.ConfigMap, scheme *runtime.Scheme) error {
	// The owner of the configmap object is a cluster of users,
	// in order to manage the life cycle of configmap
	if err := controllerutil.SetOwnerReference(task.cluster, cm, scheme); err != nil {
		return err
	}

	cmName := cm.Name
	task.volumes[cmName] = tpl
	task.renderedObjs = append(task.renderedObjs, cm)

	// Configuration.kubeblocks.io/cfg-tpl-${ctpl-name}: ${cm-instance-name}
	task.templateLabels[cfgcore.GenerateTPLUniqLabelKeyWithConfig(tpl.Name)] = cmName
	return nil
}

func updateCMConfigSelectorLabels(cm *corev1.ConfigMap, tpl appsv1alpha1.ComponentConfigSpec) {
	if len(tpl.Keys) == 0 {
		return
	}
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	cm.Labels[cfgcore.CMConfigurationCMKeysLabelKey] = strings.Join(tpl.Keys, ",")
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
	configs, err := renderConfigMap(tplBuilder, tplCfg, ctx, cli)
	if err != nil {
		return nil, err
	}

	if dataValidator == nil {
		if err = dataValidator(configs); err != nil {
			return nil, err
		}
	}

	// Using ConfigMap cue template render to configmap of config
	return builder.BuildConfigMapWithTemplate(configs, params, cmName, configConstraintName, tplCfg)
}

// renderConfigMap render config file using template engine
func renderConfigMap(
	tplBuilder *configTemplateBuilder,
	tplCfg appsv1alpha1.ComponentTemplateSpec,
	ctx context.Context,
	cli client.Client) (map[string]string, error) {
	cmObj := &corev1.ConfigMap{}
	//  Require template configmap exist
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: tplCfg.Namespace,
		Name:      tplCfg.ConfigTemplateRef,
	}, cmObj); err != nil {
		return nil, err
	}

	if len(cmObj.Data) == 0 {
		return map[string]string{}, nil
	}

	tplBuilder.setTplName(tplCfg.ConfigTemplateRef)
	renderedCfg, err := tplBuilder.render(cmObj.Data)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to render configmap")
	}
	return renderedCfg, nil
}

// validateConfigMap validate config file against constraint
func validateConfigMap(
	renderedCfg map[string]string,
	tplCfg appsv1alpha1.ComponentConfigSpec,
	ctx context.Context,
	cli client.Client) error {
	cfgTemplate := &appsv1alpha1.ConfigConstraint{}
	if len(tplCfg.ConfigConstraintRef) > 0 {
		if err := cli.Get(ctx, client.ObjectKey{
			Namespace: "",
			Name:      tplCfg.ConfigConstraintRef,
		}, cfgTemplate); err != nil {
			return cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", tplCfg)
		}
	}

	// NOTE: not require checker configuration template status
	cfgChecker := cfgcore.NewConfigValidator(&cfgTemplate.Spec, cfgcore.WithKeySelector(tplCfg.Keys))

	// NOTE: It is necessary to verify the correctness of the data
	if err := cfgChecker.Validate(renderedCfg); err != nil {
		return cfgcore.WrapError(err, "failed to validate configmap")
	}

	return nil
}
