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

package util

import (
	"encoding/json"

	"github.com/bhmj/jsonslice"
	jsonpatch "github.com/evanphx/json-patch"
)

func RetrievalWithJSONPath(jsonObj interface{}, jsonpath string) ([]byte, error) {
	jsonBytes, err := json.Marshal(&jsonObj)
	if err != nil {
		return nil, err
	}

	res, err := jsonslice.Get(jsonBytes, jsonpath)
	if err != nil {
		return res, err
	}

	resLen := len(res)
	if resLen > 2 && res[0] == '"' && res[resLen-1] == '"' {
		res = res[1 : resLen-1]
	}
	return res, err
}

func JSONPatch(originalJSON, modifiedJSON interface{}) ([]byte, error) {
	originalBytes, err := json.Marshal(originalJSON)
	if err != nil {
		return nil, err
	}

	modifiedBytes, err := json.Marshal(modifiedJSON)
	if err != nil {
		return nil, err
	}
	return jsonpatch.CreateMergePatch(originalBytes, modifiedBytes)
}
