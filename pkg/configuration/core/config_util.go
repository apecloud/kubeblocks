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

package core

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cast"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
)

type ParamPairs struct {
	Key           string
	UpdatedParams map[string]interface{}
}

const pattern = `^[a-z0-9A-Z]([a-zA-Z0-9\.\-\_]*[a-zA-Z0-9])?$`
const validLabelLength = 63

var regxPattern = regexp.MustCompile(pattern)

func FromValueToString(val interface{}) string {
	str := strings.Trim(cast.ToString(val), ` '"`)
	if IsValidLabelKeyOrValue(str) {
		return str
	}
	return ""
}

// IsValidLabelKeyOrValue checks if the input string is a valid label key or value
// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
func IsValidLabelKeyOrValue(label string) bool {
	return len(label) <= validLabelLength && regxPattern.MatchString(label)
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
		switch {
		case hasArrayField(key):
			result[GetValidFieldName(key)] = fromJSONString(v)
		case v != nil:
			if err := wrapValue(result, *v, key, valueTransformer); err != nil {
				return nil, err
			}
		default:
			result[key] = nil
		}
	}
	return result, nil
}

func wrapValue(result map[string]interface{}, paramValue string, paramName string, transformer ValueTransformer) error {
	value := interface{}(paramValue)

	if transformer != nil {
		transformedValue, err := transformer.TransformValue(paramValue, paramName)
		if err != nil {
			return err
		}
		value = transformedValue
	}

	result[paramName] = value
	return nil
}

// FromStringPointerMap converts a map[string]string to a map[string]interface{}
func FromStringPointerMap(m map[string]string) map[string]*string {
	r := make(map[string]*string, len(m))
	for key, v := range m {
		r[key] = cfgutil.ToPointer(v)
	}
	return r
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

func IsWatchModuleForTplTrigger(trigger *parametersv1alpha1.TPLScriptTrigger) bool {
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
