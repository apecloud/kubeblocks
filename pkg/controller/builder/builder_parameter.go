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

package builder

import (
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
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

func (c *ParameterBuilder) SetComponentParameters(component string, parameters parametersv1alpha1.ComponentParameters) *ParameterBuilder {
	componentSpec := safeGetComponentSpec(&c.get().Spec, component)
	componentSpec.Parameters = parameters
	return c
}

func (c *ParameterBuilder) AddCustomTemplate(component string, tpl string, customTemplates parametersv1alpha1.ConfigTemplateExtension) *ParameterBuilder {
	componentSpec := safeGetComponentSpec(&c.get().Spec, component)
	if componentSpec.CustomTemplates == nil {
		componentSpec.CustomTemplates = make(map[string]parametersv1alpha1.ConfigTemplateExtension)
	}
	componentSpec.CustomTemplates[tpl] = customTemplates
	return c
}

func safeGetComponentSpec(spec *parametersv1alpha1.ParameterSpec, component string) *parametersv1alpha1.ComponentParametersSpec {
	for i, parameter := range spec.ComponentParameters {
		if parameter.ComponentName == component {
			return &spec.ComponentParameters[i]
		}
	}
	var size = len(spec.ComponentParameters)
	spec.ComponentParameters = append(spec.ComponentParameters, parametersv1alpha1.ComponentParametersSpec{
		ComponentName: component,
	})
	return &spec.ComponentParameters[size]
}
