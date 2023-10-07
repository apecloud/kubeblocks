/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ConfigurationBuilder struct {
	BaseBuilder[v1alpha1.Configuration, *v1alpha1.Configuration, ConfigurationBuilder]
}

func NewConfigurationBuilder(namespace, name string) *ConfigurationBuilder {
	builder := &ConfigurationBuilder{}
	builder.init(namespace, name, &v1alpha1.Configuration{}, builder)
	return builder
}

func (c *ConfigurationBuilder) ClusterRef(clusterName string) *ConfigurationBuilder {
	c.get().Spec.ClusterRef = clusterName
	return c
}

func (c *ConfigurationBuilder) Component(component string) *ConfigurationBuilder {
	c.get().Spec.ComponentName = component
	return c
}

func (c *ConfigurationBuilder) AddConfigurationItem(configSpec v1alpha1.ComponentConfigSpec) *ConfigurationBuilder {
	c.get().Spec.ConfigItemDetails = append(c.get().Spec.ConfigItemDetails,
		v1alpha1.ConfigurationItemDetail{
			Name:       configSpec.Name,
			ConfigSpec: configSpec.DeepCopy(),
		})
	return c
}
