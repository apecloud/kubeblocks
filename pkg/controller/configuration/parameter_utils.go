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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func ClassifyParamsFromConfigTemplate(params parametersv1alpha1.ComponentParameters,
	cmpd *appsv1.ComponentDefinition,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	tpls map[string]*corev1.ConfigMap) []parametersv1alpha1.ConfigTemplateItemDetail {
	var itemDetails []parametersv1alpha1.ConfigTemplateItemDetail

	configs := intctrlutil.TransformConfigTemplate(cmpd.Spec.Configs)
	classifyParams := ClassifyComponentParameters(params, paramsDefs, configs, tpls)
	for _, template := range configs {
		itemDetails = append(itemDetails, generateConfigTemplateItem(classifyParams, template))
	}
	return itemDetails
}

func generateConfigTemplateItem(configParams map[string]map[string]*parametersv1alpha1.ParametersInFile, template appsv1.ComponentTemplateSpec) parametersv1alpha1.ConfigTemplateItemDetail {
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
	templates []appsv1.ComponentTemplateSpec,
	tpls map[string]*corev1.ConfigMap) map[string]map[string]*parametersv1alpha1.ParametersInFile {
	if len(parameters) == 0 || len(parametersDefs) == 0 {
		return nil
	}
	if len(parametersDefs) == 1 {
		return transformParametersInFile(parametersDefs[0], templates, parameters, tpls)
	}

	classifyParams := make(map[string]map[string]*parametersv1alpha1.ParametersInFile, len(templates))
	parametersMap := resolveSchemaFromParametersDefinition(parametersDefs, templates, tpls)
	for paramKey, paramValue := range parameters {
		updateConfigParameter(paramKey, paramValue, parametersMap, classifyParams)
	}
	return classifyParams
}

func updateConfigParameter(paramKey string,
	paramValue *string,
	parametersMap map[string]*intctrlutil.ParameterMeta,
	classifyParams map[string]map[string]*parametersv1alpha1.ParametersInFile) {

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

	meta, ok := parametersMap[paramKey]
	if !ok {
		log.Log.Info("ignore invalid param", "param", paramKey)
		return
	}
	deRefParamInFile(meta.ConfigTemplateName, meta.FileName).Parameters[paramKey] = paramValue
}

func resolveSchemaFromParametersDefinition(parametersDefs []*parametersv1alpha1.ParametersDefinition,
	templates []appsv1.ComponentTemplateSpec,
	tpls map[string]*corev1.ConfigMap) map[string]*intctrlutil.ParameterMeta {
	paramMeta := make(map[string]*intctrlutil.ParameterMeta)
	mergeParams := func(params map[string]*intctrlutil.ParameterMeta) {
		for key, meta := range params {
			paramMeta[key] = meta
		}
	}
	for _, parameterDef := range parametersDefs {
		configSpec := resolveConfigSpecFromParametersDefinition(templates, parameterDef, tpls)
		if configSpec != nil {
			mergeParams(intctrlutil.ResolveConfigParameterSchema(parameterDef, configSpec))
		}
	}
	return paramMeta
}

func transformParametersInFile(paramDef *parametersv1alpha1.ParametersDefinition,
	templates []appsv1.ComponentTemplateSpec,
	parameters parametersv1alpha1.ComponentParameters,
	tpls map[string]*corev1.ConfigMap) map[string]map[string]*parametersv1alpha1.ParametersInFile {
	configSpec := resolveConfigSpecFromParametersDefinition(templates, paramDef, tpls)
	if configSpec == nil {
		ctrl.Log.Info(fmt.Sprintf("not found config template: [%v]", paramDef))
		return nil
	}
	return map[string]map[string]*parametersv1alpha1.ParametersInFile{
		configSpec.Name: {
			paramDef.Spec.FileName: &parametersv1alpha1.ParametersInFile{
				Parameters: parameters,
			}},
	}
}

func resolveConfigSpecFromParametersDefinition(templates []appsv1.ComponentTemplateSpec,
	paramDef *parametersv1alpha1.ParametersDefinition,
	tpls map[string]*corev1.ConfigMap) *appsv1.ComponentTemplateSpec {
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
