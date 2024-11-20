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
	corev1 "k8s.io/api/core/v1"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
)

func RerenderParametersTemplate(reconcileCtx *render.ReconcileCtx, item parametersv1alpha1.ConfigTemplateItemDetail, configRender *parametersv1alpha1.ParameterDrivenConfigRender, defs []*parametersv1alpha1.ParametersDefinition) (*corev1.ConfigMap, error) {
	parametersValidate := func(m map[string]string) error {
		return validateRenderedData(m, defs, configRender)
	}

	templateRender := render.NewTemplateBuilder(reconcileCtx)
	cmObj, err := render.RenderComponentTemplate(reconcileCtx.Cluster,
		reconcileCtx.SynthesizedComponent,
		templateRender,
		core.GetComponentCfgName(reconcileCtx.SynthesizedComponent.ClusterName, reconcileCtx.SynthesizedComponent.Name, item.ConfigSpec.Name),
		*item.ConfigSpec,
		reconcileCtx.Context,
		reconcileCtx.Client,
		parametersValidate)
	if err != nil {
		return nil, err
	}
	if item.CustomTemplates != nil {
		newData, err := mergerConfigTemplate(*item.CustomTemplates, templateRender, *item.ConfigSpec, cmObj.Data, defs, configRender, reconcileCtx.Context, reconcileCtx.Client)
		if err != nil {
			return nil, err
		}
		cmObj.Data = newData
	}
	return cmObj, nil
}
