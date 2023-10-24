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

package builder

import (
	"embed"
	"fmt"

	"cuelang.org/go/cue"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/leaanthony/debme"
	"gopkg.in/yaml.v2"
)

var (
	//go:embed cue/*
	cueTemplates embed.FS
	cacheCtx     = map[string]interface{}{}
)

func getCacheCUETplValue(key string, valueCreator func() (*intctrlutil.CUETpl, error)) (*intctrlutil.CUETpl, error) {
	vIf, ok := cacheCtx[key]
	if ok {
		return vIf.(*intctrlutil.CUETpl), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	cacheCtx[key] = v
	return v, err
}

func GetSubDirFileNames(subDir string) ([]string, error) {
	cueFS, err := debme.FS(cueTemplates, fmt.Sprintf("cue/%s", subDir))
	if err != nil {
		return nil, err
	}
	cueTplFiles, err := cueFS.ReadDir(".")
	if err != nil {
		return nil, err
	}
	fileNames := make([]string, len(cueTplFiles))
	for i, file := range cueTplFiles {
		fileNames[i] = file.Name()
	}
	return fileNames, nil
}

func BuildFromCUEForOTel(tplName string, fillMap map[string]any, lookupKey string) ([]byte, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := getCacheCUETplValue(tplName, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplName))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	for k, v := range fillMap {
		if err := cueValue.FillObjWithRelativePath("parameters", k, v); err != nil {
			return nil, err
		}
	}

	value := cueValue.Value.LookupPath(cue.ParsePath(lookupKey))

	bytes, err := value.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var jsonObj interface{}
	if err := yaml.Unmarshal(bytes, &jsonObj); err != nil {
		return nil, err
	}
	yamlBytes, err := yaml.Marshal(jsonObj)
	if err != nil {
		return nil, err
	}
	return yamlBytes, nil
}

func MergeValMapFromYamlStr(defaultMap map[string]any, yamlStr string) {
	if defaultMap == nil {
		defaultMap = map[string]any{}
	}
	valMap := map[string]any{}
	err := yaml.Unmarshal([]byte(yamlStr), &valMap)
	if err != nil {
		return
	}
	for k, v := range valMap {
		defaultMap[k] = v
	}
}
