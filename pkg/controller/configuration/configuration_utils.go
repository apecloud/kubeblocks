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
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configurationv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func ClassifyParamsFromConfigTemplate(ctx context.Context,
	cli client.Reader,
	component *appsv1.Component,
	componentDef *appsv1.ComponentDefinition,
	synthesizedComponent *component.SynthesizedComponent) ([]configurationv1alpha1.ConfigTemplateItemDetail, error) {
	var itemDetails []configurationv1alpha1.ConfigTemplateItemDetail

	classifyParams, err := classifyComponentParameters(ctx, cli,
		component.Spec.ComponentParameters,
		componentDef.Spec.ParametersDescriptions,
		synthesizedComponent.ConfigTemplates)
	if err != nil {
		return nil, err
	}
	for _, template := range synthesizedComponent.ConfigTemplates {
		itemDetails = append(itemDetails, generateConfigTemplateItem(component.Spec.UserConfigTemplates, classifyParams, template))
	}
	return itemDetails, nil
}

func generateConfigTemplateItem(userConfigTemplates map[string]appsv1.ConfigTemplateExtension, configParams map[string]map[string]*configurationv1alpha1.ParametersInFile, template appsv1.ComponentConfigSpec) configurationv1alpha1.ConfigTemplateItemDetail {
	itemDetail := configurationv1alpha1.ConfigTemplateItemDetail{
		Name:       template.Name,
		ConfigSpec: template.DeepCopy(),
	}

	if v, ok := userConfigTemplates[template.Name]; ok {
		itemDetail.UserConfigTemplates = v.DeepCopy()
	}
	if configParams != nil {
		itemDetail.ConfigFileParams = derefMapValues(configParams[template.Name])
	}
	return itemDetail
}

func classifyComponentParameters(ctx context.Context,
	reader client.Reader,
	parameters appsv1.ComponentParameters,
	parametersDefs []appsv1.ComponentParametersDescription,
	templates []appsv1.ComponentConfigSpec) (map[string]map[string]*configurationv1alpha1.ParametersInFile, error) {
	if len(parameters) == 0 {
		return nil, nil
	}
	if len(parametersDefs) == 1 {
		return transformParametersInFile(parametersDefs[0], templates, parameters)
	}

	parametersMap, err := getSchemaFromParametersDefinition(ctx, reader, parametersDefs, templates)
	if err != nil {
		return nil, err
	}
	classifyParams := make(map[string]map[string]*configurationv1alpha1.ParametersInFile, len(templates))
	for paramKey, paramValue := range parameters {
		updateConfigParameter(paramKey, paramValue, parametersMap, classifyParams)
	}
	return classifyParams, nil
}

func updateConfigParameter(paramKey string, paramValue *string, parametersMap map[string]*controllerutil.ParameterMeta, classifyParams map[string]map[string]*configurationv1alpha1.ParametersInFile) {

	deRefParamInTemplate := func(name string) map[string]*configurationv1alpha1.ParametersInFile {
		if _, ok := classifyParams[name]; !ok {
			classifyParams[name] = make(map[string]*configurationv1alpha1.ParametersInFile)
		}
		return classifyParams[name]
	}
	deRefParamInFile := func(templateName, fileName string) *configurationv1alpha1.ParametersInFile {
		v := deRefParamInTemplate(templateName)
		if _, ok := v[fileName]; !ok {
			v[fileName] = &configurationv1alpha1.ParametersInFile{
				Parameters: make(map[string]*string),
			}
		}
		return v[fileName]
	}

	meta, ok := parametersMap[paramKey]
	if !ok {
		// logger.V(1).Info("ignore invalid param", "param", paramKey)
		return
	}
	deRefParamInFile(meta.ConfigTemplateName, meta.FileName).Parameters[paramKey] = paramValue
}

func getSchemaFromParametersDefinition(ctx context.Context, reader client.Reader, parametersDefs []appsv1.ComponentParametersDescription, templates []appsv1.ComponentConfigSpec) (map[string]*controllerutil.ParameterMeta, error) {
	paramMeta := make(map[string]*controllerutil.ParameterMeta)

	mergeParams := func(params map[string]*controllerutil.ParameterMeta) {
		for key, meta := range params {
			paramMeta[key] = meta
		}
	}

	for _, parameterDef := range parametersDefs {
		configSpec := fromConfigSpecFromParam(templates, parameterDef)
		if configSpec != nil {
			return nil, fmt.Errorf("not found config template: [%v]", parameterDef)
		}
		metas, err := controllerutil.GetConfigParameterMeta(ctx, reader, parameterDef, configSpec)
		if err != nil {
			return nil, err
		}
		mergeParams(metas)
	}
	return paramMeta, nil
}

func transformParametersInFile(paramDef appsv1.ComponentParametersDescription,
	templates []appsv1.ComponentConfigSpec,
	parameters appsv1.ComponentParameters) (map[string]map[string]*configurationv1alpha1.ParametersInFile, error) {
	configSpec := fromConfigSpecFromParam(templates, paramDef)
	if configSpec != nil {
		return nil, fmt.Errorf("not found config template: [%v]", paramDef)
	}
	return map[string]map[string]*configurationv1alpha1.ParametersInFile{
		configSpec.Name: {
			paramDef.Name: &configurationv1alpha1.ParametersInFile{
				Parameters: parameters,
			}},
	}, nil
}

func fromConfigSpecFromParam(templates []appsv1.ComponentConfigSpec, description appsv1.ComponentParametersDescription) *appsv1.ComponentConfigSpec {
	for i := range templates {
		template := &templates[i]
		for _, configDescription := range template.ComponentConfigDescriptions {
			if configDescription.Name == description.Name {
				return template
			}
		}
	}
	return nil
}

func derefMapValues(m map[string]*configurationv1alpha1.ParametersInFile) map[string]configurationv1alpha1.ParametersInFile {
	if len(m) == 1 {
		return nil
	}

	newMap := make(map[string]configurationv1alpha1.ParametersInFile, len(m))
	for key, inFile := range m {
		newMap[key] = *inFile
	}
	return newMap
}
