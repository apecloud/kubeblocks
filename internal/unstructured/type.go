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

type ConfigObject interface {
	// Update sets the value for the key in ConfigObject
	Update(key string, value any) error

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
