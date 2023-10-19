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

type ConfigObject interface {
	// Update sets the value for the key in ConfigObject
	Update(key string, value any) error

	// RemoveKey configuration parameter
	RemoveKey(key string) error

	// Get returns an interface.
	Get(key string) interface{}

	// GetString returns the value associated with the key as a string
	GetString(key string) (string, error)

	// GetAllParameters returns all config params as a map[string]interface{}
	GetAllParameters() map[string]interface{}

	// SubConfig returns new Sub ConfigObject instance.
	SubConfig(key string) ConfigObject

	// Marshal outputs the ConfigObject to string
	Marshal() (string, error)

	// Unmarshal reads a string and returns the valid key/value pair of valid variables.
	Unmarshal(str string) error
}

const (
	// DelimiterDot sets the delimiter used for determining key parts.
	DelimiterDot = "."

	// CfgDelimiterPlaceholder sets the delimiter used for determining key parts.
	//
	// In order to verify a configuration file, the configuration file is converted to a UnstructuredObject.
	// When there is a special character '.' in the parameter will cause the parameter of the configuration file parsing to be messed up.
	//   e.g. pg parameters: auto_explain.log_analyze = 'True'
	// To solve this problem, the CfgDelimiterPlaceholder variable is introduced to ensure that no such string exists in a configuration file.
	CfgDelimiterPlaceholder = "@#@"
)
