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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type templateMerger interface {

	// Merge merges the baseData with the data from the template.
	merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error)

	// renderTemplate renders the template and returns the data.
	renderTemplate() (map[string]string, error)
}

type mergeContext struct {
	template   appsv1alpha1.ImportConfigTemplate
	configSpec appsv1alpha1.ComponentConfigSpec
	ccSpec     *appsv1alpha1.ConfigConstraintSpec

	builder *configTemplateBuilder
	ctx     context.Context
	client  client.Client
}

func (m *mergeContext) renderTemplate() (map[string]string, error) {
	templateSpec := appsv1alpha1.ComponentTemplateSpec{
		Name:        m.template.Name,
		Namespace:   m.template.Namespace,
		TemplateRef: m.template.TemplateRef,
	}
	configs, err := renderConfigMapTemplate(m.builder, templateSpec, m.ctx, m.client)
	if err != nil {
		return nil, err
	}
	if err := validateRawData(configs, m.configSpec, m.ccSpec); err != nil {
		return nil, err
	}
	return configs, nil
}

type configPatcher struct {
	*mergeContext
}

type configReplaceMerger struct {
	*mergeContext
}

type configOnlyAddMerger struct {
	*mergeContext
}

func (c *configPatcher) merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	formatter := c.ccSpec.FormatterConfig
	configPatch, err := cfgcore.TransformConfigPatchFromData(updatedData, formatter.Format, c.configSpec.Keys)
	if err != nil {
		return nil, err
	}
	if !configPatch.IsModify {
		return baseData, nil
	}

	r := make(map[string]string)
	params := cfgcore.GenerateVisualizedParamsList(configPatch, formatter, nil)
	for key, patch := range splitParameters(params) {
		v, ok := baseData[key]
		if !ok {
			r[key] = updatedData[key]
			continue
		}
		newConfig, err := cfgcore.ApplyConfigPatch([]byte(v), patch, formatter)
		if err != nil {
			return nil, err
		}
		r[key] = newConfig
	}
	return r, err
}

func (c *configReplaceMerger) merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	return cfgcore.MergeUpdatedConfig(baseData, updatedData), nil
}

func (c *configOnlyAddMerger) merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	return nil, cfgcore.MakeError("not implemented")
}

func newTemplateMerger(template appsv1alpha1.ImportConfigTemplate, ctx context.Context, cli client.Client, builder *configTemplateBuilder, configSpec appsv1alpha1.ComponentConfigSpec) (templateMerger, error) {
	if configSpec.ConfigConstraintRef == "" {
		return nil, cfgcore.MakeError("ConfigConstraintRef require not empty, configSpec[%v]", configSpec.Name)
	}
	ccObj := &appsv1alpha1.ConfigConstraint{}
	ccKey := client.ObjectKey{
		Namespace: "",
		Name:      configSpec.ConfigConstraintRef,
	}
	if err := cli.Get(ctx, ccKey, ccObj); err != nil {
		return nil, cfgcore.WrapError(err, "failed to get ConfigConstraint, key[%v]", configSpec)
	}
	if ccObj.Spec.FormatterConfig == nil {
		return nil, cfgcore.MakeError("importedConfigTemplate require ConfigConstraint.Spec.FormatterConfig, configSpec[%v]", configSpec)
	}

	templateData := &mergeContext{
		configSpec: configSpec,
		template:   template,
		ctx:        ctx,
		client:     cli,
		builder:    builder,
		ccSpec:     &ccObj.Spec,
	}

	var merger templateMerger
	switch template.Policy {
	default:
		return nil, cfgcore.MakeError("unknown template policy: %s", template.Policy)
	case appsv1alpha1.PatchPolicy:
		merger = &configPatcher{templateData}
	case appsv1alpha1.OnlyAddPolicy:
		merger = &configOnlyAddMerger{templateData}
	case appsv1alpha1.ReplacePolicy:
		merger = &configReplaceMerger{templateData}
	}
	return merger, nil
}

func mergerConfigTemplate(template *appsv1alpha1.ImportConfigTemplate,
	builder *configTemplateBuilder,
	configSpec appsv1alpha1.ComponentConfigSpec,
	cmObj *corev1.ConfigMap,
	ctx context.Context, cli client.Client) error {
	templateMerger, err := newTemplateMerger(*template, ctx, cli, builder, configSpec)
	if err != nil {
		return err
	}
	data, err := templateMerger.renderTemplate()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	mergedData, err := templateMerger.merge(cmObj.Data, data)
	if err != nil {
		return err
	}
	cmObj.Data = mergedData
	templateKey := client.ObjectKey{
		Namespace: template.Namespace,
		Name:      template.TemplateRef,
	}
	if cmObj.ObjectMeta.Annotations == nil {
		cmObj.ObjectMeta.Annotations = make(map[string]string)
	}
	cmObj.ObjectMeta.Annotations[constant.CMImportedConfigTemplateLabelKey] = templateKey.String()
	return nil
}

func splitParameters(params []cfgcore.VisualizedParam) map[string]map[string]string {
	r := make(map[string]map[string]string)
	for _, param := range params {
		if _, ok := r[param.Key]; !ok {
			r[param.Key] = make(map[string]string)
		}
		for _, kv := range param.Parameters {
			r[param.Key][kv.Key] = kv.Value
		}
	}
	return r
}
