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
