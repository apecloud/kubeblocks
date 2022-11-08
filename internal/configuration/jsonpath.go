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

package configuration

import (
	"encoding/json"

	"github.com/bhmj/jsonslice"
	jsonpatch "github.com/evanphx/json-patch"
)

func RetrievalWithJsonPath(jsonobj interface{}, jsonpath string) ([]byte, error) {

	jsonbytes, err := json.Marshal(&jsonobj)
	if err != nil {
		return nil, err
	}

	res, err := jsonslice.Get(jsonbytes, jsonpath)
	if err != nil {
		return res, err
	}

	reslen := len(res)
	if reslen > 2 && res[0] == '"' && res[reslen-1] == '"' {
		res = res[1 : reslen-1]
	}

	return res, err
}

func jsonPatch(originalJSON, modifiedJSON interface{}) ([]byte, error) {
	originalBytes, err := json.Marshal(originalJSON)
	if err != nil {
		return nil, err
	}

	modifiedBytes, err := json.Marshal(modifiedJSON)
	if err != nil {
		return nil, err
	}

	// TODO(zt) It's a hack to do the logic, json object --> bytes, bytes --> json object
	return jsonpatch.CreateMergePatch(originalBytes, modifiedBytes)
}
