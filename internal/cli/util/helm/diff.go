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
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/spf13/cast"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
)

const k8sCRD = "CustomResourceDefinition"
const TYPE = "type"
const PROPERTIES = "properties"
const ADDITIONALPROPERTIES = "additionalProperties"
const REQUIRED = "required"

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
	//OpenAPIV3Schema map[string]interface{} `yaml:"openAPIV3Schema"`
}

const (
	boolean string = "boolean"
	object  string = "object"
	str     string = "string"
	array   string = "array"
	integer string = "integer"
)

type Mode string

const (
	Modified Mode = "Modified"
	Add      Mode = "Add"
	Remove   Mode = "Remove"
)

type apiInfo struct {
	name string
	//description string
	isRequired   bool
	defaultValue any
	pattern      string
	apiType      string
	apiMode      Mode
}

//func (info *apiInfo) toString() string {
//	return fmt.Sprintf("api: %s description: %s isRequired: %v\n", info.name, info.description, info.isRequired)
//}

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
		if err := yaml.Unmarshal([]byte(content), &data); err != nil {
			return nil, err
		}
		// it‘s dangerous ！
		openAPIV3Schema := cast.ToStringMap(cast.ToStringMap(cast.ToStringMap(cast.ToSlice(cast.ToStringMap(data["spec"])["versions"])[0])["schema"])["openAPIV3Schema"])[PROPERTIES]
		//fmt.Printf("%v", stringMap)
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
		if k8sCRD == parsedMetadata.Kind {
			return parseOpenAPIV3Schema()
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
// todo: do not use GetUnifiedDiffString but have a clear way ,for
func OutputDiff(releaseA *release.Release, releaseB *release.Release, versionA, versionB string, out io.Writer) error {
	detail := false
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

	//mayAddAPI := make(map[string]map[string]*apiInfo)
	//mayRemoveAPI := make(map[string]map[string]*apiInfo)
	//var mayAddAPI []string
	var mayRemoveAPI []string
	var mayAddAPI []string
	for _, key := range sortedKeys(manifestsMapA) {
		//recordA := make(map[string]*apiInfo)
		//recordB := make(map[string]*apiInfo)
		manifestA := manifestsMapA[key]
		if manifestA.Kind != k8sCRD {
			continue
		}
		apiContentsA := make(map[string]any)
		err := yaml.Unmarshal([]byte(manifestA.Content), &apiContentsA)
		if err != nil {
			return err
		}
		//getPropertyInfo(data, []string{}, recordA)
		if manifestB, ok := manifestsMapB[key]; ok {
			if manifestA.Content == manifestB.Content {
				continue
			}
			apiContentsB := make(map[string]any)
			err := yaml.Unmarshal([]byte(manifestB.Content), &apiContentsB)
			if err != nil {
				return err
			}
			outputAPIDiff(apiContentsA, apiContentsB, key, out)
			//getPropertyInfo(data, []string{}, recordB)
			//outputApiDiff(recordA, recordB, name, out)
		} else {
			//mayRemove = append(mayRemove, manifestA)
			//mayRemoveAPI[name] = recordA
			mayRemoveAPI = append(mayRemoveAPI, manifestA.Name)
		}
	}

	for _, key := range sortedKeys(manifestsMapB) {
		manifestB := manifestsMapB[key]
		if manifestB.Kind != k8sCRD {
			continue
		}
		if _, ok := manifestsMapA[key]; !ok {
			mayAddAPI = append(mayAddAPI, manifestB.Name)
		}
	}
	tblPrinter := printer.NewTablePrinter(out)
	tblPrinter.SetHeader("CRD", "MODE")
	sort.Strings(mayRemoveAPI)
	sort.Strings(mayAddAPI)

	for i := range mayRemoveAPI {
		tblPrinter.AddRow(mayRemoveAPI[i], printer.BoldRed(Remove))
	}
	printer.PrintBlankLine(out)
	for i := range mayAddAPI {
		tblPrinter.AddRow(mayAddAPI[i], printer.BoldGreen(Add))
	}
	// Todo: support find Rename chart.yaml between mayRemove and mayAdd
	//for _, name := range sortedKeys(manifestsMapB) {
	//	manifestB := manifestsMapB[name]
	//	if manifestB.Kind != k8sCRD {
	//		continue
	//	}
	//	if _, ok := manifestsMapB[name]; !ok {
	//		data := make(map[string]any)
	//		err := yaml.Unmarshal([]byte(manifestB.Content), &data)
	//		if err != nil {
	//			return err
	//		}
	//		recordB := make(map[string]*apiInfo)
	//		getPropertyInfo(data, []string{}, recordB)
	//		mayAddAPI[name] = recordB
	//	}
	//}
	//
	//for name, val := range mayAddAPI {
	//	outputApiDiff(nil, val, name, out)
	//	//diffString, err := util.GetUnifiedDiffString("", elem.Content, "", fmt.Sprintf("%s %s", elem.Name, versionB), 1)
	//	//if err != nil {
	//	//	return err
	//	//}
	//	//util.DisplayDiffWithColor(out, diffString)
	//}
	//
	//for name, val := range mayRemoveAPI {
	//	//diffString, err := util.GetUnifiedDiffString(elem.Content, "", fmt.Sprintf("%s %s", elem.Name, versionA), "", 1)
	//	//if err != nil {
	//	//	return err
	//	//}
	//	//util.DisplayDiffWithColor(out, diffString)
	//	outputApiDiff(val, nil, name, out)
	//}

	if !detail {
		return nil
	}
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
		// if mapResult == nil maybe something wrong
		// todo: fix what make ParseContent return a nil
		if mapResult == nil {
			//fmt.Printf(printer.BoldYellow("Warn:")+" %v release content is empty, something maybe wrong\n", v)
			continue
		}
		manifestsMap[mapResult.Name] = mapResult
	}
	return manifestsMap, nil
}

