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
