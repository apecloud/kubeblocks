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

package engine

import "fmt"

// DataEngines engineName --- moduleName ---context (custom define)
var DataEngines = map[string]map[string]interface{}{}

func Registry(engineName string, moduleName string, moduleContext interface{}) {
	v, ok := DataEngines[engineName]
	if ok {
		v[moduleName] = moduleContext
	} else {
		vv := map[string]interface{}{
			moduleName: moduleContext,
		}
		DataEngines[engineName] = vv
	}
}

func GetContext(engineName string, moduleName string) (interface{}, error) {
	v, ok := DataEngines[engineName]
	if !ok {
		return nil, fmt.Errorf("no registered data engine %s", engineName)
	} else {
		vv, ok := v[moduleName]
		if ok {
			return vv, nil
		} else {
			return nil, fmt.Errorf("no registered context for module %s", moduleName)
		}
	}
}
