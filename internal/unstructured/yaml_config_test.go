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
}
