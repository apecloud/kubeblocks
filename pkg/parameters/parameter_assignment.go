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

package parameters

import (
	"fmt"

	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/parameters/openapi"
)

// ValidateComponentParameterAssignments validates parameter assignments when the component
// has ParametersDefinition JSON schema. Components without schema keep legacy behavior.
func ValidateComponentParameterAssignments(assignments parametersv1alpha1.ComponentParameters,
	paramsDefs []*parametersv1alpha1.ParametersDefinition) error {
	if !hasParameterSchema(paramsDefs) {
		return nil
	}
	for key, value := range assignments {
		paramSchema, ok := findParameterSchema(paramsDefs, key)
		if !ok {
			return fmt.Errorf("parameter %s not found in parameters schema", key)
		}
		if value == nil {
			continue
		}
		schema := &apiext.JSONSchemaProps{
			Type: "object",
			Properties: map[string]apiext.JSONSchemaProps{
				key: paramSchema,
			},
		}
		typedValue, err := common.ConvertStringToInterfaceBySchemaType(schema, map[string]string{key: *value})
		if err != nil {
			return fmt.Errorf("parameter %s value %q is invalid: %w", key, *value, err)
		}
		if err := common.ValidateDataWithSchema(schema, typedValue); err != nil {
			return fmt.Errorf("parameter %s value %q is invalid: %w", key, *value, err)
		}
	}
	return nil
}

func hasParameterSchema(paramsDefs []*parametersv1alpha1.ParametersDefinition) bool {
	for _, paramsDef := range paramsDefs {
		if paramsDef != nil && paramsDef.Spec.ParametersSchema != nil && paramsDef.Spec.ParametersSchema.SchemaInJSON != nil {
			return true
		}
	}
	return false
}

func findParameterSchema(paramsDefs []*parametersv1alpha1.ParametersDefinition, key string) (apiext.JSONSchemaProps, bool) {
	for _, paramsDef := range paramsDefs {
		if paramsDef == nil || paramsDef.Spec.ParametersSchema == nil || paramsDef.Spec.ParametersSchema.SchemaInJSON == nil {
			continue
		}
		specSchema, ok := paramsDef.Spec.ParametersSchema.SchemaInJSON.Properties[openapi.DefaultSchemaName]
		if !ok {
			continue
		}
		flattenedSchema := openapi.FlattenSchema(specSchema)
		if schema, ok := FindParameterSchema(flattenedSchema.Properties, key); ok {
			return schema, true
		}
	}
	return apiext.JSONSchemaProps{}, false
}
