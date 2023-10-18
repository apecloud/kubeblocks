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
	"bytes"

	"github.com/magiconair/properties"
	"github.com/spf13/cast"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type propertiesConfig struct {
	name       string
	Properties *properties.Properties
}

const commentPrefix = "# "

func init() {
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.PropertiesPlus, func(name string) ConfigObject {
		return &propertiesConfig{name: name}
	})
}

func (p *propertiesConfig) Update(key string, value any) error {
	_, _, err := p.Properties.Set(key, cast.ToString(value))
	return err
}

func (p *propertiesConfig) RemoveKey(key string) error {
	p.Properties.Delete(key)
	return nil
}

func (p *propertiesConfig) Get(key string) interface{} {
	val, ok := p.Properties.Get(key)
	if ok {
		return val
	}
	return nil
}

func (p *propertiesConfig) GetString(key string) (string, error) {
	if val := p.Get(key); val != nil {
		return val.(string), nil
	}
	return "", nil
}

func (p *propertiesConfig) GetAllParameters() map[string]interface{} {
	r := make(map[string]interface{}, len(p.Properties.Keys()))
	for _, key := range p.Properties.Keys() {
		r[key] = p.Get(key)
	}
	return r
}

func (p *propertiesConfig) SubConfig(key string) ConfigObject {
	return nil
}

func (p *propertiesConfig) Marshal() (string, error) {
	if p.Properties.Len() == 0 {
		return "", nil
	}

	var buf bytes.Buffer
	_, err := p.Properties.WriteComment(&buf, commentPrefix, properties.UTF8)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (p *propertiesConfig) Unmarshal(str string) (err error) {
	l := &properties.Loader{
		Encoding:         properties.UTF8,
		DisableExpansion: true,
	}
	p.Properties, err = l.LoadBytes([]byte(str))
	return err
}
