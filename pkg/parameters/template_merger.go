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

package parameters

import (
	"fmt"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

type TemplateMerger interface {

	// merge merges the baseData with the data from the template.
	merge(baseData map[string]string, updatedData map[string]string, manager *valueManager) (map[string]string, error)

	// renderTemplate renders the template and returns the data.
	renderTemplate() (map[string]string, error)
}

type mergeContext struct {
	render.TemplateRender

	template     parametersv1alpha1.ConfigTemplateExtension
	configSpec   appsv1.ComponentFileTemplate
	paramsDefs   []*parametersv1alpha1.ParametersDefinition
	configRender *parametersv1alpha1.ParamConfigRenderer
}

func (m *mergeContext) renderTemplate() (map[string]string, error) {
	templateSpec := appsv1.ComponentFileTemplate{
		// Name:        m.template.Name,
		Namespace: m.template.Namespace,
		Template:  m.template.TemplateRef,
	}
	configs, err := m.RenderConfigMapTemplate(templateSpec)
	if err != nil {
		return nil, err
	}
	if err := validateRenderedData(configs, m.paramsDefs, m.configRender); err != nil {
		return nil, err
	}
	return configs, nil
}

type noneOp struct {
	*mergeContext
}

func (n noneOp) merge(_ map[string]string, updatedData map[string]string, _ *valueManager) (map[string]string, error) {
	return updatedData, nil
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

func (c *configPatcher) merge(baseData map[string]string, updatedData map[string]string, manager *valueManager) (map[string]string, error) {
	if c.configRender == nil || len(c.configRender.Spec.Configs) == 0 {
		return nil, fmt.Errorf("not support patch merge policy")
	}
	configPatch, err := core.TransformConfigPatchFromData(updatedData, c.configRender.Spec)
	if err != nil {
		return nil, err
	}
	if !configPatch.IsModify {
		return baseData, nil
	}

	params := core.GenerateVisualizedParamsList(configPatch, c.configRender.Spec.Configs)
	mergedData := copyMap(baseData)
	for key, patch := range splitParameters(params) {
		v, ok := baseData[key]
		if !ok {
			mergedData[key] = updatedData[key]
			continue
		}
		newConfig, err := core.ApplyConfigPatch([]byte(v), patch, core.ResolveConfigFormat(c.configRender.Spec.Configs, key), manager.BuildValueTransformer(key))
		if err != nil {
			return nil, err
		}
		mergedData[key] = newConfig
	}

	for key, content := range updatedData {
		if _, ok := mergedData[key]; !ok {
			mergedData[key] = content
		}
	}
	return mergedData, err
}

func (c *configReplaceMerger) merge(baseData map[string]string, updatedData map[string]string, _ *valueManager) (map[string]string, error) {
	return core.MergeUpdatedConfig(baseData, updatedData), nil
}

func (c *configOnlyAddMerger) merge(baseData map[string]string, updatedData map[string]string, _ *valueManager) (map[string]string, error) {
	return nil, core.MakeError("not implemented")
}

func NewTemplateMerger(template parametersv1alpha1.ConfigTemplateExtension,
	templateRender render.TemplateRender,
	configSpec appsv1.ComponentFileTemplate,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	configRender *parametersv1alpha1.ParamConfigRenderer,
) (TemplateMerger, error) {
	templateData := &mergeContext{
		configSpec:     configSpec,
		template:       template,
		TemplateRender: templateRender,
		paramsDefs:     paramsDefs,
		configRender:   configRender,
	}

	var merger TemplateMerger
	switch template.Policy {
	default:
		return nil, core.MakeError("unknown template policy: %s", template.Policy)
	case parametersv1alpha1.NoneMergePolicy:
		merger = &noneOp{templateData}
	case parametersv1alpha1.PatchPolicy:
		merger = &configPatcher{templateData}
	case parametersv1alpha1.OnlyAddPolicy:
		merger = &configOnlyAddMerger{templateData}
	case parametersv1alpha1.ReplacePolicy:
		merger = &configReplaceMerger{templateData}
	}
	return merger, nil
}

func mergerConfigTemplate(template parametersv1alpha1.ConfigTemplateExtension,
	templateRender render.TemplateRender,
	configSpec appsv1.ComponentFileTemplate,
	baseData map[string]string,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	configRender *parametersv1alpha1.ParamConfigRenderer) (map[string]string, error) {
	templateMerger, err := NewTemplateMerger(template, templateRender, configSpec, paramsDefs, configRender)
	if err != nil {
		return nil, err
	}
	data, err := templateMerger.renderTemplate()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	return templateMerger.merge(baseData, data, NewValueManager(paramsDefs, configRender.Spec.Configs))
}

func splitParameters(params []core.VisualizedParam) map[string]map[string]*string {
	r := make(map[string]map[string]*string)
	for _, param := range params {
		if _, ok := r[param.Key]; !ok {
			r[param.Key] = make(map[string]*string)
		}
		for _, kv := range param.Parameters {
			r[param.Key][kv.Key] = kv.Value
		}
	}
	return r
}
