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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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

func ToV1ConfigSpec(spec *appsv1alpha1.ComponentConfigSpec) *appsv1.ComponentConfigSpec {
	v1 := &appsv1.ComponentConfigSpec{
		ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
			Name:        spec.Name,
			TemplateRef: spec.TemplateRef,
			Namespace:   spec.Namespace,
			VolumeName:  spec.VolumeName,
			DefaultMode: spec.DefaultMode,
		},
		Keys:                spec.Keys,
		ConfigConstraintRef: spec.ConfigConstraintRef,
		AsEnvFrom:           spec.AsEnvFrom,
		InjectEnvTo:         spec.InjectEnvTo,
		AsSecret:            spec.AsSecret,
	}
	if spec.LegacyRenderedConfigSpec != nil {
		v1.LegacyRenderedConfigSpec = &appsv1.LegacyRenderedTemplateSpec{
			ConfigTemplateExtension: appsv1.ConfigTemplateExtension{
				TemplateRef: spec.LegacyRenderedConfigSpec.TemplateRef,
				Namespace:   spec.LegacyRenderedConfigSpec.Namespace,
				Policy:      appsv1.MergedPolicy(spec.LegacyRenderedConfigSpec.Policy),
			},
		}
	}
	if spec.ReRenderResourceTypes != nil {
		v1.ReRenderResourceTypes = make([]appsv1.RerenderResourceType, 0)
		for _, r := range spec.ReRenderResourceTypes {
			v1.ReRenderResourceTypes = append(v1.ReRenderResourceTypes, appsv1.RerenderResourceType(r))
		}
	}
	return v1
}

func (c *ConfigurationParameterBuilder) SetConfigurationItem(items []configurationv1alpha1.ConfigTemplateItemDetail) *ConfigurationParameterBuilder {
	c.get().Spec.ConfigItemDetails = items
	return c
}
