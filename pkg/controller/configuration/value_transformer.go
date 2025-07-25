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

	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
)

type defaultValueTransformer struct {
	flattenedSchema apiextv1.JSONSchemaProps
}

func (d *defaultValueTransformer) resolveValueWithType(value string, fieldName string) (any, error) {
	schema, ok := d.flattenedSchema.Properties[fieldName]
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

type ValueTransformerBuilder interface {
	BuildValueTransformer() core.ValueTransformerFunc
}

type valueManager struct {
	paramsDef     *appsv1beta1.ConfigConstraint
	formatConfigs *appsv1beta1.FileFormatConfig
}

func needValueTransformer(formatter appsv1beta1.CfgFileFormat) bool {
	return formatter == appsv1beta1.JSON ||
		formatter == appsv1beta1.YAML
}

func (v *valueManager) BuildValueTransformer() core.ValueTransformerFunc {
	// NODE: The JSON format requires distinguishing value types, and encode/decode will not perform automatic conversion.
	if v.formatConfigs == nil || !needValueTransformer(v.formatConfigs.Format) || v.paramsDef.Spec.ParametersSchema == nil {
		return nil
	}

	schema := v.paramsDef.Spec.ParametersSchema.SchemaInJSON
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

func NewValueManager(paramsDefs *appsv1beta1.ConfigConstraint) ValueTransformerBuilder {
	return &valueManager{
		paramsDef:     paramsDefs,
		formatConfigs: paramsDefs.Spec.FileFormatConfig,
	}
}
