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

package policy

import (
	"encoding/json"
	"reflect"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

func getUpdateParameterList(cfg *cfgcore.ConfigDiffInformation) ([]string, error) {
	params := make([]string, 0)
	for _, diff := range cfg.UpdateConfig {
		var updatedParams any
		if err := json.Unmarshal(diff, &updatedParams); err != nil {
			return nil, err
		}
		if err := cfgcore.UnstructuredObjectWalk(updatedParams,
			func(parent, cur string, v reflect.Value, fn cfgcore.UpdateFn) error {
				if cur != "" {
					params = append(params, cur)
				}
				return nil
			}, true); err != nil {
			return nil, cfgcore.WrapError(err, "failed to walk params: [%s]", diff)
		}
	}
	return params, nil
}

func isUpdateDynamicParameters(tpl *dbaasv1alpha1.ConfigurationTemplateSpec, cfg *cfgcore.ConfigDiffInformation) (bool, error) {
	// TODO(zt) how to process new or delete file
	if len(cfg.DeleteConfig) > 0 || len(cfg.AddConfig) > 0 {
		return false, nil
	}

	params, err := getUpdateParameterList(cfg)
	if err != nil {
		return false, err
	}
	updateParams := cfgcore.NewSetFromList(params)

	// if has StaticParameters, update static parameter
	if len(tpl.StaticParameters) > 0 {
		staticParams := cfgcore.NewSetFromList(tpl.StaticParameters)
		union := cfgcore.Union(staticParams, updateParams)
		if !union.Empty() {
			return false, nil
		}
	}

	// if has dynamic parameters, all updated param in dynamic params
	if len(tpl.DynamicParameters) > 0 {
		dynamicParams := cfgcore.NewSetFromList(tpl.DynamicParameters)
		union := cfgcore.Difference(updateParams, dynamicParams)
		return union.Empty(), nil
	}

	// default static parameters
	return false, nil
}
