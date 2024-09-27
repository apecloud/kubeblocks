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

package configuration

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type TemplateMerger interface {

	// Merge merges the baseData with the data from the template.
	Merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error)

	// renderTemplate renders the template and returns the data.
	renderTemplate() (map[string]string, error)
}

type mergeContext struct {
	template   appsv1.ConfigTemplateExtension
	configSpec appsv1.ComponentConfigSpec
	ccSpec     *appsv1beta1.ConfigConstraintSpec

	builder *configTemplateBuilder
	ctx     context.Context
	client  client.Client
}

func (m *mergeContext) renderTemplate() (map[string]string, error) {
	templateSpec := appsv1.ComponentTemplateSpec{
		// Name:        m.template.Name,
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

type noneOp struct {
	*mergeContext
}

func (n noneOp) Merge(_ map[string]string, updatedData map[string]string) (map[string]string, error) {
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

func (c *configPatcher) Merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	formatter := c.ccSpec.FileFormatConfig
	configPatch, err := core.TransformConfigPatchFromData(updatedData, formatter.Format, c.configSpec.Keys)
	if err != nil {
		return nil, err
	}
	if !configPatch.IsModify {
		return baseData, nil
	}

	mergedData := copyMap(baseData)
	params := core.GenerateVisualizedParamsList(configPatch, formatter, nil)
	for key, patch := range splitParameters(params) {
		v, ok := baseData[key]
		if !ok {
			mergedData[key] = updatedData[key]
			continue
		}
		newConfig, err := core.ApplyConfigPatch([]byte(v), patch, formatter)
		if err != nil {
			return nil, err
		}
		mergedData[key] = newConfig
	}
	return mergedData, err
}

func (c *configReplaceMerger) Merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	return core.MergeUpdatedConfig(baseData, updatedData), nil
}

func (c *configOnlyAddMerger) Merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	return nil, core.MakeError("not implemented")
}

func NewTemplateMerger(template appsv1.ConfigTemplateExtension, ctx context.Context, cli client.Client, builder *configTemplateBuilder, configSpec appsv1.ComponentConfigSpec, ccSpec *appsv1beta1.ConfigConstraintSpec) (TemplateMerger, error) {
	templateData := &mergeContext{
		configSpec: configSpec,
		template:   template,
		ctx:        ctx,
		client:     cli,
		builder:    builder,
		ccSpec:     ccSpec,
	}

	var merger TemplateMerger
	switch template.Policy {
	default:
		return nil, core.MakeError("unknown template policy: %s", template.Policy)
	case appsv1.NoneMergePolicy:
		merger = &noneOp{templateData}
	case appsv1.PatchPolicy:
		merger = &configPatcher{templateData}
	case appsv1.OnlyAddPolicy:
		merger = &configOnlyAddMerger{templateData}
	case appsv1.ReplacePolicy:
		merger = &configReplaceMerger{templateData}
	}
	return merger, nil
}

func mergerConfigTemplate(template *appsv1.LegacyRenderedTemplateSpec,
	builder *configTemplateBuilder,
	configSpec appsv1.ComponentConfigSpec,
	baseData map[string]string,
	ctx context.Context, cli client.Client) (map[string]string, error) {
	if configSpec.ConfigConstraintRef == "" {
		return nil, core.MakeError("ConfigConstraintRef require not empty, configSpec[%v]", configSpec.Name)
	}
	ccObj := &appsv1beta1.ConfigConstraint{}
	ccKey := client.ObjectKey{
		Namespace: "",
		Name:      configSpec.ConfigConstraintRef,
	}
	if err := cli.Get(ctx, ccKey, ccObj); err != nil {
		return nil, core.WrapError(err, "failed to get ConfigConstraint, key[%v]", configSpec)
	}
	if ccObj.Spec.FileFormatConfig == nil {
		return nil, core.MakeError("importedConfigTemplate require ConfigConstraint.Spec.FileFormatConfig, configSpec[%v]", configSpec)
	}

	templateMerger, err := NewTemplateMerger(template.ConfigTemplateExtension, ctx, cli, builder, configSpec, &ccObj.Spec)
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
	return templateMerger.Merge(baseData, data)
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
