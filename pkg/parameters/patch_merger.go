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

package parameters

import (
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func DoMerge(baseData map[string]string,
	patch map[string]parametersv1alpha1.ParametersInFile,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	configDescs []parametersv1alpha1.ComponentConfigDescription) (map[string]string, error) {
	var (
		updatedFiles  = make(map[string]string, len(patch))
		updatedParams = make([]core.ParamPairs, 0, len(patch))
	)

	builder := NewValueManager(paramsDefs, configDescs)
	for key, params := range patch {
		if params.Content != nil {
			updatedFiles[key] = *params.Content
		}
		if len(params.Parameters) > 0 {
			upParams, _ := core.FromStringMap(params.Parameters, builder.BuildValueTransformer(key))
			updatedParams = append(updatedParams, core.ParamPairs{
				Key:           key,
				UpdatedParams: upParams,
			})
		}
	}
	return mergeUpdatedParams(baseData, updatedFiles, updatedParams, paramsDefs, configDescs)
}

func mergeUpdatedParams(base map[string]string,
	updatedFiles map[string]string,
	updatedParams []core.ParamPairs,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	configDescs []parametersv1alpha1.ComponentConfigDescription) (map[string]string, error) {
	updatedConfig := base

	// merge updated files into configmap
	if len(updatedFiles) != 0 {
		updatedConfig = core.MergeUpdatedConfig(base, updatedFiles)
	}
	if len(configDescs) == 0 {
		return updatedConfig, nil
	}
	return MergeAndValidateConfigs(updatedConfig, updatedParams, paramsDefs, configDescs)
}
