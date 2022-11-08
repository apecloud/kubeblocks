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

package policy

import (
	"encoding/json"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

func GetUpdateParameterList(cfg *cfgcore.ConfigDiffInformation) ([]string, error) {
	params := make([]string, 0)
	for _, diff := range cfg.UpdateConfig {
		var updatedParams any
		if err := json.Unmarshal(diff, &updatedParams); err != nil {
			return nil, err
		}
		switch updatedParams.(type) {
		case map[string]interface{}:
			params = append(params, extractUpdatedParams(updatedParams)...)
		default:
			return nil, cfgcore.MakeError("expect to map json. actual: [%s]", diff)
		}
	}
	return params, nil
}

func extractUpdatedParams(params interface{}) []string {
	return extractMapParameters(params.(map[string]interface{}))
}

func extractMapParameters(object map[string]interface{}) []string {
	params := make([]string, 0)
	for key, value := range object {
		switch val := value.(type) {
		case []interface{}:
			// val := value.([]interface{})
			params = append(params, extractListParameters(key, val)...)
		case map[string]interface{}:
			// val := value.(map[string]interface{})
			params = append(params, extractMapParameters(val)...)
		default:
			params = append(params, key)
		}
	}
	return params
}

func extractListParameters(prefix string, objects []interface{}) []string {
	if len(objects) == 0 {
		return []string{prefix}
	}

	params := make([]string, 0)
	for _, value := range objects {
		switch val := value.(type) {
		case []interface{}:
			// val := value.([]interface{})
			params = append(params, extractListParameters(prefix, val)...)
		case map[string]interface{}:
			// val := value.(map[string]interface{})
			params = append(params, extractMapParameters(val)...)
		default:
			if prefix != "" {
				params = append(params, prefix)
			}
		}
	}
	return params
}

func IsUpdateDynamicParameters(tpl *dbaasv1alpha1.ConfigurationTemplateSpec, cfg *cfgcore.ConfigDiffInformation) (bool, error) {
	if len(cfg.DeleteConfig) > 0 || len(cfg.AddConfig) > 0 {
		return false, nil
	}

	if len(tpl.StaticParameters) == 0 {
		return true, nil
	}

	params, err := GetUpdateParameterList(cfg)
	if err != nil {
		return false, nil
	}
	updateParams := cfgcore.NewSetFromList(params)
	staticParams := cfgcore.NewSetFromList(tpl.StaticParameters)

	union := cfgcore.Union(staticParams, updateParams)
	return union.Empty(), nil
}
