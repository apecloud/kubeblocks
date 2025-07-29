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

package configuration

import (
	"strconv"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type defaultValueTransformer struct {
	flattenedSchema apiextv1.JSONSchemaProps
}

func (d *defaultValueTransformer) resolveValueWithType(value string, fieldName string) (any, error) {
	schema, ok := FindParameterSchema(d.flattenedSchema.Properties, fieldName)
	if !ok {
		return value, nil
	}
	switch schema.Type {
	default:
		return value, nil
	case "integer":
		return strconv.ParseInt(value, 10, 64)
	case "number":
		return strconv.ParseFloat(value, 64)
	case "boolean":
		return strconv.ParseBool(value)
	}
}

type valueManager struct {
	paramsDefs    []*parametersv1alpha1.ParametersDefinition
	formatConfigs map[string]parametersv1alpha1.FileFormatConfig
}

func needValueTransformer(formatter parametersv1alpha1.CfgFileFormat) bool {
	return formatter == parametersv1alpha1.JSON ||
		formatter == parametersv1alpha1.YAML ||
		formatter == parametersv1alpha1.TOML
}

func (v *valueManager) BuildValueTransformer(key string) core.ValueTransformerFunc {
	// NODE: The JSON format requires distinguishing value types, and encode/decode will not perform automatic conversion.
	if format, ok := v.formatConfigs[key]; !ok || !needValueTransformer(format.Format) {
		return nil
	}
	index := generics.FindFirstFunc(v.paramsDefs, func(paramDef *parametersv1alpha1.ParametersDefinition) bool {
		return paramDef.Spec.FileName == key
	})
	if index < 0 || v.paramsDefs[index].Spec.ParametersSchema == nil {
		return nil
	}
	schema := v.paramsDefs[index].Spec.ParametersSchema.SchemaInJSON
	if _, ok := schema.Properties[openapi.DefaultSchemaName]; !ok {
		return nil
	}
	defaultTransformer := &defaultValueTransformer{
		openapi.FlattenSchema(schema.Properties[openapi.DefaultSchemaName]),
	}
	return func(value string, fieldName string) (any, error) {
		return defaultTransformer.resolveValueWithType(value, fieldName)
	}
}

func NewValueManager(paramsDefs []*parametersv1alpha1.ParametersDefinition, configs []parametersv1alpha1.ComponentConfigDescription) *valueManager {
	manager := &valueManager{
		paramsDefs:    paramsDefs,
		formatConfigs: make(map[string]parametersv1alpha1.FileFormatConfig),
	}
	for _, config := range configs {
		if config.FileFormatConfig != nil {
			manager.formatConfigs[config.Name] = *config.FileFormatConfig
		}
	}
	return manager
}
