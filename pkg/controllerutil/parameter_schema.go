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

package controllerutil

import (
	"slices"

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

func ResolveConfigParameterSchema(paramDef *parametersv1alpha1.ParametersDefinition, configTemplate *appsv1.ComponentFileTemplate) map[string]*ParameterMeta {
	if paramDef.Spec.ParametersSchema == nil || paramDef.Spec.ParametersSchema.SchemaInJSON == nil {
		return nil
	}

	schema := paramDef.Spec.ParametersSchema.SchemaInJSON
	if _, ok := schema.Properties[openapi.DefaultSchemaName]; !ok {
		return nil
	}

	paramMeta := &ParameterMeta{
		FileName:           paramDef.Spec.FileName,
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

func ParametersDrivenConfigRenderTerminalPhases(status parametersv1alpha1.ParamConfigRendererStatus, generation int64) bool {
	return status.ObservedGeneration == generation && status.Phase == parametersv1alpha1.PDAvailablePhase
}

func ParametersTerminalPhases(status parametersv1alpha1.ParameterStatus, generation int64) bool {
	return status.ObservedGeneration == generation && IsParameterFinished(status.Phase)
}

func IsParameterFinished(phase parametersv1alpha1.ParameterPhase) bool {
	return slices.Contains([]parametersv1alpha1.ParameterPhase{
		parametersv1alpha1.CFinishedPhase,
		parametersv1alpha1.CMergeFailedPhase,
		parametersv1alpha1.CFailedAndPausePhase,
	}, phase)
}

func IsFailedPhase(phase parametersv1alpha1.ParameterPhase) bool {
	return slices.Contains([]parametersv1alpha1.ParameterPhase{
		parametersv1alpha1.CMergeFailedPhase,
		parametersv1alpha1.CFailedAndPausePhase,
	}, phase)
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

func GetParameter(spec *parametersv1alpha1.ParameterSpec, component string) *parametersv1alpha1.ComponentParametersSpec {
	match := func(status parametersv1alpha1.ComponentParametersSpec) bool {
		return status.ComponentName == component
	}

	if index := generics.FindFirstFunc(spec.ComponentParameters, match); index >= 0 {
		return &spec.ComponentParameters[index]
	}
	return nil

}

func GetParameterStatus(status *parametersv1alpha1.ParameterStatus, name string) *parametersv1alpha1.ComponentReconfiguringStatus {
	match := func(status parametersv1alpha1.ComponentReconfiguringStatus) bool {
		return status.ComponentName == name
	}

	if index := generics.FindFirstFunc(status.ReconfiguringStatus, match); index >= 0 {
		return &status.ReconfiguringStatus[index]
	}
	return nil
}

func GetParameterReconfiguringStatus(status *parametersv1alpha1.ComponentReconfiguringStatus, name string) *parametersv1alpha1.ReconfiguringStatus {
	match := func(status parametersv1alpha1.ReconfiguringStatus) bool {
		return status.Name == name
	}

	if index := generics.FindFirstFunc(status.ParameterStatus, match); index >= 0 {
		return &status.ParameterStatus[index]
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

func GetComponentConfigDescription(pdcr *parametersv1alpha1.ParamConfigRendererSpec, name string) *parametersv1alpha1.ComponentConfigDescription {
	match := func(desc parametersv1alpha1.ComponentConfigDescription) bool {
		return desc.Name == name
	}

	if index := generics.FindFirstFunc(pdcr.Configs, match); index >= 0 {
		return &pdcr.Configs[index]
	}
	return nil
}

func GetComponentConfigDescriptions(pdcr *parametersv1alpha1.ParamConfigRendererSpec, tpl string) []parametersv1alpha1.ComponentConfigDescription {
	match := func(desc parametersv1alpha1.ComponentConfigDescription) bool {
		return desc.TemplateName == tpl
	}
	return generics.FindFunc(pdcr.Configs, match)
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
