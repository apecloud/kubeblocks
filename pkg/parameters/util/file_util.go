/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"os"

	oyaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func FromYamlConfig[T any](yamlConfig string, obj T) error {
	if _, err := os.Stat(yamlConfig); err != nil {
		return err
	}
	b, err := os.ReadFile(yamlConfig)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, obj)
}

func ToYamlConfig(obj interface{}) ([]byte, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var jsonObj interface{}
	if err := yaml.Unmarshal(b, &jsonObj); err != nil {
		return nil, err
	}
	return oyaml.Marshal(jsonObj)
}
