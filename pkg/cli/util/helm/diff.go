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
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cast"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"

	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

// constants in k8s yaml
const k8sCRD = "CustomResourceDefinition"
const TYPE = "type"
const PROPERTIES = "properties"
const ADDITIONALPROPERTIES = "additionalProperties"
const REQUIRED = "required"

// APIPath is the key name to record the API fullpath
const APIPath = "KB-API-PATH"

var (
	// four level BlackList to filter useless info between two release, now they are customized for kubeblocks
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
		"chartsImage",
	}

	labelBlackList = []string{
		"helm.sh/chart",
		"app.kubernetes.io/version",
	}
)

// MappingResult to store result to diff
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

const (
	object string = "object"
	array  string = "array"
)

type Mode string

const (
	Modified Mode = "Modified"
	Added    Mode = "Added"
	Removed  Mode = "Removed"
)

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

	parseOpenAPIV3Schema := func() (*MappingResult, error) {
		data := make(map[string]interface{})
		if err := yaml.Unmarshal([]byte(content), &data); err != nil {
			return nil, err
		}
		// The content must strictly adhere to Kubernetes' YAML format to ensure correct parsing of the CRD's API Schema
		openAPIV3Schema := cast.ToStringMap(cast.ToStringMap(cast.ToStringMap(cast.ToSlice(cast.ToStringMap(data["spec"])["versions"])[0])["schema"])["openAPIV3Schema"])[PROPERTIES]
		normalizedContent, err := yaml.Marshal(openAPIV3Schema)
		if err != nil {
			return nil, err
		}
		return &MappingResult{
			Name:    parsedMetadata.String(),
			Kind:    k8sCRD,
			Content: string(normalizedContent),
		}, nil
	}

	if err := yaml.Unmarshal([]byte(content), &parsedMetadata); err != nil {

		return nil, err
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
	if k8sCRD == parsedMetadata.Kind {
		return parseOpenAPIV3Schema()
	}
	var object map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(content), &object); err != nil {
		return nil, err
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
		return nil, err
	}
	content = string(normalizedContent)
	name := parsedMetadata.String()
	return &MappingResult{
		Name:    name,
		Kind:    parsedMetadata.Kind,
		Content: content,
	}, nil
}

// OutputDiff output the difference between different version for a chart
// releaseA corresponds to versionA and releaseB corresponds to versionB.
// if detail is true, the detailed lines in YAML will be displayed
func OutputDiff(releaseA *release.Release, releaseB *release.Release, versionA, versionB string, out io.Writer, detail bool) error {
	manifestsMapA, err := buildManifestMapByRelease(releaseA)
	if err != nil {
		return err
	}
	manifestsMapB, err := buildManifestMapByRelease(releaseB)
	if err != nil {
		return err
	}

	var mayRemoveCRD []string
	var mayAddCRD []string
	for _, key := range sortedKeys(manifestsMapA) {
		manifestA := manifestsMapA[key]
		if manifestA.Kind != k8sCRD {
			continue
		}
		apiContentsA := make(map[string]any)
		err := yaml.Unmarshal([]byte(manifestA.Content), &apiContentsA)
		if err != nil {
			return err
		}
		if manifestB, ok := manifestsMapB[key]; ok {
			if manifestA.Content == manifestB.Content {
				continue
			}
			apiContentsB := make(map[string]any)
			err := yaml.Unmarshal([]byte(manifestB.Content), &apiContentsB)
			if err != nil {
				return err
			}
			outputCRDDiff(apiContentsA, apiContentsB, strings.Split(key, ",")[0], out)
		} else {
			mayRemoveCRD = append(mayRemoveCRD, manifestA.Name)
		}
	}

	for _, key := range sortedKeys(manifestsMapB) {
		manifestB := manifestsMapB[key]
		if manifestB.Kind != k8sCRD {
			continue
		}
		if _, ok := manifestsMapA[key]; !ok {
			mayAddCRD = append(mayAddCRD, manifestB.Name)
		}
	}
	tblPrinter := printer.NewTablePrinter(out)
	tblPrinter.SetHeader("CustomResourceDefinition", "MODE")
	sort.Strings(mayRemoveCRD)
	sort.Strings(mayAddCRD)

	for i := range mayRemoveCRD {
		tblPrinter.AddRow(strings.Split(mayRemoveCRD[i], ",")[0], printer.BoldRed(Removed))
	}
	for i := range mayAddCRD {
		tblPrinter.AddRow(strings.Split(mayAddCRD[i], ",")[0], printer.BoldGreen(Added))
	}
	if tblPrinter.Tbl.Length() != 0 {
		tblPrinter.Print()
		printer.PrintBlankLine(out)
	}
	// detail will output the yaml files change
	if !detail {
		return nil
	}
	mayRemove := make([]*MappingResult, 0)
	mayAdd := make([]*MappingResult, 0)
	for _, key := range sortedKeys(manifestsMapA) {
		manifestA := manifestsMapA[key]
		if manifestB, ok := manifestsMapB[key]; ok {
			if manifestA.Content == manifestB.Content {
				continue
			}
			diffString, err := util.GetUnifiedDiffString(manifestA.Content, manifestB.Content, fmt.Sprintf("%s %s", manifestA.Name, versionA), fmt.Sprintf("%s %s", manifestB.Name, versionB), 1)
			if err != nil {
				return err
			}
			util.DisplayDiffWithColor(out, diffString)
		} else {
			mayRemove = append(mayRemove, manifestA)
		}

	}

	// Todo: support find Rename chart.yaml between mayRemove and mayAdd
	for _, key := range sortedKeys(manifestsMapB) {
		manifestB := manifestsMapB[key]
		if _, ok := manifestsMapA[key]; !ok {
			mayAdd = append(mayAdd, manifestB)
		}
	}

	for _, elem := range mayAdd {
		diffString, err := util.GetUnifiedDiffString("", elem.Content, "", fmt.Sprintf("%s %s", elem.Name, versionB), 1)
		if err != nil {
			return err
		}
		util.DisplayDiffWithColor(out, diffString)
	}

	for _, elem := range mayRemove {
		diffString, err := util.GetUnifiedDiffString(elem.Content, "", fmt.Sprintf("%s %s", elem.Name, versionA), "", 1)
		if err != nil {
			return err
		}
		util.DisplayDiffWithColor(out, diffString)
	}
	return nil
}

