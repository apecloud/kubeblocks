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

func TestXMLFormat(t *testing.T) {
	const xmlContext = `
<!-- Settings profiles -->
<profiles>
    <!-- Default settings -->
    <default>
        <!-- The maximum number of threads when running a single query. -->
        <max_threads>8</max_threads>
    </default>

    <!-- Settings for quries from the user interface -->
    <web>
        <max_execution_time>600</max_execution_time>
        <min_execution_speed>1000000</min_execution_speed>
        <timeout_before_checking_execution_speed>15</timeout_before_checking_execution_speed>

        <readonly>1</readonly>
    </web>
</profiles>
`
	xmlConfigObj, err := LoadConfig("xml_test", xmlContext, appsv1alpha1.XML)
	assert.Nil(t, err)

	assert.EqualValues(t, xmlConfigObj.Get("profiles.default.max_threads"), 8)
	assert.EqualValues(t, xmlConfigObj.Get("profiles.web.min_execution_speed"), 1000000)
	assert.EqualValues(t, xmlConfigObj.Get("profiles.web.max_threads"), nil)

	v, _ := xmlConfigObj.GetString("profiles.default.max_threads")
	assert.EqualValues(t, v, "8")
	v, _ = xmlConfigObj.GetString("profiles.web.min_execution_speed")
	assert.EqualValues(t, v, "1000000")

	_, err = xmlConfigObj.GetString("profiles.web.xxxx")
	assert.Nil(t, err)

	dumpContext, err := xmlConfigObj.Marshal()
	assert.Nil(t, err)
	newObj, err := LoadConfig("xml_test", dumpContext, appsv1alpha1.XML)
	assert.Nil(t, err)
	assert.EqualValues(t, newObj.GetAllParameters(), xmlConfigObj.GetAllParameters())

	assert.Nil(t, xmlConfigObj.Update("profiles.web.timeout_before_checking_execution_speed", 200))
	assert.EqualValues(t, xmlConfigObj.Get("profiles.web.timeout_before_checking_execution_speed"), 200)
}
