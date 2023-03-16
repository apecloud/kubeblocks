/*
Copyright ApeCloud, Inc.

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

package plan

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
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

	// TLS Built-in
	builtInGetCAFile   = "getCAFile"
	builtInGetCertFile = "getCertFile"
	builtInGetKeyFile  = "getKeyFile"
)

type ResourceDefinition struct {
	MemorySize int64 `json:"memorySize,omitempty"`
	CoreNum    int64 `json:"coreNum,omitempty"`
}

type componentTemplateValues struct {
	TypeName    string
	ServiceName string
	Replicas    int32

	// Container *corev1.Container
	Resource  *ResourceDefinition
	ConfigTpl []appsv1alpha1.ComponentConfigSpec
}

type configTemplateBuilder struct {
	namespace   string
	clusterName string
	tplName     string

	// Global Var
	componentValues  *componentTemplateValues
	builtInFunctions *gotemplate.BuiltInObjectsFunc

	// cluster object
	component      *component.SynthesizedComponent
	clusterVersion *appsv1alpha1.ClusterVersion
	cluster        *appsv1alpha1.Cluster
	podSpec        *corev1.PodSpec

	ctx context.Context
	cli client.Client
}

func newCfgTemplateBuilder(
	clusterName, namespace string,
	cluster *appsv1alpha1.Cluster,
	version *appsv1alpha1.ClusterVersion,
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
	values, err := c.builtinObjectsAsValues()
	if err != nil {
		return nil, err
	}
	engine := gotemplate.NewTplEngine(values, c.builtInFunctions, c.tplName, c.cli, c.ctx)
	for file, configContext := range configs {
		newContext, err := engine.Render(configContext)
		if err != nil {
			return nil, c.formatError(file, err)
		}
		rendered[file] = newContext
	}
	return rendered, nil
}

func (c *configTemplateBuilder) builtinObjectsAsValues() (*gotemplate.TplValues, error) {
	builtInObjs := map[string]interface{}{
		builtinClusterObject:           c.cluster,
		builtinComponentObject:         c.component,
		builtinPodObject:               c.podSpec,
		builtinComponentResourceObject: c.componentValues.Resource,
		builtinClusterVersionObject:    c.clusterVersion,
	}
	b, err := json.Marshal(builtInObjs)
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
	configs []appsv1alpha1.ComponentConfigSpec,
	component *component.SynthesizedComponent) error {
	if err := c.injectBuiltInObjects(podSpec, component, configs); err != nil {
		return err
	}
	if err := c.injectBuiltInFunctions(component); err != nil {
		return err
	}
	return nil
}

func (c *configTemplateBuilder) injectBuiltInFunctions(component *component.SynthesizedComponent) error {
	// TODO add built-in function
	c.builtInFunctions = &gotemplate.BuiltInObjectsFunc{
		builtInMysqlCalBufferFunctionName:     calDBPoolSize,
		builtInGetVolumeFunctionName:          getVolumeMountPathByName,
		builtInGetPvcFunctionName:             getPVCByName,
		builtInGetEnvFunctionName:             getEnvByName,
		builtInGetPortFunctionName:            getPortByName,
		builtInGetArgFunctionName:             getArgByName,
		builtInGetContainerFunctionName:       getPodContainerByName,
		builtInGetContainerMemoryFunctionName: getContainerMemory,
		builtInGetCAFile:                      getCAFile,
		builtInGetCertFile:                    getCertFile,
		builtInGetKeyFile:                     getKeyFile,
	}
	return nil
}

func (c *configTemplateBuilder) injectBuiltInObjects(podSpec *corev1.PodSpec, component *component.SynthesizedComponent, configs []appsv1alpha1.ComponentConfigSpec) error {
	var resource *ResourceDefinition
	container := intctrlutil.GetContainerByConfigTemplate(podSpec, configs)
	if container != nil && len(container.Resources.Limits) > 0 {
		resource = &ResourceDefinition{
			MemorySize: intctrlutil.GetMemorySize(*container),
			CoreNum:    intctrlutil.GetCoreNum(*container),
		}
	}
	c.componentValues = &componentTemplateValues{
		TypeName:  component.Type,
		Replicas:  component.Replicas,
		Resource:  resource,
		ConfigTpl: configs,
	}
	c.podSpec = podSpec
	c.component = component
	return nil
}
