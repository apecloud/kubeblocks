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
