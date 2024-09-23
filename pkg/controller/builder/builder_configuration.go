/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	configurationv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
)

type ConfigurationParameterBuilder struct {
	BaseBuilder[configurationv1alpha1.ComponentParameter, *configurationv1alpha1.ComponentParameter, ConfigurationParameterBuilder]
}

func NewConfigurationBuilder(namespace, name string) *ConfigurationParameterBuilder {
	builder := &ConfigurationParameterBuilder{}
	builder.init(namespace, name, &configurationv1alpha1.ComponentParameter{}, builder)
	return builder
}

func (c *ConfigurationParameterBuilder) ClusterRef(clusterName string) *ConfigurationParameterBuilder {
	c.get().Spec.ClusterName = clusterName
	return c
}

func (c *ConfigurationParameterBuilder) Component(component string) *ConfigurationParameterBuilder {
	c.get().Spec.ComponentName = component
	return c
}

func (c *ConfigurationParameterBuilder) AddConfigurationItem(configSpec appsv1.ComponentConfigSpec) *ConfigurationParameterBuilder {
	c.get().Spec.ConfigItemDetails = append(c.get().Spec.ConfigItemDetails,
		configurationv1alpha1.ConfigTemplateItemDetail{
			Name:       configSpec.Name,
			ConfigSpec: configSpec.DeepCopy(),
		})
	return c
}

func (c *ConfigurationParameterBuilder) SetConfigurationItem(items []configurationv1alpha1.ConfigTemplateItemDetail) *ConfigurationParameterBuilder {
	c.get().Spec.ConfigItemDetails = items
	return c
}
