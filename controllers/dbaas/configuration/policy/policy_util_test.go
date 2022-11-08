/*
Copyright 2022.

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

package policy

import (
	"encoding/json"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetUpdateParameterList(t *testing.T) {

	testData := `
{
	"a": "b",
	"f": 10.2,
	"c": [
		"edcl",
		"cde"
	],
	"d" : [],
	"n" : [{}],
	"xxx" : [
		{
			"test1": 2,
			"test2": 5
		}
	],
	"g": {
		"cd" : "abcd",
		"msld" : "cakl"
	}
}
`
	var obj any
	err := json.Unmarshal([]byte(testData), &obj)
	require.Nil(t, err)

	params := extractUpdatedParams(obj)
	require.Equal(t, cfgcore.NewSetFromList(
		[]string{
			"a", "c", "msld", "cd", "f", "test1", "test2", "d",
		}),
		cfgcore.NewSetFromList(params))
}
