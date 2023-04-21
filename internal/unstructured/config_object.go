/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
