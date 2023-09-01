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

func TestProperitesFormat(t *testing.T) {
	const propsContext = `
# Time of inactivity after which the broker will discard the deduplication information
# relative to a disconnected producer. Default is 6 hours.
brokerDeduplicationProducerInactivityTimeoutMinutes=360

# When a namespace is created without specifying the number of bundle, this
# value will be used as the default
defaultNumberOfNamespaceBundles=4

# The maximum number of namespaces that each tenant can create
# This configuration is not precise control, in a concurrent scenario, the threshold will be exceeded
maxNamespacesPerTenant.brokerMaxConnections.threadMethod=660

# Max number of topics allowed to be created in the namespace. When the topics reach the max topics of the namespace,
# the broker should reject the new topic request(include topic auto-created by the producer or consumer)
# until the number of connected consumers decrease.
# Using a value of 0, is disabling maxTopicsPerNamespace-limit check.
maxTopicsPerNamespace=0

# The maximum number of connections in the broker. If it exceeds, new connections are rejected.
brokerMaxConnections=0

# The maximum number of connections per IP. If it exceeds, new connections are rejected.
brokerMaxConnectionsPerIp=0
`
	propsConfigObj, err := LoadConfig("props_test", propsContext, appsv1alpha1.PropertiesPlus)
	assert.Nil(t, err)

	assert.EqualValues(t, propsConfigObj.Get("brokerDeduplicationProducerInactivityTimeoutMinutes"), "360")
	assert.EqualValues(t, propsConfigObj.Get("maxNamespacesPerTenant.brokerMaxConnections.threadMethod"), "660")

	v, _ := propsConfigObj.GetString("defaultNumberOfNamespaceBundles")
	assert.EqualValues(t, v, "4")
	v, _ = propsConfigObj.GetString("brokerMaxConnectionsPerIp")
	assert.EqualValues(t, v, "0")

	v, err = propsConfigObj.GetString("profiles.web.xxxx")
	assert.Nil(t, err)
	assert.EqualValues(t, v, "")

	dumpContext, err := propsConfigObj.Marshal()
	assert.Nil(t, err)
	newObj, err := LoadConfig("props_test", dumpContext, appsv1alpha1.PropertiesPlus)
	assert.Nil(t, err)
	assert.EqualValues(t, newObj.GetAllParameters(), propsConfigObj.GetAllParameters())

	assert.EqualValues(t, newObj.SubConfig("test"), nil)

	assert.Nil(t, propsConfigObj.Update("profiles.web.timeout_before_checking_execution_speed", 200))
	assert.EqualValues(t, propsConfigObj.Get("profiles.web.timeout_before_checking_execution_speed"), "200")

	assert.Nil(t, propsConfigObj.Update("defaultNumberOfNamespaceBundles", "600"))
	assert.EqualValues(t, propsConfigObj.Get("defaultNumberOfNamespaceBundles"), "600")
}

func TestProperitesEmpty(t *testing.T) {
	propsConfigObj, err := LoadConfig("props_test", "", appsv1alpha1.PropertiesPlus)
	assert.Nil(t, err)

	v, err := propsConfigObj.Marshal()
	assert.Nil(t, err)
	assert.EqualValues(t, v, "")

	assert.Nil(t, propsConfigObj.Update("emptyField", ""))
	assert.EqualValues(t, propsConfigObj.Get("emptyField"), "")

	assert.Nil(t, propsConfigObj.Update("emptyField2", "\"\""))
	assert.EqualValues(t, propsConfigObj.Get("emptyField2"), "\"\"")

	dumpContext, err := propsConfigObj.Marshal()
	assert.Nil(t, err)
	assert.EqualValues(t, "emptyField = \nemptyField2 = \"\"\n", dumpContext)
}
