/*
Copyright 2022.

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

package dbaas

import (
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type BuiltinObjectsFunc func() map[string]interface{}

type ResourceDefinition struct {
	MemorySize int64
	CoreNum    int
}

type ComponentTemplateValues struct {
	Name        string
	ServiceName string
	Replicas    int

	// Container *corev1.Container
	Resource  *ResourceDefinition
	ConfigTpl []dbaasv1alpha1.ConfigTemplate
}

type ConfigTemplateBuilder struct {
	namespace   string
	clusterName string

	// Global Var
	componentValues *ComponentTemplateValues
}

func NewCfgTemplateBuilder(clusterName, namespace string) *ConfigTemplateBuilder {
	return &ConfigTemplateBuilder{
		namespace:   namespace,
		clusterName: clusterName,
	}
}

func (c *ConfigTemplateBuilder) Render(configs map[string]string) (map[string]string, error) {
	rendered := make(map[string]string, len(configs))
	engine := intctrlutil.NewTplEngine(c.builtinObjects(), nil)
	for file, configContext := range configs {
		newContext, err := engine.Render(configContext)
		if err != nil {
			return nil, err
		}
		rendered[file] = newContext
	}

	return rendered, nil
}

func (c *ConfigTemplateBuilder) builtinObjects() *intctrlutil.TplValues {
	return &intctrlutil.TplValues{
		"Cluster": map[string]interface{}{
			"Namespace": c.namespace,
			"Name":      c.clusterName,
		},
		"Component": c.componentValues,
	}
}

func (c *ConfigTemplateBuilder) InjectBuiltInObjectsAndFunctions(podTemplate corev1.PodTemplateSpec, configs []dbaasv1alpha1.ConfigTemplate, component *Component, group *RoleGroup) error {
	if err := injectBuiltInObjects(c, podTemplate, component, group, configs); err != nil {
		return err
	}

	if err := injectBuiltInFunctions(c, podTemplate, component, group); err != nil {
		return err
	}
	return nil
}

func injectBuiltInFunctions(tplBuilder *ConfigTemplateBuilder, podTemplate corev1.PodTemplateSpec, component *Component, group *RoleGroup) error {
	// TODO add built-in function
	return nil
}

func injectBuiltInObjects(tplBuilder *ConfigTemplateBuilder, podTemplate corev1.PodTemplateSpec, component *Component, group *RoleGroup, configs []dbaasv1alpha1.ConfigTemplate) error {
	var resource *ResourceDefinition
	container := intctrlutil.GetContainerUsingConfig(podTemplate, configs)
	if container != nil && len(container.Resources.Limits) > 0 {
		resource = &ResourceDefinition{
			MemorySize: intctrlutil.GetMemorySize(*container),
			CoreNum:    intctrlutil.GetCoreNum(*container),
		}
	}

	tplBuilder.componentValues = &ComponentTemplateValues{
		Name: component.Name,
		// TODO add Component service name
		ServiceName: "",
		Replicas:    group.Replicas,
		Resource:    resource,
	}

	return nil
}
