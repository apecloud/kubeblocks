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

package helm

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	kindBlackList = []string{
		"ConfigMapList",
	}

	nameBlackList = []string{
		"grafana",
		"prometheus",
	}

	fieldBlackList = []string{
		"description",
		"image",
		"chartLocationURL",
	}

	labelBlackList = []string{
		"helm.sh/chart",
		"app.kubernetes.io/version",
	}
)

// MappingResult to store result of diff
type MappingResult struct {
	Name    string
	Kind    string
	Content string
}

type metadata struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name   string            `yaml:"name"`
		Labels map[string]string `yaml:"labels"`
	}
}

func (m metadata) String() string {
	apiBase := m.APIVersion
	sp := strings.Split(apiBase, "/")
	if len(sp) > 1 {
		apiBase = strings.Join(sp[:len(sp)-1], "/")
	}
	name := m.Metadata.Name
	return fmt.Sprintf("%s, %s (%s)", name, m.Kind, apiBase)
}

func ParseContent(content string) (*MappingResult, error) {
	var parsedMetadata metadata
	if err := yaml.Unmarshal([]byte(content), &parsedMetadata); err != nil {
		log.Fatalf("YAML unmarshal error: %s\nCan't unmarshal %s", err, content)
	}
	if parsedMetadata.APIVersion == "" && parsedMetadata.Kind == "" {
		return nil, nil
	}
	// filter Kind
	for i := range kindBlackList {
		if kindBlackList[i] == parsedMetadata.Kind {
			return nil, nil
		}
	}
	// filter Name
	for i := range nameBlackList {
		if strings.Contains(parsedMetadata.Metadata.Name, nameBlackList[i]) {
			return nil, nil
		}
	}

	var object map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(content), &object); err != nil {
		log.Fatalf("YAML unmarshal error: %s\nCan't unmarshal %s", err, content)
	}
	// filter Label
	for i := range labelBlackList {
		deleteLabel(&object, labelBlackList[i])
	}
	// filter Field
	for i := range fieldBlackList {
		deleteObjField(&object, fieldBlackList[i])
	}
	normalizedContent, err := yaml.Marshal(object)
	if err != nil {
		log.Fatalf("YAML marshal error: %s\nCan't marshal %v", err, object)
	}
	content = string(normalizedContent)
	name := parsedMetadata.String()
	return &MappingResult{
		Name:    name,
		Kind:    parsedMetadata.Kind,
		Content: content,
	}, nil
}

func OutputDiff(releaseA *release.Release, releaseB *release.Release, versionA, versionB string, out io.Writer) error {
	manifestsMapA, err := buildManifestMapByRelease(releaseA)
	if err != nil {
		return err
	}
	manifestsMapB, err := buildManifestMapByRelease(releaseB)
	if err != nil {
		return err
	}

	mayRemove := make([]*MappingResult, 0)
	mayAdd := make([]*MappingResult, 0)

	for _, key := range sortedKeys(manifestsMapA) {
		manifestA := manifestsMapA[key]
		if manifestB, ok := manifestsMapB[key]; ok {
			if manifestA.Content == manifestB.Content {
				continue
			}
			diffString, err := util.GetUnifiedDiffString(manifestA.Content, manifestB.Content, fmt.Sprintf("%s %s", manifestA.Name, versionA), fmt.Sprintf("%s %s", manifestB.Name, versionB))
			if err != nil {
				return err
			}
			util.DisplayDiffWithColor(out, diffString)
		} else {
			mayRemove = append(mayRemove, manifestA)
		}

	}

	// Todo: support find Rename chart.yaml between mayRemove and mayAdd
	for k, v := range manifestsMapB {
		if _, ok := manifestsMapA[k]; !ok {
			mayAdd = append(mayAdd, v)
		}
	}

	for _, elem := range mayAdd {
		diffString, err := util.GetUnifiedDiffString("", elem.Content, "", fmt.Sprintf("%s %s", elem.Name, versionB))
		if err != nil {
			return err
		}
		util.DisplayDiffWithColor(out, diffString)
	}

	for _, elem := range mayRemove {
		diffString, err := util.GetUnifiedDiffString(elem.Content, "", fmt.Sprintf("%s %s", elem.Name, versionA), "")
		if err != nil {
			return err
		}
		util.DisplayDiffWithColor(out, diffString)
	}
	return nil
}

func buildManifestMapByRelease(release *release.Release) (map[string]*MappingResult, error) {
	var manifests bytes.Buffer
	fmt.Fprintln(&manifests, strings.TrimSpace(release.Manifest))
	manifestsKeys := releaseutil.SplitManifests(manifests.String())
	manifestsMap := make(map[string]*MappingResult)
	for _, v := range manifestsKeys {
		mapResult, err := ParseContent(v)
		if err != nil {
			return nil, err
		}
		if mapResult == nil {
			continue
		}
		manifestsMap[mapResult.Name] = mapResult
	}
	return manifestsMap, nil
}

func sortedKeys(manifests map[string]*MappingResult) []string {
	keys := maps.Keys(manifests)
	sort.Strings(keys)
	return keys
}

func deleteObjField(obj *map[interface{}]interface{}, field string) {
	ori := *obj
	_, ok := ori[field]
	if ok {
		delete(ori, field)
	}

	for _, v := range ori {
		if v == nil {
			continue
		}
		switch reflect.TypeOf(v).Kind() {
		case reflect.Map:
			m := v.(map[interface{}]interface{})
			deleteObjField(&m, field)
		case reflect.Slice:
			s := v.([]interface{})
			for i := range s {
				if m, ok := s[i].(map[interface{}]interface{}); ok {
					deleteObjField(&m, field)
				}
			}
		}
	}
}

func deleteLabel(object *map[interface{}]interface{}, s string) {
	obj := *object
	if _, ok := obj["metadata"]; !ok {
		return
	}

	if m, ok := obj["metadata"].(map[interface{}]interface{}); ok {
		label, ok := m["labels"].(map[interface{}]interface{})
		if !ok {
			return
		}
		if label[s] != "" {
			delete(label, s)
		}
	}
}
