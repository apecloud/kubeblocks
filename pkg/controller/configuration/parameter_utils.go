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
	"fmt"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

func ClassifyParamsFromConfigTemplate(params parametersv1alpha1.ComponentParameters,
	cmpd *appsv1.ComponentDefinition,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	tpls map[string]*corev1.ConfigMap,
	pcr *parametersv1alpha1.ParamConfigRenderer) ([]parametersv1alpha1.ConfigTemplateItemDetail, error) {
	var itemDetails []parametersv1alpha1.ConfigTemplateItemDetail

	classifyParams, err := ClassifyComponentParameters(params, paramsDefs, cmpd.Spec.Configs, tpls, pcr)
	if err != nil {
		return nil, err
	}
	for _, template := range cmpd.Spec.Configs {
		itemDetails = append(itemDetails, generateConfigTemplateItem(classifyParams, template))
	}
	return itemDetails, nil
}

func generateConfigTemplateItem(configParams map[string]map[string]*parametersv1alpha1.ParametersInFile, template appsv1.ComponentFileTemplate) parametersv1alpha1.ConfigTemplateItemDetail {
	itemDetail := parametersv1alpha1.ConfigTemplateItemDetail{
		Name:       template.Name,
		ConfigSpec: template.DeepCopy(),
	}

	if tls, ok := configParams[template.Name]; ok {
		itemDetail.ConfigFileParams = DerefMapValues(tls)
	}
	return itemDetail
}

func ClassifyComponentParameters(parameters parametersv1alpha1.ComponentParameters,
	parametersDefs []*parametersv1alpha1.ParametersDefinition,
	templates []appsv1.ComponentFileTemplate,
	tpls map[string]*corev1.ConfigMap,
	pcr *parametersv1alpha1.ParamConfigRenderer) (map[string]map[string]*parametersv1alpha1.ParametersInFile, error) {
	if len(parameters) == 0 {
		return nil, nil
	}
	if !hasValidParametersDefinition(parametersDefs) {
		return transformDefaultParameters(parameters, pcr)
	}

	classifyParams := make(map[string]map[string]*parametersv1alpha1.ParametersInFile, len(templates))
	parametersMap, err := resolveSchemaFromParametersDefinition(parametersDefs, templates, tpls)
	if err != nil {
		return nil, err
	}
	for paramKey, paramValue := range parameters {
		if err = updateConfigParameter(paramKey, paramValue, parametersMap, classifyParams); err != nil {
			return nil, err
		}
	}
	return classifyParams, nil
}

func updateConfigParameter(paramKey string,
	paramValue *string,
	parametersMap map[string]*intctrlutil.ParameterMeta,
	classifyParams map[string]map[string]*parametersv1alpha1.ParametersInFile) error {

	deRefParamInTemplate := func(name string) map[string]*parametersv1alpha1.ParametersInFile {
		if _, ok := classifyParams[name]; !ok {
			classifyParams[name] = make(map[string]*parametersv1alpha1.ParametersInFile)
		}
		return classifyParams[name]
	}
	deRefParamInFile := func(templateName, fileName string) *parametersv1alpha1.ParametersInFile {
		v := deRefParamInTemplate(templateName)
		if _, ok := v[fileName]; !ok {
			v[fileName] = &parametersv1alpha1.ParametersInFile{
				Parameters: make(map[string]*string),
			}
		}
		return v[fileName]
	}

	parameterMeta, ok := parametersMap[paramKey]
	if !ok {
		return fmt.Errorf("parameter %s not found in parameters schema", paramKey)
	}
	deRefParamInFile(parameterMeta.ConfigTemplateName, parameterMeta.FileName).Parameters[paramKey] = paramValue
	return nil
}

func resolveSchemaFromParametersDefinition(parametersDefs []*parametersv1alpha1.ParametersDefinition,
	templates []appsv1.ComponentFileTemplate,
	tpls map[string]*corev1.ConfigMap) (map[string]*intctrlutil.ParameterMeta, error) {
	paramMeta := make(map[string]*intctrlutil.ParameterMeta)
	mergeParams := func(params map[string]*intctrlutil.ParameterMeta) {
		for key, meta := range params {
			paramMeta[key] = meta
		}
	}
	for _, parameterDef := range parametersDefs {
		configSpec := resolveConfigSpecFromParametersDefinition(templates, parameterDef, tpls)
		if configSpec == nil {
			return nil, fmt.Errorf("config template not found for parameters definition %s", parameterDef.Name)
		}
		mergeParams(intctrlutil.ResolveConfigParameterSchema(parameterDef, configSpec))
	}
	return paramMeta, nil
}

func hasValidParametersDefinition(defs []*parametersv1alpha1.ParametersDefinition) bool {
	if len(defs) == 0 {
		return false
	}
	match := func(def *parametersv1alpha1.ParametersDefinition) bool {
		return def.Spec.ParametersSchema != nil
	}
	return generics.CountFunc(defs, match) != 0
}

func transformDefaultParameters(
	parameters parametersv1alpha1.ComponentParameters,
	pcr *parametersv1alpha1.ParamConfigRenderer) (map[string]map[string]*parametersv1alpha1.ParametersInFile, error) {

	match := func(config parametersv1alpha1.ComponentConfigDescription) bool {
		return config.TemplateName != "" && config.FileFormatConfig != nil
	}
	configs := generics.FindFunc(pcr.Spec.Configs, match)
	if len(configs) == 0 {
		return nil, fmt.Errorf("the component does not support parameters reconfigure")
	}
	config := configs[0]
	return map[string]map[string]*parametersv1alpha1.ParametersInFile{
		config.TemplateName: {
			config.Name: &parametersv1alpha1.ParametersInFile{
				Parameters: parameters,
			}},
	}, nil
}

func resolveConfigSpecFromParametersDefinition(templates []appsv1.ComponentFileTemplate,
	paramDef *parametersv1alpha1.ParametersDefinition,
	tpls map[string]*corev1.ConfigMap) *appsv1.ComponentFileTemplate {
	for i, item := range templates {
		tpl, ok := tpls[item.Name]
		if !ok {
			continue
		}
		if _, ok = tpl.Data[paramDef.Spec.FileName]; ok {
			return &templates[i]
		}
	}
	return nil
}

func DerefMapValues(m map[string]*parametersv1alpha1.ParametersInFile) map[string]parametersv1alpha1.ParametersInFile {
	if len(m) == 0 {
		return nil
	}

	newMap := make(map[string]parametersv1alpha1.ParametersInFile, len(m))
	for key, inFile := range m {
		newMap[key] = *inFile
	}
	return newMap
}
