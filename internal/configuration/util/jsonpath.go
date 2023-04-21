/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
