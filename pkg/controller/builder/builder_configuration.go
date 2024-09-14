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
)

type ConfigurationBuilder struct {
	BaseBuilder[appsv1alpha1.Configuration, *appsv1alpha1.Configuration, ConfigurationBuilder]
}

func NewConfigurationBuilder(namespace, name string) *ConfigurationBuilder {
	builder := &ConfigurationBuilder{}
	builder.init(namespace, name, &appsv1alpha1.Configuration{}, builder)
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

func (c *ConfigurationBuilder) AddConfigurationItem(configSpec appsv1.ComponentConfigSpec) *ConfigurationBuilder {
	c.get().Spec.ConfigItemDetails = append(c.get().Spec.ConfigItemDetails,
		appsv1alpha1.ConfigurationItemDetail{
			Name:       configSpec.Name,
			ConfigSpec: ToV1alpha1ConfigSpec(configSpec.DeepCopy()),
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

func ToV1alpha1ConfigSpec(spec *appsv1.ComponentConfigSpec) *appsv1alpha1.ComponentConfigSpec {
	v1 := &appsv1alpha1.ComponentConfigSpec{
		ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
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
		v1.LegacyRenderedConfigSpec = &appsv1alpha1.LegacyRenderedTemplateSpec{
			ConfigTemplateExtension: appsv1alpha1.ConfigTemplateExtension{
				TemplateRef: spec.LegacyRenderedConfigSpec.TemplateRef,
				Namespace:   spec.LegacyRenderedConfigSpec.Namespace,
				Policy:      appsv1alpha1.MergedPolicy(spec.LegacyRenderedConfigSpec.Policy),
			},
		}
	}
	if spec.ReRenderResourceTypes != nil {
		v1.ReRenderResourceTypes = make([]appsv1alpha1.RerenderResourceType, 0)
		for _, r := range spec.ReRenderResourceTypes {
			v1.ReRenderResourceTypes = append(v1.ReRenderResourceTypes, appsv1alpha1.RerenderResourceType(r))
		}
	}
	return v1
}
