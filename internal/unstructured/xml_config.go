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

	mxjv2 "github.com/clbanning/mxj/v2"
	"github.com/spf13/cast"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type xmlConfig struct {
	name string
	data mxjv2.Map
}

func init() {
	// disable cast to float
	mxjv2.CastValuesToFloat(false)
	// enable cast to bool
	mxjv2.CastValuesToBool(true)
	// enable cast to int
	mxjv2.CastValuesToInt(true)

	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.XML, func(name string) ConfigObject {
		return &xmlConfig{name: name}
	})
}

func (x *xmlConfig) Update(key string, value any) error {
	return x.data.SetValueForPath(value, key)
}

func (x *xmlConfig) Get(key string) interface{} {
	keys := strings.Split(key, ".")
	keysLen := len(keys)
	m := prefixKeys(x.data.Old(), keys[:keysLen-1])
	if m != nil {
		return m[keys[keysLen-1]]
	}
	return nil
}

func prefixKeys(m map[string]interface{}, keys []string) map[string]interface{} {
	r := m
	for _, k := range keys {
		if m == nil {
			break
		}
		v, ok := r[k]
		if !ok {
			return nil
		}

		switch vt := v.(type) {
		default:
			r = nil
		case map[string]interface{}:
			r = vt
		}
	}
	return r
}

func (x *xmlConfig) GetString(key string) (string, error) {
	v := x.Get(key)
	if v != nil {
		return cast.ToStringE(v)
	}
	return "", nil
}

func (x *xmlConfig) GetAllParameters() map[string]interface{} {
	return x.data
}

func (x *xmlConfig) SubConfig(key string) ConfigObject {
	v := x.Get(key)
	if v == nil {
		return nil
	}

	switch t := v.(type) {
	case map[string]interface{}:
		return &xmlConfig{data: t}
	default:
		return nil
	}
}

func (x *xmlConfig) Marshal() (string, error) {
	b, err := x.data.Xml()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (x *xmlConfig) Unmarshal(str string) error {
	m, err := mxjv2.NewMapXml([]byte(str), true)
	if err != nil {
		return err
	}
	x.data = m
	return nil
}
