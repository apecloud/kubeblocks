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
	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

type ComponentParameterBuilder struct {
	BaseBuilder[parametersv1alpha1.ComponentParameter, *parametersv1alpha1.ComponentParameter, ComponentParameterBuilder]
}

func NewComponentParameterBuilder(namespace, name string) *ComponentParameterBuilder {
	builder := &ComponentParameterBuilder{}
	builder.init(namespace, name, &parametersv1alpha1.ComponentParameter{}, builder)
	return builder
}

func (c *ComponentParameterBuilder) ClusterRef(clusterName string) *ComponentParameterBuilder {
	c.get().Spec.ClusterName = clusterName
	return c
}

func (c *ComponentParameterBuilder) Component(component string) *ComponentParameterBuilder {
	c.get().Spec.ComponentName = component
	return c
}

func (c *ComponentParameterBuilder) AddConfigurationItem(configSpec appsv1.ComponentTemplateSpec) *ComponentParameterBuilder {
	c.get().Spec.ConfigItemDetails = append(c.get().Spec.ConfigItemDetails,
		parametersv1alpha1.ConfigTemplateItemDetail{
			Name:       configSpec.Name,
			ConfigSpec: configSpec.DeepCopy(),
		})
	return c
}

func (c *ComponentParameterBuilder) SetConfigurationItem(items []parametersv1alpha1.ConfigTemplateItemDetail) *ComponentParameterBuilder {
	c.get().Spec.ConfigItemDetails = items
	return c
}
