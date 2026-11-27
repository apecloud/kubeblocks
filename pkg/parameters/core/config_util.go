/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package core

import (
	"context"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/log"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

type ParamPairs struct {
	Key           string
	UpdatedParams map[string]interface{}
}

// MergeUpdatedConfig replaces the file content of the changed key.
// baseMap is the original configuration file,
// updatedMap is the updated configuration file
func MergeUpdatedConfig(baseMap, updatedMap map[string]string) map[string]string {
	r := make(map[string]string)
	for key, val := range baseMap {
		r[key] = val
		if v, ok := updatedMap[key]; ok {
			r[key] = v
		}
	}
	return r
}

// FromStringMap converts a map[string]string to a map[string]interface{}
func FromStringMap(m map[string]*string, valueTransformer ValueTransformer) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(m))
	for key, v := range m {
		value, err := DecodeParameterValue(key, v, valueTransformer)
		if err != nil {
			return nil, err
		}
		result[GetValidFieldName(key)] = value
	}
	return result, nil
}

// DecodeParameterValue reads a parameter value from the existing string representation.
func DecodeParameterValue(key string, value *string, transformer ValueTransformer) (any, error) {
	if value == nil {
		return nil, nil
	}
	if hasArrayField(key) {
		return fromJSONString(value), nil
	}
	if transformer != nil {
		return transformer.TransformValue(*value, key)
	}
	return *value, nil
}

func ApplyConfigPatch(baseCfg []byte,
	updatedParameters map[string]*string,
	formatConfig *parametersv1alpha1.FileFormatConfig,
	valueTransformer ValueTransformer,
) (string, error) {
	configLoaderOption := CfgOption{
		Type:    CfgRawType,
		Log:     log.FromContext(context.TODO()),
		CfgType: formatConfig.Format,
		RawData: baseCfg,
	}
	configWrapper, err := NewConfigLoader(configLoaderOption)
	if err != nil {
		return "", err
	}

	mergedOptions := NewCfgOptions("", WithFormatterConfig(formatConfig))
	params, err := FromStringMap(updatedParameters, valueTransformer)
	if err != nil {
		return "", err
	}
	if err = configWrapper.MergeFrom(params, mergedOptions); err != nil {
		return "", err
	}
	mergedConfig := configWrapper.getConfigObject(mergedOptions)
	return mergedConfig.Marshal()
}

func IsWatchModuleForShellTrigger(trigger *parametersv1alpha1.ShellTrigger) bool {
	if trigger == nil || trigger.Sync == nil {
		return true
	}
	return !*trigger.Sync
}

func ToV1ConfigDescription(keys []string, format *parametersv1alpha1.FileFormatConfig) []parametersv1alpha1.ComponentConfigDescription {
	var configs []parametersv1alpha1.ComponentConfigDescription
	for _, key := range keys {
		configs = append(configs, parametersv1alpha1.ComponentConfigDescription{
			Name:             filepath.Base(key),
			FileFormatConfig: format,
		})
	}
	return configs
}
