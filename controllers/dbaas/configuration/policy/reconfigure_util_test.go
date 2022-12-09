/*
Copyright ApeCloud Inc.

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
	"testing"

	"github.com/stretchr/testify/require"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
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
	params, err := extractUpdatedParams(testData)
	require.Nil(t, err)
	require.Equal(t, cfgcore.NewSetFromList(
		[]string{
			"a", "c_1", "c_0", "msld", "cd", "f", "test1", "test2",
		}),
		cfgcore.NewSetFromList(params))
}

func extractUpdatedParams(testData string) ([]string, error) {
	cfg := cfgcore.ConfigDiffInformation{
		UpdateConfig: map[string][]byte{
			"k": []byte(testData),
		},
	}

	return getUpdateParameterList(&cfg)
}
