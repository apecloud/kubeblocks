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
	"strings"

	"github.com/spf13/cast"
	"gopkg.in/yaml.v2"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type yamlConfig struct {
	name   string
	config map[string]any
}

func init() {
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.YAML, func(name string) ConfigObject {
		return &yamlConfig{name: name}
	})
}

func (y *yamlConfig) Update(key string, value any) error {
	path := strings.Split(key, ".")
	lastKey := path[len(path)-1]
	deepestMap := checkAndCreateNestedPrefixMap(y.config, path[0:len(path)-1])
	deepestMap[lastKey] = value
	return nil
}

func (y *yamlConfig) Get(key string) any {
	keys := strings.Split(key, ".")
	return searchMap(y.config, keys)
}

func (y *yamlConfig) GetString(key string) (string, error) {
	v := y.Get(key)
	if v != nil {
		return cast.ToStringE(v)
	}
	return "", nil
}

func (y *yamlConfig) GetAllParameters() map[string]any {
	return y.config
}

func (y *yamlConfig) SubConfig(key string) ConfigObject {
	v := y.Get(key)
	if m, ok := v.(map[string]any); ok {
		return &yamlConfig{
			name:   y.name,
			config: m,
		}
	}
	return nil
}

func (y *yamlConfig) Marshal() (string, error) {
	b, err := yaml.Marshal(y.config)
	return string(b), err
}

func (y *yamlConfig) Unmarshal(str string) error {
	config := make(map[any]any)
	err := yaml.Unmarshal([]byte(str), config)
	if err != nil {
		return err
	}
	y.config = transKeyString(config)
	return nil
}

func checkAndCreateNestedPrefixMap(m map[string]any, path []string) map[string]any {
	for _, k := range path {
		m2, ok := m[k]
		// if the key is not exist, create a new map
		if !ok {
			m3 := make(map[string]any)
			m[k] = m3
			m = m3
			continue
		}
		m3, ok := m2.(map[string]any)
		// if the type is not map, replace with a new map
		if !ok {
			m3 = make(map[string]any)
			m[k] = m3
		}
		m = m3
	}
	return m
}

func searchMap(m map[string]any, path []string) any {
	if len(path) == 0 {
		return m
	}

	next, ok := m[path[0]]
	if !ok {
		return nil
	}
	if len(path) == 1 {
		return next
	}
	switch t := next.(type) {
	default:
		return nil
	case map[any]any:
		return searchMap(cast.ToStringMap(t), path[1:])
	case map[string]any:
		return searchMap(t, path[1:])
	}
}

func transKeyString(m map[any]any) map[string]any {
	m2 := make(map[string]any, len(m))
	for k, v := range m {
		if vi, ok := v.(map[any]any); ok {
			m2[cast.ToString(k)] = transKeyString(vi)
		} else {
			m2[cast.ToString(k)] = v
		}
	}
	return m2
}
