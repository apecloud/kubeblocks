/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
