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

package controllerutil

import (
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
)

type ParameterMeta struct {
	FileName           string
	ConfigTemplateName string
}

func ResolveConfigParameterSchema(paramDef *parametersv1alpha1.ParametersDefinition, configTemplate *appsv1.ComponentTemplateSpec) map[string]*ParameterMeta {
	if paramDef.Spec.ParametersSchema == nil || paramDef.Spec.ParametersSchema.SchemaInJSON == nil {
		return nil
	}

	schema := paramDef.Spec.ParametersSchema.SchemaInJSON
	if _, ok := schema.Properties[openapi.DefaultSchemaName]; !ok {
		return nil
	}

	paramMeta := &ParameterMeta{
		FileName:           paramDef.Name,
		ConfigTemplateName: configTemplate.Name,
	}
	props := openapi.FlattenSchema(schema.Properties[openapi.DefaultSchemaName]).Properties
	params := make(map[string]*ParameterMeta, len(props))
	for key := range props {
		params[key] = paramMeta
	}

	return params
}

func ParametersDefinitionTerminalPhases(status parametersv1alpha1.ParametersDefinitionStatus, generation int64) bool {
	return status.ObservedGeneration == generation && status.Phase == parametersv1alpha1.PDAvailablePhase
}

func GetItemStatus(status *parametersv1alpha1.ComponentParameterStatus, name string) *parametersv1alpha1.ConfigTemplateItemDetailStatus {
	for i := range status.ConfigurationItemStatus {
		itemStatus := &status.ConfigurationItemStatus[i]
		if itemStatus.Name == name {
			return itemStatus
		}
	}
	return nil
}

func GetConfigTemplateItem(parameterSpec *parametersv1alpha1.ComponentParameterSpec, name string) *parametersv1alpha1.ConfigTemplateItemDetail {
	for i := range parameterSpec.ConfigItemDetails {
		item := &parameterSpec.ConfigItemDetails[i]
		if item.Name == name {
			return item
		}
	}
	return nil
}
