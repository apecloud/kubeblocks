/*
Copyright ApeCloud Inc.

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
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

// General Built-in objects
const (
	builtinClusterObject           = "cluster"
	builtinComponentObject         = "component"
	builtinPodObject               = "podSpec"
	builtinClusterVersionObject    = "version"
	builtinComponentResourceObject = "componentResource"
)

// General Built-in functions
const (
	builtInGetVolumeFunctionName          = "getVolumePathByName"
	builtInGetPvcFunctionName             = "getPVCByName"
	builtInGetEnvFunctionName             = "getEnvByName"
	builtInGetArgFunctionName             = "getArgByName"
	builtInGetPortFunctionName            = "getPortByName"
	builtInGetContainerFunctionName       = "getContainerByName"
	builtInGetContainerMemoryFunctionName = "getContainerMemory"

	// BuiltinMysqlCalBufferFunctionName Mysql Built-in
	// TODO: This function migrate to configuration template
	builtInMysqlCalBufferFunctionName = "callBufferSizeByResource"
)

func newCfgTemplateBuilder(
	clusterName, namespace string,
	cluster *dbaasv1alpha1.Cluster,
	version *dbaasv1alpha1.ClusterVersion,
	ctx context.Context,
	cli client.Client) *configTemplateBuilder {
	return &configTemplateBuilder{
		namespace:      namespace,
		clusterName:    clusterName,
		cluster:        cluster,
		clusterVersion: version,
		tplName:        "KBTPL",
		ctx:            ctx,
		cli:            cli,
	}
}

func (c *configTemplateBuilder) setTplName(tplName string) {
	c.tplName = tplName
}

func (c *configTemplateBuilder) formatError(file string, err error) error {
	return fmt.Errorf("failed to render configuration template[cm:%s][key:%s], error: [%v]", c.tplName, file, err)
}

func (c *configTemplateBuilder) render(configs map[string]string) (map[string]string, error) {
	rendered := make(map[string]string, len(configs))
	o, err := c.builtinObjects()
	if err != nil {
		return nil, err
	}
	engine := gotemplate.NewTplEngine(o, c.builtInFunctions, c.tplName, c.cli, c.ctx)
	for file, configContext := range configs {
		newContext, err := engine.Render(configContext)
		if err != nil {
			return nil, c.formatError(file, err)
		}
		rendered[file] = newContext
	}
	return rendered, nil
}

func (c *configTemplateBuilder) builtinObjects() (*gotemplate.TplValues, error) {
	bultInObj := map[string]interface{}{
		builtinClusterObject:           c.cluster,
		builtinComponentObject:         c.component,
		builtinPodObject:               c.podSpec,
		builtinComponentResourceObject: c.componentValues.Resource,
		builtinClusterVersionObject:    c.clusterVersion,
	}
	b, err := json.Marshal(bultInObj)
	if err != nil {
		return nil, err
	}
	var tplValue gotemplate.TplValues
	if err = json.Unmarshal(b, &tplValue); err != nil {
		return nil, err
	}
	return &tplValue, nil
}

func (c *configTemplateBuilder) injectBuiltInObjectsAndFunctions(
	podSpec *corev1.PodSpec,
	configs []dbaasv1alpha1.ConfigTemplate,
	component *Component) error {
	if err := injectBuiltInObjects(c, podSpec, component, configs); err != nil {
		return err
	}
	if err := injectBuiltInFunctions(c, component); err != nil {
		return err
	}
	return nil
}

func injectBuiltInFunctions(tplBuilder *configTemplateBuilder, component *Component) error {
	// TODO add built-in function
	tplBuilder.builtInFunctions = &gotemplate.BuiltInObjectsFunc{
		builtInMysqlCalBufferFunctionName:     calDBPoolSize,
		builtInGetVolumeFunctionName:          getVolumeMountPathByName,
		builtInGetPvcFunctionName:             getPVCByName,
		builtInGetEnvFunctionName:             getEnvByName,
		builtInGetPortFunctionName:            getPortByName,
		builtInGetArgFunctionName:             getArgByName,
		builtInGetContainerFunctionName:       getPodContainerByName,
		builtInGetContainerMemoryFunctionName: getContainerMemory,
	}
	return nil
}

func injectBuiltInObjects(tplBuilder *configTemplateBuilder, podSpec *corev1.PodSpec, component *Component, configs []dbaasv1alpha1.ConfigTemplate) error {
	var resource *ResourceDefinition
	container := intctrlutil.GetContainerByConfigTemplate(podSpec, configs)
	if container != nil && len(container.Resources.Limits) > 0 {
		resource = &ResourceDefinition{
			MemorySize: intctrlutil.GetMemorySize(*container),
			CoreNum:    intctrlutil.GetCoreNum(*container),
		}
	}
	tplBuilder.componentValues = &componentTemplateValues{
		TypeName: component.Type,
		// TODO add Component service name
		ServiceName: "",
		Replicas:    component.Replicas,
		Resource:    resource,
	}
	tplBuilder.podSpec = podSpec
	tplBuilder.component = component
	return nil
}
