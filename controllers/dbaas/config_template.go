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
	"fmt"
	"math"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	corev1 "k8s.io/api/core/v1"
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
}

func NewCfgTemplateBuilder(clusterName, namespace string) *ConfigTemplateBuilder {
	return &ConfigTemplateBuilder{
		namespace:   namespace,
		clusterName: clusterName,
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
	tplBuilder.buildinFunctions = &intctrlutil.BuiltinObjectsFunc{
		"mysql_buffer_size_cal": calMysqlPoolSizeByResource,
	}
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

// calReverseRebaseBuffer Cal reserved memory for system
func calReverseRebaseBuffer(memSizeMB int64, cpuNum int) int64 {
	const (
		RebaseMemorySize        = int64(2048)
		ReverseRebaseBufferSize = 285
	)

	// MIN(RDS ins class for mem / 2, 2048)
	r1 := int64(math.Min(float64(memSizeMB>>1), float64(RebaseMemorySize)))
	// MAX(RDS ins class for CPU * 64, RDS ins class for mem / 64)
	r2 := int64(math.Max(float64(cpuNum<<6), float64(memSizeMB>>6)))

	return r1 + r2 + memSizeMB>>6 + ReverseRebaseBufferSize
}

// https://help.aliyun.com/document_detail/162326.html?utm_content=g_1000230851&spm=5176.20966629.toubu.3.f2991ddcpxxvD1#title-rey-j7j-4dt
// build-in function
// calMysqlPoolSizeByResource Cal mysql buffer size
func calMysqlPoolSizeByResource(resource *ResourceDefinition, isShared bool) string {
	const (
		DefaultPoolSize      = "128M"
		MinBufferSizeMB      = 128
		SmallClassMemorySize = int64(1024 * 1024 * 1024)
	)

	if resource == nil || resource.CoreNum == 0 || resource.MemorySize == 0 {
		return DefaultPoolSize
	}

	// small instance class
	// mem_size <= 1G or
	// core <= 2
	if resource.MemorySize <= SmallClassMemorySize {
		return DefaultPoolSize
	}

	memSizeMB := resource.MemorySize / 1024 / 1024
	maxBufferSize := int32(memSizeMB * 80 / 100)
	totalMemorySize := memSizeMB

	if !isShared {
		reverseBuffer := calReverseRebaseBuffer(memSizeMB, resource.CoreNum)
		totalMemorySize = memSizeMB - reverseBuffer

		// for small instance class
		if resource.CoreNum <= 2 {
			totalMemorySize -= 128
		}
	}

	if totalMemorySize <= MinBufferSizeMB {
		return DefaultPoolSize
	}

	// (total_memory - reverseBuffer) * 75
	bufferSize := int32(totalMemorySize * 75 / 100)
	if bufferSize > maxBufferSize {
		bufferSize = maxBufferSize
	}

	// https://dev.mysql.com/doc/refman/8.0/en/innodb-parameters.html#sysvar_innodb_buffer_pool_size
	// Buffer size require aligned 128MB or 1G
	var alignedSize int32 = 128
	if bufferSize > 1024 {
		alignedSize = 1024
	}

	bufferSize /= alignedSize
	bufferSize *= alignedSize
	return fmt.Sprintf("%dM", bufferSize)
}
