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

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
)

func RerenderParametersTemplate(reconcileCtx *render.ReconcileCtx,
	item parametersv1alpha1.ConfigTemplateItemDetail,
	configRender *parametersv1alpha1.ParamConfigRenderer,
	parametersDefs []*parametersv1alpha1.ParametersDefinition) (*corev1.ConfigMap, error) {
	parametersValidate := func(m map[string]string) error {
		return validateRenderedData(m, parametersDefs, configRender)
	}

	configSpec := *item.ConfigSpec
	templateRender := render.NewTemplateBuilder(reconcileCtx)
	rerenderCMObj, err := templateRender.RenderComponentTemplate(configSpec,
		configcore.GetComponentCfgName(reconcileCtx.SynthesizedComponent.ClusterName,
			reconcileCtx.SynthesizedComponent.Name,
			item.ConfigSpec.Name),
		parametersValidate)
	if err != nil {
		return nil, err
	}
	if item.CustomTemplates == nil {
		return rerenderCMObj, nil
	}

	mergedData, err := mergerConfigTemplate(*item.CustomTemplates,
		templateRender,
		configSpec,
		rerenderCMObj.Data,
		parametersDefs,
		configRender)
	if err != nil {
		return nil, err
	}
	rerenderCMObj.Data = mergedData
	return rerenderCMObj, nil
}

func ApplyParameters(item parametersv1alpha1.ConfigTemplateItemDetail,
	baseConfig *corev1.ConfigMap,
	configRender *parametersv1alpha1.ParamConfigRenderer,
	paramsDefs []*parametersv1alpha1.ParametersDefinition) (*corev1.ConfigMap, error) {
	if configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil, fmt.Errorf("not support parameter reconfigure")
	}

	newData, err := DoMerge(baseConfig.Data, item.ConfigFileParams, paramsDefs, configRender.Spec.Configs)
	if err != nil {
		return nil, err
	}

	expected := baseConfig.DeepCopy()
	expected.Data = newData
	return expected, nil
}
