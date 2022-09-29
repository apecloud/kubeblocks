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

package controllerutil

const (
	MYSQL = "mysql"
)

var (
	wellKnownCharacterType = map[string]bool{
		MYSQL: true,
	}
)

func CalcCharacterType(clusterType string, componentType string) string {
	switch {
	case clusterType == "stat.mysql" && componentType == "replicasets":
		return MYSQL
	case clusterType == "state.mysql-8" && componentType == "replicasets":
		return MYSQL
	}
	return ""
}

func IsWellKnownCharacterType(characterType string) bool {
	return isWellKnowCharacterType(characterType, wellKnownCharacterType)
}

func isWellKnowCharacterType(characterType string, wellKnownCharacterType map[string]bool) bool {
	val, ok := wellKnownCharacterType[characterType]
	if val && ok {
		return true
	}
	return false
}
