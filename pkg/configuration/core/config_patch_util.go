/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"

	"github.com/StudioSol/set"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/unstructured"
)

// CreateConfigPatch creates a patch for configuration files with different version.
func CreateConfigPatch(oldVersion, newVersion map[string]string, configRender parametersv1alpha1.ParamConfigRendererSpec, comparableAllFiles bool) (*ConfigPatchInfo, bool, error) {
	var hasFilesUpdated = false
	var keys = ResolveConfigFiles(configRender.Configs)

	if comparableAllFiles && len(keys) > 0 {
		hasFilesUpdated = checkExcludeConfigDifference(oldVersion, newVersion, keys)
	}

	cmKeyFilter := NewConfigFileFilter(configRender.Configs)
	patch, err := CreateMergePatch(
		FromConfigData(oldVersion, cmKeyFilter),
		FromConfigData(newVersion, cmKeyFilter),
		CfgOption{
			FileFormatFn: WithConfigFileFormat(configRender.Configs),
			Type:         CfgTplType,
			Log:          log.FromContext(context.TODO()),
		})
	return patch, hasFilesUpdated, err
}

func checkExcludeConfigDifference(oldVersion map[string]string, newVersion map[string]string, keys []string) bool {
	keySet := set.NewLinkedHashSetString(keys...)
	leftOldKey := util.Difference(util.ToSet(oldVersion), keySet)
	leftNewKey := util.Difference(util.ToSet(newVersion), keySet)

	if !util.EqSet(leftOldKey, leftNewKey) {
		return true
	}

	for e := range leftOldKey.Iter() {
		if oldVersion[e] != newVersion[e] {
			return true
		}
	}
	return false
}

func LoadRawConfigObject(data map[string]string, formatConfig *parametersv1alpha1.FileFormatConfig, keys []string) (map[string]unstructured.ConfigObject, error) {
	r := make(map[string]unstructured.ConfigObject)
	cmKeySet := FromCMKeysSelector(keys)
	for key, val := range data {
		if cmKeySet != nil && !cmKeySet.InArray(key) {
			continue
		}
		configObject, err := FromConfigObject(key, val, formatConfig)
		if err != nil {
			return nil, err
		}
		r[key] = configObject
	}
	return r, nil
}

func FromConfigObject(name, config string, formatConfig *parametersv1alpha1.FileFormatConfig) (unstructured.ConfigObject, error) {
	configObject, err := unstructured.LoadConfig(name, config, formatConfig.Format)
	if err != nil {
		return nil, err
	}
	if formatConfig.IniConfig != nil {
		configObject = configObject.SubConfig(formatConfig.IniConfig.SectionName)
	}
	return configObject, nil
}

// TransformConfigFileToKeyValueMap transforms a config file in appsv1alpha1.CfgFileFormat format to a map in which the key is config name and the value is config value
// sectionName means the desired section of config file, such as [mysqld] section.
// If config file has no section structure, sectionName should be default to get all values in this config file.
func TransformConfigFileToKeyValueMap(fileName string, configRender parametersv1alpha1.ParamConfigRendererSpec, configData []byte) (map[string]string, error) {
	formatterConfig := ResolveConfigFormat(configRender.Configs, fileName)
	if formatterConfig == nil {
		return nil, fmt.Errorf("not found file formatter config: [%s]", fileName)
	}

	oldData := map[string]string{
		fileName: "",
	}
	newData := map[string]string{
		fileName: string(configData),
	}
	patchInfo, _, err := CreateConfigPatch(oldData, newData, configRender, false)
	if err != nil {
		return nil, err
	}
	params := GenerateVisualizedParamsList(patchInfo, configRender.Configs)
	result := make(map[string]string)
	for _, param := range params {
		if param.Key != fileName {
			continue
		}
		for _, kv := range param.Parameters {
			if kv.Value != nil {
				result[kv.Key] = *kv.Value
			}
		}
	}
	return result, nil
}

func ResolveConfigFormat(descriptions []parametersv1alpha1.ComponentConfigDescription, file string) *parametersv1alpha1.FileFormatConfig {
	for _, config := range descriptions {
		if config.Name == file {
			return config.FileFormatConfig
		}
	}
	return nil
}

func WithConfigFileFormat(descriptions []parametersv1alpha1.ComponentConfigDescription) func(file string) *parametersv1alpha1.FileFormatConfig {
	return func(file string) *parametersv1alpha1.FileFormatConfig {
		return ResolveConfigFormat(descriptions, file)
	}
}

func ResolveConfigFiles(descriptions []parametersv1alpha1.ComponentConfigDescription) []string {
	var keys []string
	for _, config := range descriptions {
		keys = append(keys, config.Name)
	}
	return keys
}

func NewConfigFileFilter(descriptions []parametersv1alpha1.ComponentConfigDescription) *util.Sets {
	return util.NewSet(ResolveConfigFiles(descriptions)...)
}
