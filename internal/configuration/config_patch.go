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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
)

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

func TransformConfigPatchFromData(data map[string]string, format appsv1alpha1.CfgFileFormat, keys []string) (*ConfigPatchInfo, error) {
	emptyData := func(m map[string]string) map[string]string {
		r := make(map[string]string, len(m))
		for key := range m {
			r[key] = ""
		}
		return r
	}
	patch, _, err := CreateConfigPatch(emptyData(data), data, format, keys, false)
	return patch, err
}
