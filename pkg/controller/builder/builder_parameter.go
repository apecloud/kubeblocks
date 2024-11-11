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

package builder

import (
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ParameterBuilder struct {
	BaseBuilder[parametersv1alpha1.Parameter, *parametersv1alpha1.Parameter, ParameterBuilder]
}

func NewParameterBuilder(namespace, name string) *ParameterBuilder {
	builder := &ParameterBuilder{}
	builder.init(namespace, name, &parametersv1alpha1.Parameter{}, builder)
	return builder
}

func (c *ParameterBuilder) ClusterRef(clusterName string) *ParameterBuilder {
	c.get().Spec.ClusterName = clusterName
	return c
}

func (c *ParameterBuilder) SetComponentParameters(component string, parameters appsv1.ComponentParameters) *ParameterBuilder {
	componentSpec := intctrlutil.GetParameter(&c.get().Spec, component)
	if componentSpec != nil {
		componentSpec.Parameters = parameters
		return c
	}
	c.get().Spec.ComponentParameters = append(c.get().Spec.ComponentParameters, parametersv1alpha1.ComponentParametersSpec{
		ComponentName: component,
		Parameters:    parameters,
	})
	return c
}

func (c *ParameterBuilder) AddCustomTemplate(component string, tpl string, customTemplates appsv1.ConfigTemplateExtension) *ParameterBuilder {
	componentSpec := intctrlutil.GetParameter(&c.get().Spec, component)
	if componentSpec == nil {
		c.get().Spec.ComponentParameters = append(c.get().Spec.ComponentParameters, parametersv1alpha1.ComponentParametersSpec{
			ComponentName: component,
		})
	}
	componentSpec = intctrlutil.GetParameter(&c.get().Spec, component)
	if componentSpec.CustomTemplates == nil {
		componentSpec.CustomTemplates = make(map[string]appsv1.ConfigTemplateExtension)
	}
	componentSpec.CustomTemplates[tpl] = customTemplates
	return c
}
