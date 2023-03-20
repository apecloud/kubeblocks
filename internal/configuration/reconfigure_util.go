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

package configuration

import (
	"encoding/json"
	"reflect"

	"github.com/StudioSol/set"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func getUpdateParameterList(cfg *ConfigPatchInfo) ([]string, error) {
	params := make([]string, 0)
	walkFn := func(parent, cur string, v reflect.Value, fn UpdateFn) error {
		if cur != "" {
			params = append(params, cur)
		}
		return nil
	}

	for _, diff := range cfg.UpdateConfig {
		var updatedParams any
		if err := json.Unmarshal(diff, &updatedParams); err != nil {
			return nil, err
		}
		if err := UnstructuredObjectWalk(updatedParams, walkFn, true); err != nil {
			return nil, WrapError(err, "failed to walk params: [%s]", diff)
		}
	}
	return params, nil
}

// IsUpdateDynamicParameters is used to check whether the changed parameters require a restart
func IsUpdateDynamicParameters(tpl *appsv1alpha1.ConfigConstraintSpec, cfg *ConfigPatchInfo) (bool, error) {
	// TODO(zt) how to process new or delete file
	if len(cfg.DeleteConfig) > 0 || len(cfg.AddConfig) > 0 {
		return false, nil
	}

	params, err := getUpdateParameterList(cfg)
	if err != nil {
		return false, err
	}
	updateParams := set.NewLinkedHashSetString(params...)

	// if ConfigConstraint has StaticParameters, check updated parameter
	if len(tpl.StaticParameters) > 0 {
		staticParams := set.NewLinkedHashSetString(tpl.StaticParameters...)
		union := Union(staticParams, updateParams)
		if union.Length() > 0 {
			return false, nil
		}
		// if no dynamicParameters is configured, reload is the default behavior
		if len(tpl.DynamicParameters) == 0 {
			return true, nil
		}
	}

	// if ConfigConstraint has DynamicParameter, all updated param in dynamic params
	if len(tpl.DynamicParameters) > 0 {
		dynamicParams := set.NewLinkedHashSetString(tpl.DynamicParameters...)
		union := Difference(updateParams, dynamicParams)
		return union.Length() == 0, nil
	}

	// if the updated parameter is not in list of DynamicParameter and in list of StaticParameter,
	// restart is the default behavior.
	return false, nil
}
