/*
Copyright 2022 The KubeBlocks Authors

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

package controllerutil

// ClusterDefinition Type Const Define
const (
	kStateMysql  = "state.mysql"
	kStateMysql8 = "state.mysql-8"
)

// ClusterDefinitionComponent TypeName Const Define
const (
	kWesql = "wesql"
)

// ClusterDefinitionComponent CharacterType Const Define
const (
	kMysql = "mysql"
	kEmpty = ""
)

var (
	kWellKnownTypeMaps = map[string]map[string]string{
		kStateMysql: {
			kWesql: kMysql,
		},
		kStateMysql8: {
			kWesql: kMysql,
		},
	}
	kWellKnownCharacterType = map[string]bool{
		kMysql: true,
	}
)

// CalcCharacterType calc wellknown CharacterType, if not wellknown return empty string
func CalcCharacterType(clusterType string, componentType string) string {
	v1, ok := kWellKnownTypeMaps[clusterType]
	if !ok {
		return kEmpty
	}
	v2, ok := v1[componentType]
	if !ok {
		return kEmpty
	}
	return v2
}

// IsWellKnownCharacterType check CharacterType is wellknown
func IsWellKnownCharacterType(characterType string) bool {
	return isWellKnowCharacterType(characterType, kWellKnownCharacterType)
}

func isWellKnowCharacterType(characterType string, wellKnownCharacterType map[string]bool) bool {
	val, ok := wellKnownCharacterType[characterType]
	if val && ok {
		return true
	}
	return false
}
