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

import "github.com/apecloud/kubeblocks/internal/configuration/util"

func CreateMergePatch(oldVersion, newVersion interface{}, option CfgOption) (*ConfigPatchInfo, error) {

	ok, err := compareWithConfig(oldVersion, newVersion, option)
	if err != nil {
		return nil, err
	} else if ok {
		return &ConfigPatchInfo{IsModify: false}, err
	}

	old, err := NewConfigLoader(withOption(option, oldVersion))
	if err != nil {
		return nil, WrapError(err, "failed to create config: [%s]", oldVersion)
	}

	new, err := NewConfigLoader(withOption(option, newVersion))
	if err != nil {
		return nil, WrapError(err, "failed to create config: [%s]", oldVersion)
	}
	return difference(old.cfgWrapper, new.cfgWrapper)
}

func difference(base *cfgWrapper, target *cfgWrapper) (*ConfigPatchInfo, error) {
	fromOMap := util.ToSet(base.indexer)
	fromNMap := util.ToSet(target.indexer)

	addSet := util.Difference(fromNMap, fromOMap)
	deleteSet := util.Difference(fromOMap, fromNMap)
	updateSet := util.Difference(fromOMap, deleteSet)

	reconfigureInfo := &ConfigPatchInfo{
		IsModify:     false,
		AddConfig:    make(map[string]interface{}, addSet.Length()),
		DeleteConfig: make(map[string]interface{}, deleteSet.Length()),
		UpdateConfig: make(map[string][]byte, updateSet.Length()),

		Target:      target,
		LastVersion: base,
	}

	for elem := range addSet.Iter() {
		reconfigureInfo.AddConfig[elem] = target.indexer[elem].GetAllParameters()
		reconfigureInfo.IsModify = true
	}

	for elem := range deleteSet.Iter() {
		reconfigureInfo.DeleteConfig[elem] = base.indexer[elem].GetAllParameters()
		reconfigureInfo.IsModify = true
	}

	for elem := range updateSet.Iter() {
		old := base.indexer[elem]
		new := target.indexer[elem]

		patch, err := util.JSONPatch(old.GetAllParameters(), new.GetAllParameters())
		if err != nil {
			return nil, err
		}
		if len(patch) > len(emptyJSON) {
			reconfigureInfo.UpdateConfig[elem] = patch
			reconfigureInfo.IsModify = true
		}
	}

	return reconfigureInfo, nil
}
