/*
Copyright 2022 The KubeBlocks Authors

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
	componentValues  *ComponentTemplateValues
	buildinFunctions *intctrlutil.BuiltinObjectsFunc

	// DBaas cluster object
	component  *Component
	appVersion *dbaasv1alpha1.AppVersion
	cluster    *dbaasv1alpha1.Cluster
	podSpec    *corev1.PodSpec
	roleGroup  *RoleGroup
}

func NewCfgTemplateBuilder(clusterName, namespace string, cluster *dbaasv1alpha1.Cluster, version *dbaasv1alpha1.AppVersion) *ConfigTemplateBuilder {
	return &ConfigTemplateBuilder{
		namespace:   namespace,
		clusterName: clusterName,
		cluster:     cluster,
		appVersion:  version,
	}
}

func (c *ConfigTemplateBuilder) Render(configs map[string]string) (map[string]string, error) {
	rendered := make(map[string]string, len(configs))
	engine := intctrlutil.NewTplEngine(c.builtinObjects(), c.buildinFunctions)
	for file, configContext := range configs {
		newContext, err := engine.Render(configContext)
		if err != nil {
			return nil, err
		}
		rendered[file] = newContext
	}

	return rendered, nil
}

// General Built-in objects
const (
	BuiltinClusterObject           = "Cluster"
	BuiltinComponentObject         = "Component"
	BuiltinPodObject               = "PodSpec"
	BuiltinRoleObject              = "RoleGroup"
	BuiltinAppVersionObject        = "Version"
	BuiltinComponentResourceObject = "ComponentResource"
)

func (c *ConfigTemplateBuilder) builtinObjects() *intctrlutil.TplValues {
	return &intctrlutil.TplValues{
		BuiltinClusterObject:           c.cluster,
		BuiltinComponentObject:         c.component,
		BuiltinPodObject:               c.podSpec,
		BuiltinRoleObject:              c.roleGroup,
		BuiltinComponentResourceObject: c.componentValues.Resource,
		BuiltinAppVersionObject:        c.appVersion,
	}
}

func (c *ConfigTemplateBuilder) InjectBuiltInObjectsAndFunctions(podTemplate *corev1.PodTemplateSpec, configs []dbaasv1alpha1.ConfigTemplate, component *Component, group *RoleGroup) error {
	if err := injectBuiltInObjects(c, podTemplate, component, group, configs); err != nil {
		return err
	}

	if err := injectBuiltInFunctions(c, podTemplate, component, group); err != nil {
		return err
	}
	return nil
}

// General Built-in functions
const (
	BuiltinGetVolumeFunctionName    = "getVolumePathByName"
	BuiltinGetPvcFunctionName       = "getPvcByName"
	BuiltinGetEnvFunctionName       = "getEnvByName"
	BuiltinGetArgFunctionName       = "getArgByName"
	BuiltinGetPortFunctionName      = "getPortByName"
	BuiltinGetContainerFunctionName = "getContainerByName"

	// BuiltinMysqlCalBufferFunctionName Mysql Built-in
	// TODO: This function migrate to configuration template
	BuiltinMysqlCalBufferFunctionName = "callBufferSizeByResource"
)

func injectBuiltInFunctions(tplBuilder *ConfigTemplateBuilder, podTemplate *corev1.PodTemplateSpec, component *Component, group *RoleGroup) error {
	// TODO add built-in function
	tplBuilder.buildinFunctions = &intctrlutil.BuiltinObjectsFunc{
		BuiltinMysqlCalBufferFunctionName: calDbPoolSize,
		BuiltinGetVolumeFunctionName:      getVolumeMountPathByName,
		BuiltinGetPvcFunctionName:         getPvcByName,
		BuiltinGetEnvFunctionName:         getEnvByName,
		BuiltinGetPortFunctionName:        getPortByName,
		BuiltinGetArgFunctionName:         getArgByName,
		BuiltinGetContainerFunctionName:   getPodContainerByName,
	}
	return nil
}

func injectBuiltInObjects(tplBuilder *ConfigTemplateBuilder, podTemplate *corev1.PodTemplateSpec, component *Component, group *RoleGroup, configs []dbaasv1alpha1.ConfigTemplate) error {
	var resource *ResourceDefinition
	container := intctrlutil.GetContainerUsingConfig(*podTemplate, configs)
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

	tplBuilder.podSpec = &podTemplate.Spec
	tplBuilder.component = component
	tplBuilder.roleGroup = group

	return nil
}
