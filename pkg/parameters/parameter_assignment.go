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
	"context"
	"fmt"

	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	"github.com/apecloud/kubeblocks/pkg/parameters/openapi"
)

type AssignmentValidationError struct {
	Err error
}

func (e *AssignmentValidationError) Error() string {
	return e.Err.Error()
}

func (e *AssignmentValidationError) Unwrap() error {
	return e.Err
}

type AssignmentTargetNotFoundError struct {
	ComponentName string
}

func (e *AssignmentTargetNotFoundError) Error() string {
	return fmt.Sprintf("component not found: %s", e.ComponentName)
}

// ValidateComponentParameterAssignmentsForCluster resolves the component's ParametersDefinition
// and validates assignments at the parameters layer.
func ValidateComponentParameterAssignmentsForCluster(ctx context.Context, reader client.Reader,
	cluster *appsv1.Cluster, componentName string, assignments parametersv1alpha1.ComponentParameters) error {
	if len(assignments) == 0 {
		return nil
	}
	compDefName, err := resolveComponentDefinitionName(ctx, reader, cluster, componentName)
	if err != nil {
		return err
	}
	compDef, err := component.GetCompDefByName(ctx, reader, compDefName)
	if err != nil {
		return err
	}
	_, paramsDefs, err := ResolveCmpdParametersDefs(ctx, reader, compDef)
	if err != nil {
		return err
	}
	if err := ValidateComponentParameterAssignments(assignments, paramsDefs); err != nil {
		return &AssignmentValidationError{Err: err}
	}
	return nil
}

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

func resolveComponentDefinitionName(ctx context.Context, reader client.Reader, cluster *appsv1.Cluster, componentName string) (string, error) {
	if compSpec := cluster.Spec.GetComponentByName(componentName); compSpec != nil {
		if compSpec.ComponentDef != "" {
			return compSpec.ComponentDef, nil
		}
		return resolveComponentDefinitionNameFromComponent(ctx, reader, cluster, componentName)
	}
	if shardingSpec := cluster.Spec.GetShardingByName(componentName); shardingSpec != nil {
		if shardingSpec.Template.ComponentDef != "" {
			return shardingSpec.Template.ComponentDef, nil
		}
		comps, err := sharding.ListShardingComponents(ctx, reader, cluster, componentName)
		if err != nil {
			return "", err
		}
		if len(comps) == 0 {
			return "", fmt.Errorf("no component found for sharding %s", componentName)
		}
		if comps[0].Spec.CompDef == "" {
			return "", fmt.Errorf("component definition is empty for sharding %s", componentName)
		}
		return comps[0].Spec.CompDef, nil
	}
	return "", &AssignmentTargetNotFoundError{ComponentName: componentName}
}

func resolveComponentDefinitionNameFromComponent(ctx context.Context, reader client.Reader, cluster *appsv1.Cluster, componentName string) (string, error) {
	comp, err := component.GetComponentByName(ctx, reader, cluster.Namespace, component.FullName(cluster.Name, componentName))
	if err != nil {
		return "", err
	}
	if comp.Spec.CompDef == "" {
		return "", fmt.Errorf("component definition is empty for component %s", componentName)
	}
	return comp.Spec.CompDef, nil
}
