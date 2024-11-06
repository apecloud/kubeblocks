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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
	"github.com/apecloud/kubeblocks/pkg/generics"
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
	match := func(status parametersv1alpha1.ConfigTemplateItemDetailStatus) bool {
		return status.Name == name
	}

	if index := generics.FindFirstFunc(status.ConfigurationItemStatus, match); index >= 0 {
		return &status.ConfigurationItemStatus[index]
	}

	return nil
}

func GetConfigTemplateItem(parameterSpec *parametersv1alpha1.ComponentParameterSpec, name string) *parametersv1alpha1.ConfigTemplateItemDetail {
	match := func(spec parametersv1alpha1.ConfigTemplateItemDetail) bool {
		return spec.Name == name
	}

	if index := generics.FindFirstFunc(parameterSpec.ConfigItemDetails, match); index >= 0 {
		return &parameterSpec.ConfigItemDetails[index]
	}
	return nil
}

func GetComponentConfigDescription(pdcr *parametersv1alpha1.ParameterDrivenConfigRenderSpec, name string) *parametersv1alpha1.ComponentConfigDescription {
	match := func(desc parametersv1alpha1.ComponentConfigDescription) bool {
		return desc.Name == name
	}

	if index := generics.FindFirstFunc(pdcr.Configs, match); index >= 0 {
		return &pdcr.Configs[index]
	}
	return nil
}

func GetPodSelector(pd *parametersv1alpha1.ParametersDefinitionSpec) *metav1.LabelSelector {
	if pd.ReloadAction != nil {
		return pd.ReloadAction.TargetPodSelector
	}
	return nil
}

func AsSidecarContainerImage(toolImage parametersv1alpha1.ToolConfig) bool {
	return toolImage.AsContainerImage != nil && *toolImage.AsContainerImage
}