// buildManifestMapByRelease parse a helm release manifest, it will get a map which include all k8s resources in
// the helm release and the map name is generate by metadata.String()
func buildManifestMapByRelease(release *release.Release) (map[string]*MappingResult, error) {
	if release == nil {
		return map[string]*MappingResult{}, nil
	}
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
			// resources in BlackList
			continue
		}
		manifestsMap[mapResult.Name] = mapResult
	}
	return manifestsMap, nil
}

// sortedKeys return sorted keys of manifests
func sortedKeys[K any](manifests map[string]K) []string {
	keys := maps.Keys(manifests)
	sort.Strings(keys)
	return keys
}

// deleteObjField delete the field in fieldBlackList recursively
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

// deleteLabel delete the label in labelBlackList
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

// outputCRDDiff will compare and output the differences between crdA and crdB for the same crd named crdName
func outputCRDDiff(crdA, crdB map[string]any, crdName string, out io.Writer) {
	fmt.Fprintf(out, "%s\n", printer.BoldYellow(crdName))
	tblPrinter := printer.NewTablePrinter(out)
	tblPrinter.SetHeader("API", "IS-REQUIRED", "MODE", "DETAILS")
	tblPrinter.SortBy(3, 1)
	getNextLevelAPI := func(curPath, key string) string {
		if len(curPath) == 0 {
			return key
		}
		return curPath + "." + key
	}
	crdA[APIPath] = ""
	crdB[APIPath] = ""
	var queueA []map[string]any = []map[string]any{crdA}
	queueB := make(map[string]map[string]any)
	requiredA := make(map[string]bool) // to remember requiredAPI
	requiredB := make(map[string]bool)
	queueB[""] = crdB

	for len(queueA) > 0 {
		curA := queueA[0]
		queueA = queueA[1:]
		curAPath := curA[APIPath].(string)
		curB := queueB[curAPath]
		if curB == nil {
			// crdA have API but crdB do not have
			tblPrinter.AddRow(curAPath, requiredA[curAPath], printer.BoldRed(Removed))
			continue
		}
		delete(queueB, curAPath)
		// add Content crdB
		for key, val := range curB {
			if key == APIPath {
				continue
			}
			contentB := cast.ToStringMap(val)
			nextLevelAPIKey := getNextLevelAPI(curAPath, key)
			if slice := cast.ToSlice(contentB[REQUIRED]); slice != nil {
				for _, key := range slice {
					requiredB[getNextLevelAPI(nextLevelAPIKey, key.(string))] = true
				}
			}
			switch t, _ := contentB[TYPE].(string); t {
			case object:
				queueB[nextLevelAPIKey] = cast.ToStringMap(contentB[PROPERTIES])
			case array:
				itemContent := cast.ToStringMap(contentB["items"])
				curPath := getNextLevelAPI(nextLevelAPIKey, "items")
				if slice := cast.ToSlice(itemContent[REQUIRED]); slice != nil {
					for _, key := range slice {
						requiredB[getNextLevelAPI(curPath, key.(string))] = true
					}
				}
				queueB[curPath] = cast.ToStringMap(itemContent[PROPERTIES])
			default:
				queueB[nextLevelAPIKey] = cast.ToStringMap(val)
			}
		}

		// check api if equal and add next level api
		for key, val := range curA {
			if key == APIPath {
				continue
			}
			contentA := cast.ToStringMap(val)
			nextLevelAPIKey := getNextLevelAPI(curAPath, key)
			contentB := cast.ToStringMap(curB[key])

			delete(contentA, "description")
			delete(contentB, "description")
			if slice := cast.ToSlice(contentA[REQUIRED]); slice != nil {
				for _, key := range slice {
					requiredA[getNextLevelAPI(nextLevelAPIKey, key.(string))] = true
				}
			}
			// compare contentA and contentB rules by different Type
			if requiredA[nextLevelAPIKey] != requiredB[nextLevelAPIKey] {
				tblPrinter.AddRow(nextLevelAPIKey, fmt.Sprintf("%v -> %v", requiredA[nextLevelAPIKey], requiredB[nextLevelAPIKey]), printer.BoldYellow(Modified))
			}
			switch t, _ := contentA[TYPE].(string); t {
			case object:
				// compare object , check required
				nextLevelAPI := cast.ToStringMap(contentA[PROPERTIES])
				nextLevelAPI[APIPath] = nextLevelAPIKey
				queueA = append(queueA, nextLevelAPI)
			case array:
				itemContent := cast.ToStringMap(contentA["items"])
				curPath := getNextLevelAPI(nextLevelAPIKey, "items")
				if slice := cast.ToSlice(itemContent[REQUIRED]); slice != nil {
					for _, key := range slice {
						requiredA[getNextLevelAPI(curPath, key.(string))] = true
					}
				}
				nextLevelAPI := cast.ToStringMap(itemContent[PROPERTIES])
				nextLevelAPI[APIPath] = curPath
				queueA = append(queueA, nextLevelAPI)
			default:
				contentAJson := getAPIInfo(contentA)
				contentBJson := getAPIInfo(contentB)
				if contentAJson != contentBJson {
					switch {
					case !maps.Equal(contentA, map[string]any{}) && !maps.Equal(contentB, map[string]any{}):
						tblPrinter.AddRow(nextLevelAPIKey, requiredA[nextLevelAPIKey], printer.BoldYellow(Modified), fmt.Sprintf("%s -> %s", contentAJson, contentBJson))
					case !maps.Equal(contentA, map[string]any{}) && maps.Equal(contentB, map[string]any{}):
						tblPrinter.AddRow(nextLevelAPIKey, requiredA[nextLevelAPIKey], printer.BoldRed(Removed), contentAJson)
					case maps.Equal(contentA, map[string]any{}) && !maps.Equal(contentB, map[string]any{}):
						tblPrinter.AddRow(nextLevelAPIKey, requiredB[nextLevelAPIKey], printer.BoldGreen(Added), contentBJson)
					}
				}
				delete(queueB, nextLevelAPIKey)
			}
		}
	}
	for key := range queueB {
		tblPrinter.AddRow(key, requiredB[key], printer.BoldGreen(Added))
	}
	if tblPrinter.Tbl.Length() != 0 {
		tblPrinter.Print()
		printer.PrintBlankLine(out)
	}
}

func getAPIInfo(api map[string]any) string {
	contentAJson, err := json.Marshal(api)
	if err == nil {
		return string(contentAJson)
	}
	res := "{"
	for i, key := range sortedKeys(api) {
		if i > 0 {
			res += ","
		}
		res += fmt.Sprintf("\"%s\":\"%v\"", key, api[key])
	}
	res += "}"
	return res
}
