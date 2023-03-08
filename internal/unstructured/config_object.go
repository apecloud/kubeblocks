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

package unstructured

import (
	"fmt"
	"sync"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type ConfigObjectCreator = func(name string) ConfigObject

type ConfigObjectRegistry struct {
	objectCreator map[appsv1alpha1.CfgFileFormat]ConfigObjectCreator
}

var (
	ConfigRegistryOnce   sync.Once
	configObjectRegistry *ConfigObjectRegistry
)

func CfgObjectRegistry() *ConfigObjectRegistry {
	ConfigRegistryOnce.Do(func() {
		configObjectRegistry = &ConfigObjectRegistry{objectCreator: make(map[appsv1alpha1.CfgFileFormat]ConfigObjectCreator)}
	})
	return configObjectRegistry
}

func (c *ConfigObjectRegistry) RegisterConfigCreator(format appsv1alpha1.CfgFileFormat, creator ConfigObjectCreator) {
	c.objectCreator[format] = creator
}

func (c *ConfigObjectRegistry) GetConfigObject(name string, format appsv1alpha1.CfgFileFormat) (ConfigObject, error) {
	creator, ok := c.objectCreator[format]
	if !ok {
		return nil, fmt.Errorf("not support type[%s]", format)
	}
	return creator(name), nil
}

func LoadConfig(name string, content string, format appsv1alpha1.CfgFileFormat) (ConfigObject, error) {
	configObject, err := CfgObjectRegistry().GetConfigObject(name, format)
	if err != nil {
		return nil, err
	}
	if err := configObject.Unmarshal(content); err != nil {
		return nil, err
	}
	return configObject, nil
}
