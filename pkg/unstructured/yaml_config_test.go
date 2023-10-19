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
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/test/testdata"
)

func TestYAMLFormat(t *testing.T) {
	const yamlContext = `
spec:
  clusterRef: pg
  reconfigure:
    componentName: postgresql
    configurations:
    - keys:
      - key: postgresql.conf
        parameters:
        - key: max_connections
          value: "2666"
      name: postgresql-configuration
`

	yamlConfigObj, err := LoadConfig("yaml_test", yamlContext, appsv1alpha1.YAML)
	assert.Nil(t, err)

	assert.EqualValues(t, yamlConfigObj.Get("spec.clusterRef"), "pg")
	assert.EqualValues(t, yamlConfigObj.Get("spec.reconfigure.componentName"), "postgresql")

	dumpContext, err := yamlConfigObj.Marshal()
	assert.Nil(t, err)
	assert.EqualValues(t, dumpContext, yamlContext[1:]) // trim "\n"

	assert.Nil(t, yamlConfigObj.Update("spec.my_test", "100"))
	assert.EqualValues(t, yamlConfigObj.Get("spec.my_test"), "100")
	assert.Nil(t, yamlConfigObj.RemoveKey("spec.my_test.xxx"))
	assert.Nil(t, yamlConfigObj.RemoveKey("spec.my_test"))
	assert.EqualValues(t, yamlConfigObj.Get("spec.my_test"), nil)
}

func TestYAMLFormatForBadCase(t *testing.T) {
	b, err := testdata.GetTestDataFileContent("config_encoding/prometheus.yaml")
	assert.Nil(t, err)

	yamlConfigObj, err := LoadConfig("yaml_test", string(b), appsv1alpha1.YAML)
	assert.Nil(t, err)
	assert.NotNil(t, yamlConfigObj)
	yamlConfigObj.GetAllParameters()
	_, err = util.JSONPatch(nil, yamlConfigObj.GetAllParameters())
	assert.Nil(t, err)
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name string
		args any
		want any
	}{{
		name: "test",
		args: 10,
		want: 10,
	}, {
		name: "test",
		args: map[string]interface{}{
			"key":   "value",
			"test2": 100,
		},
		want: map[string]interface{}{
			"key":   "value",
			"test2": 100,
		},
	}, {
		name: "test",
		args: []interface{}{
			"key",
			"value",
		},
		want: []interface{}{
			"key",
			"value",
		},
	}, {
		name: "test",
		args: map[interface{}]interface{}{
			1: "value",
			2: 200,
		},
		want: map[string]interface{}{
			"1": "value",
			"2": 200,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, convert(tt.args), "convert(%v)", tt.args)
		})
	}
}
