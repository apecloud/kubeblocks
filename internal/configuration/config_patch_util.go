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
func TransformConfigFileToKeyValueMap(fileName, sectionName string, format appsv1alpha1.CfgFileFormat, configData []byte) (map[string]string, error) {
	oldData := map[string]string{
		fileName: "",
	}
	newData := map[string]string{
		fileName: string(configData),
	}
	keys := []string{fileName}
	patchInfo, _, err := CreateConfigPatch(oldData, newData, format, keys, false)
	if err != nil {
		return nil, err
	}
	formatConfig := &appsv1alpha1.FormatterConfig{
		Format:           format,
		FormatterOptions: appsv1alpha1.FormatterOptions{IniConfig: &appsv1alpha1.IniConfig{SectionName: sectionName}},
	}
	params := GenerateVisualizedParamsList(patchInfo, formatConfig, nil)
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
