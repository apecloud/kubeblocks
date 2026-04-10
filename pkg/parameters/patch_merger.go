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
	"fmt"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func DoMerge(baseData map[string]string,
	patch map[string]parametersv1alpha1.ParametersInFile,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	configDescs []parametersv1alpha1.ComponentConfigDescription) (map[string]string, error) {
	var (
		updatedFiles            = make(map[string]string, len(patch))
		updatedParams           = make([]core.ParamPairs, 0, len(patch))
		unmanagedUpdatedByFiles = make(map[string][]parametersv1alpha1.UnmanagedParameterSectionUpdate, len(patch))
	)

	builder := NewValueManager(paramsDefs, configDescs)
	for key, params := range patch {
		if params.Content != nil {
			updatedFiles[key] = *params.Content
		}
		if len(params.Parameters) > 0 {
			upParams, _ := core.FromStringMap(DecodeParameterOverlay(params.Parameters), builder.BuildValueTransformer(key))
			updatedParams = append(updatedParams, core.ParamPairs{
				Key:           key,
				UpdatedParams: upParams,
			})
		}
		if len(params.UnmanagedUpdates) > 0 {
			unmanagedUpdatedByFiles[key] = params.UnmanagedUpdates
		}
	}
	return mergeUpdatedParams(baseData, updatedFiles, updatedParams, unmanagedUpdatedByFiles, paramsDefs, configDescs)
}

func mergeUpdatedParams(base map[string]string,
	updatedFiles map[string]string,
	updatedParams []core.ParamPairs,
	unmanagedUpdatedByFiles map[string][]parametersv1alpha1.UnmanagedParameterSectionUpdate,
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
	updatedConfig, err := MergeAndValidateConfigs(updatedConfig, updatedParams, paramsDefs, configDescs)
	if err != nil {
		return nil, err
	}
	return mergeUnmanagedUpdates(updatedConfig, unmanagedUpdatedByFiles, configDescs)
}

func mergeUnmanagedUpdates(base map[string]string,
	unmanagedUpdatedByFiles map[string][]parametersv1alpha1.UnmanagedParameterSectionUpdate,
	configDescs []parametersv1alpha1.ComponentConfigDescription) (map[string]string, error) {
	if len(unmanagedUpdatedByFiles) == 0 {
		return base, nil
	}
	updatedConfig := core.MergeUpdatedConfig(base, nil)
	for file, sectionUpdates := range unmanagedUpdatedByFiles {
		current, ok := updatedConfig[file]
		if !ok {
			current = ""
		}
		fileFormat := core.ResolveConfigFormat(configDescs, file)
		if fileFormat == nil {
			return nil, fmt.Errorf("not support unmanaged updates for file: %s", file)
		}
		next := current
		for _, sectionUpdate := range sectionUpdates {
			formatConfig, err := resolveUnmanagedFormatConfig(fileFormat, sectionUpdate.Section)
			if err != nil {
				return nil, err
			}
			normalizedUpdates, err := normalizeUnmanagedParameterUpdates(sectionUpdate.Updates)
			if err != nil {
				return nil, err
			}
			next, err = core.ApplyConfigPatch([]byte(next), normalizedUpdates, formatConfig, nil)
			if err != nil {
				return nil, err
			}
		}
		updatedConfig[file] = next
	}
	return updatedConfig, nil
}

func resolveUnmanagedFormatConfig(base *parametersv1alpha1.FileFormatConfig, section *string) (*parametersv1alpha1.FileFormatConfig, error) {
	if base == nil {
		if section != nil {
			return nil, fmt.Errorf("section is not supported without file format configuration")
		}
		return nil, fmt.Errorf("file format configuration is required for unmanaged updates")
	}
	formatConfig := base.DeepCopy()
	if section == nil {
		return formatConfig, nil
	}
	if formatConfig.Format != parametersv1alpha1.Ini {
		return nil, fmt.Errorf("section is only supported for ini unmanaged updates")
	}
	if formatConfig.IniConfig == nil {
		formatConfig.IniConfig = &parametersv1alpha1.IniConfig{}
	}
	formatConfig.IniConfig.SectionName = *section
	return formatConfig, nil
}

func normalizeUnmanagedParameterUpdates(updates []parametersv1alpha1.ParameterUpdate) (map[string]*string, error) {
	normalized := make(map[string]*string, len(updates))
	for _, update := range updates {
		switch update.Type {
		case parametersv1alpha1.ParameterUpdateSet:
			if update.Value == nil {
				return nil, fmt.Errorf("unmanaged parameter update %q with type %q requires a value", update.Key, update.Type)
			}
			normalized[update.Key] = update.Value
		case parametersv1alpha1.ParameterUpdateRemove:
			normalized[update.Key] = nil
		default:
			return nil, fmt.Errorf("unsupported unmanaged parameter update type %q for key %q", update.Type, update.Key)
		}
	}
	return normalized, nil
}
