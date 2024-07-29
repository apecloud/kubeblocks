/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/plugin"
)

// WrapperArgs Used to convert map[string]interface{} to parameter
func WrapperArgs(args map[string]interface{}) ([]*plugin.Parameter, error) {
	var parameters []*plugin.Parameter
	for key, value := range args {
		param, err := CreateParameter(key, value)
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, param)
	}
	return parameters, nil
}

// ParseArgs Used to parse args to map[string]string
func ParseArgs(parameters []*plugin.Parameter) (map[string]string, error) {
	m := map[string]string{}
	for _, param := range parameters {
		m[param.GetKey()] = param.GetValue()
	}
	return m, nil
}

// CreateParameter Used to create parameter
func CreateParameter(key string, value interface{}) (*plugin.Parameter, error) {
	param := &plugin.Parameter{Key: key}
	switch v := value.(type) {
	case string:
		param.Value = v
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}
	return param, nil
}
