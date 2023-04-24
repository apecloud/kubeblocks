/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package configuration

import (
	"context"

	"github.com/StudioSol/set"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/unstructured"
)

// CreateConfigPatch creates a patch for configuration files with difference version.
func CreateConfigPatch(oldVersion, newVersion map[string]string, format appsv1alpha1.CfgFileFormat, keys []string, comparableAllFiles bool) (*ConfigPatchInfo, bool, error) {
	var hasFilesUpdated = false

	if comparableAllFiles && len(keys) > 0 {
		hasFilesUpdated = checkExcludeConfigDifference(oldVersion, newVersion, keys)
	}

	cmKeySet := FromCMKeysSelector(keys)
	patch, err := CreateMergePatch(
		FromConfigData(oldVersion, cmKeySet),
		FromConfigData(newVersion, cmKeySet),
		CfgOption{
			CfgType: format,
			Type:    CfgTplType,
			Log:     log.FromContext(context.TODO()),
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

func LoadRawConfigObject(data map[string]string, formatConfig *appsv1alpha1.FormatterConfig, keys []string) (map[string]unstructured.ConfigObject, error) {
	r := make(map[string]unstructured.ConfigObject)
	cmKeySet := FromCMKeysSelector(keys)
	for key, val := range data {
		if cmKeySet != nil && !cmKeySet.InArray(key) {
			continue
		}
		configObject, err := unstructured.LoadConfig(key, val, formatConfig.Format)
		if err != nil {
			return nil, err
		}
		if formatConfig.IniConfig != nil {
			configObject = configObject.SubConfig(formatConfig.IniConfig.SectionName)
		}
		r[key] = configObject
	}
	return r, nil
}

// TransformConfigFileToKeyValueMap transforms a config file which formed by appsv1alpha1.CfgFileFormat format to a map in which the key is config name and the value is config value。
// sectionName means the desired section of config file, such as [mysqld] section.
// If config file has no section structure, sectionName should be default to get all values in this config file.
func TransformConfigFileToKeyValueMap(fileName string, formatterConfig *appsv1alpha1.FormatterConfig, configData []byte) (map[string]string, error) {
	oldData := map[string]string{
		fileName: "",
	}
	newData := map[string]string{
		fileName: string(configData),
	}
	keys := []string{fileName}
	patchInfo, _, err := CreateConfigPatch(oldData, newData, formatterConfig.Format, keys, false)
	if err != nil {
		return nil, err
	}
	params := GenerateVisualizedParamsList(patchInfo, formatterConfig, nil)
	result := make(map[string]string)
	for _, param := range params {
		if param.Key != fileName {
			continue
		}
		for _, kv := range param.Parameters {
			result[kv.Key] = kv.Value
		}
	}
	return result, nil
}