// sortedKeys return sorted keys of manifests
func sortedKeys(manifests map[string]*MappingResult) []string {
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

//func getPropertyInfo(content map[string]any, path []string, record map[string]*apiInfo) {
//	if content == nil {
//		return
//	}
//	if content["type"] != "object" && content["type"] != "array" {
//		fillTheApi(content, strings.Join(path, "."), record)
//		return
//	}
//	switch content["type"] {
//	case "object":
//		properties := cast.ToStringMap(content["properties"])
//		for name, val := range properties {
//			getPropertyInfo(cast.ToStringMap(val), append(path, name), record)
//		}
//		//name := strings.Join(path, ".")
//		//fillTheApi(content, name, record)
//	case "array":
//		items := cast.ToStringMap(content["items"])
//		for name, val := range items {
//			getPropertyInfo(cast.ToStringMap(val), append(path, name), record)
//		}
//		//name := strings.Join(path, ".")
//		//fillTheApi(content, name, record)
//	}
//
//	return
//}

//func fillTheApi(content map[string]any, name string, record map[string]*apiInfo) {
//	if len(name) == 0 {
//		return
//	}
//	add := &apiInfo{
//		name: name,
//	}
//	if content["description"] != nil {
//		if description, ok := content["description"].(string); ok {
//			add.description = description
//		}
//	}
//	if content["required"] != nil {
//		slice := cast.ToSlice(content["required"])
//		for i := range slice {
//			temp := record[name+"."+slice[i].(string)]
//			if temp != nil {
//				temp.isRequired = true
//			}
//		}
//	}
//	record[name] = add
//}

// outputApiDiff out the different between releaseA's API and releaseB's API, typically releaseA is older version
//func outputApiDiff(releaseA map[string]*apiInfo, releaseB map[string]*apiInfo, CRD string, out io.Writer) {
//	fmt.Fprintf(out, printer.BoldYellow(fmt.Sprintf("\nCustomResourceDefinition %s's API Modification:\n", CRD)))
//	var add []*apiInfo
//	var remove []*apiInfo
//	for name, val := range releaseA {
//		if _, ok := releaseB[name]; ok {
//			if *val != *releaseB[name] {
//				fmt.Fprintf(out, printer.BoldRed(val.toString()))
//				fmt.Fprintf(out, printer.BoldGreen(releaseB[name].toString()))
//			}
//		} else {
//			remove = append(remove, val)
//		}
//	}
//	for name, val := range releaseB {
//		if _, ok := releaseA[name]; !ok {
//			add = append(add, val)
//		}
//	}
//	for i := range add {
//		fmt.Fprintf(out, printer.BoldGreen(add[i].toString()))
//	}
//	for i := range remove {
//		fmt.Fprintf(out, printer.BoldRed(remove[i].toString()))
//	}
//}

const APIPath = "KB-API-PATH"

func outputAPIDiff(A, B map[string]any, CRD string, out io.Writer) {
	fmt.Fprintf(out, fmt.Sprintf("%s\n", printer.BoldYellow(CRD)))
	tblPrinter := printer.NewTablePrinter(out)
	tblPrinter.AddRow("API", "IS-REQUIRED", "DETAILS")
	getNextLevelAPI := func(curPath, key string) string {
		if len(curPath) == 0 {
			return key
		}
		return curPath + "." + key
	}
	//queueA := make(,0)
	//var printRes []*apiInfo
	A[APIPath] = ""
	B[APIPath] = ""
	var queueA []map[string]any = []map[string]any{A}
	queueB := make(map[string]map[string]any)
	requiredA := make(map[string]bool) // to remember requiredAPI
	requiredB := make(map[string]bool)
	queueB[""] = B

	for len(queueA) > 0 {
		curA := queueA[0]
		queueA = queueA[1:]
		curAPath := curA[APIPath].(string)
		//if len(queueB) == 0 {
		//	printRes = append(printRes, getAPIInfo(curA))
		//}
		curB := queueB[curAPath]
		if curB == nil {
			// A have API but B do not have
			//printRes = append(printRes, getAPIInfo(curA, curAPath, Remove))
			//contentAJson, _ := json.Marshal(curA)
			tblPrinter.AddRow(curAPath, requiredA[curAPath], printer.BoldRed(Remove))
			continue
		}
		delete(queueB, curAPath)
		// add Content B
		for key, val := range curB {
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
			}
		}

		// check api if equal and add next level api
		for key, val := range curA {
			contentA := cast.ToStringMap(val)
			nextLevelAPIKey := getNextLevelAPI(curAPath, key)
			contentB := cast.ToStringMap(curB[nextLevelAPIKey])

			delete(contentA, "description")
			delete(contentB, "description")
			if slice := cast.ToSlice(contentA[REQUIRED]); slice != nil {
				for _, key := range slice {
					requiredA[getNextLevelAPI(nextLevelAPIKey, key.(string))] = true
				}
			}
			// compare contentA and contentB rules by different Type
			if requiredA[nextLevelAPIKey] != requiredB[nextLevelAPIKey] {
				tblPrinter.AddRow(nextLevelAPIKey, requiredA[nextLevelAPIKey], printer.BoldRed(Remove))
				tblPrinter.AddRow(nextLevelAPIKey, requiredB[nextLevelAPIKey], printer.BoldGreen(Add))

				//printRes = append(printRes, getAPIInfo(contentA, nextLevelAPIKey, Remove))
				//printRes = append(printRes, getAPIInfo(contentB, nextLevelAPIKey, Add))
			}
			switch t, _ := contentB[TYPE].(string); t {
			case object:
				// compare object , check required
				nextLevelAPI := cast.ToStringMap(contentA[PROPERTIES])
				nextLevelAPI[APIPath] = nextLevelAPIKey
				queueA = append(queueA, nextLevelAPI)
			case array:
				//
				itemContent := cast.ToStringMap(contentA["items"])
				curPath := getNextLevelAPI(nextLevelAPIKey, "items")
				if slice := cast.ToSlice(itemContent[REQUIRED]); slice != nil {
					for _, key := range slice {
						requiredA[getNextLevelAPI(curPath, key.(string))] = true
					}
				}
				//queueB[nextLevelAPIKey] = cast.ToStringMap(itemContent[PROPERTIES])
				nextLevelAPI := cast.ToStringMap(itemContent[PROPERTIES])
				nextLevelAPI[APIPath] = curPath
				queueA = append(queueA, nextLevelAPI)
			default:
				contentAJson, _ := json.Marshal(contentA)
				contentBJson, _ := json.Marshal(contentB)
				if string(contentAJson) != string(contentBJson) {
					if !maps.Equal(contentA, map[string]any{}) {
						tblPrinter.AddRow(nextLevelAPIKey, requiredA[nextLevelAPIKey], printer.BoldRed(Remove))
					}
					if !maps.Equal(contentB, map[string]any{}) {
						tblPrinter.AddRow(nextLevelAPIKey, requiredB[nextLevelAPIKey], printer.BoldGreen(Add))
					}
					//printRes = append(printRes, getAPIInfo(contentA, nextLevelAPIKey, Remove))
					//printRes = append(printRes, getAPIInfo(contentB, nextLevelAPIKey, Add))
				}
			}
		}
	}

	for key, _ := range queueB {
		//printRes = append(printRes, getAPIInfo(val, key, Remove))
		tblPrinter.AddRow(key, requiredB[key], printer.BoldGreen(Add))
	}
	tblPrinter.Print()
	printer.PrintBlankLine(out)
}

func isEmpty(a map[string]interface{}) bool {
	return maps.Equal(a, map[string]any{})
}
