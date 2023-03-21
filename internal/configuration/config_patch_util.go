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

	"github.com/apecloud/kubeblocks/internal/unstructured"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
	leftOldKey := Difference(ToSet(oldVersion), keySet)
	leftNewKey := Difference(ToSet(newVersion), keySet)

	if !EqSet(leftOldKey, leftNewKey) {
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
