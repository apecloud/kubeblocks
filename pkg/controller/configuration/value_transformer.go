/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

func (v *valueManager) buildValueTransformer(key string) core.ValueTransformerFunc {
	// NODE: The JSON format requires distinguishing value types, and encode/decode will not perform automatic conversion.
	if format, ok := v.formatConfigs[key]; !ok || format.Format != parametersv1alpha1.JSON {
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
