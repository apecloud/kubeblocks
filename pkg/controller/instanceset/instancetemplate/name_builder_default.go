/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package instancetemplate

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

type defaultNameBuilder struct {
	itsExt *InstanceSetExt
}

func (s *defaultNameBuilder) BuildInstanceName2TemplateMap() (map[string]*InstanceTemplateExt, error) {
	instanceTemplateList := buildInstanceTemplateExts(s.itsExt)
	allNameTemplateMap := make(map[string]*InstanceTemplateExt)
	for _, template := range instanceTemplateList {
		ordinalList, err := getOrdinalListByTemplateName(s.itsExt.InstanceSet, template.Name)
		if err != nil {
			return nil, err
		}
		instanceNames, err := generateInstanceNamesFromTemplate(s.itsExt.InstanceSet.Name, template.Name, template.Replicas, s.itsExt.InstanceSet.Spec.OfflineInstances, ordinalList)
		if err != nil {
			return nil, err
		}
		for _, name := range instanceNames {
			allNameTemplateMap[name] = template
		}
	}
	return allNameTemplateMap, nil
}

func (s *defaultNameBuilder) GenerateAllInstanceNames() ([]string, error) {
	instanceNameList := make([]string, 0)
	for _, template := range s.itsExt.InstanceTemplates {
		replicas := template.GetReplicas()
		ordinalList := convertOrdinalsToSortedList(template.GetOrdinals())
		names, err := generateInstanceNamesFromTemplate(s.itsExt.InstanceSet.Name, template.GetName(), replicas, s.itsExt.InstanceSet.Spec.OfflineInstances, ordinalList)
		if err != nil {
			return nil, err
		}
		instanceNameList = append(instanceNameList, names...)
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return parseParentNameAndOrdinal(instanceNameList[i])
	}
	// first sort with template name, then with ordinal
	baseSort(instanceNameList, getNameNOrdinalFunc, nil, true)
	return instanceNameList, nil
}

func (s *defaultNameBuilder) Validate() error {
	// no specific rules needed
	return nil
}

func baseSort(x any, getNameNOrdinalFunc func(i int) (string, int), getRolePriorityFunc func(i int) int, reverse bool) {
	if getRolePriorityFunc == nil {
		getRolePriorityFunc = func(_ int) int {
			return 0
		}
	}
	sort.SliceStable(x, func(i, j int) bool {
		if reverse {
			i, j = j, i
		}
		rolePriI := getRolePriorityFunc(i)
		rolePriJ := getRolePriorityFunc(j)
		if rolePriI != rolePriJ {
			return rolePriI < rolePriJ
		}
		name1, ordinal1 := getNameNOrdinalFunc(i)
		name2, ordinal2 := getNameNOrdinalFunc(j)
		if name1 != name2 {
			return name1 > name2
		}
		return ordinal1 > ordinal2
	})
}

func buildInstanceTemplateExts(itsExt *InstanceSetExt) []*InstanceTemplateExt {
	var instanceTemplateExtList []*InstanceTemplateExt
	for _, template := range itsExt.InstanceTemplates {
		templateExt := buildInstanceTemplateExt(template, itsExt.InstanceSet)
		instanceTemplateExtList = append(instanceTemplateExtList, templateExt)
	}
	return instanceTemplateExtList
}

func getOrdinalListByTemplateName(its *workloads.InstanceSet, templateName string) ([]int32, error) {
	ordinals, err := getOrdinalsByTemplateName(its, templateName)
	if err != nil {
		return nil, err
	}
	return convertOrdinalsToSortedList(ordinals), nil
}

func getOrdinalsByTemplateName(its *workloads.InstanceSet, templateName string) (kbappsv1.Ordinals, error) {
	if templateName == "" {
		return its.Spec.DefaultTemplateOrdinals, nil
	}
	for _, template := range its.Spec.Instances {
		if template.Name == templateName {
			return template.Ordinals, nil
		}
	}
	return kbappsv1.Ordinals{}, fmt.Errorf("template %s not found", templateName)
}

func generateInstanceNamesFromTemplate(parentName, templateName string, replicas int32, offlineInstances []string, ordinalList []int32) ([]string, error) {
	instanceNames, err := generateInstanceNames(parentName, templateName, replicas, 0, offlineInstances, ordinalList)
	return instanceNames, err
}

// generateInstanceNames generates instance names based on certain rules:
// The naming convention for instances (pods) based on the Parent Name, InstanceTemplate Name, and ordinal.
// The constructed instance name follows the pattern: $(parent.name)-$(template.name)-$(ordinal).
func generateInstanceNames(parentName, templateName string,
	replicas int32, ordinal int32, offlineInstances []string, ordinalList []int32) ([]string, error) {
	if len(ordinalList) > 0 {
		return generateInstanceNamesWithOrdinalList(parentName, templateName, replicas, offlineInstances, ordinalList)
	}
	usedNames := sets.New(offlineInstances...)
	var instanceNameList []string
	for count := int32(0); count < replicas; count++ {
		var name string
		for {
			if len(templateName) == 0 {
				name = fmt.Sprintf("%s-%d", parentName, ordinal)
			} else {
				name = fmt.Sprintf("%s-%s-%d", parentName, templateName, ordinal)
			}
			ordinal++
			if !usedNames.Has(name) {
				instanceNameList = append(instanceNameList, name)
				break
			}
		}
	}
	return instanceNameList, nil
}

// generateInstanceNamesWithOrdinalList generates instance names based on ordinalList and offlineInstances.
func generateInstanceNamesWithOrdinalList(parentName, templateName string,
	replicas int32, offlineInstances []string, ordinalList []int32) ([]string, error) {
	var instanceNameList []string
	usedNames := sets.New(offlineInstances...)
	slices.Sort(ordinalList)
	for _, ordinal := range ordinalList {
		if len(instanceNameList) >= int(replicas) {
			break
		}
		var name string
		if len(templateName) == 0 {
			name = fmt.Sprintf("%s-%d", parentName, ordinal)
		} else {
			name = fmt.Sprintf("%s-%s-%d", parentName, templateName, ordinal)
		}
		if usedNames.Has(name) {
			continue
		}
		instanceNameList = append(instanceNameList, name)
	}
	if int32(len(instanceNameList)) != replicas {
		err := fmt.Errorf("for template '%s', expected %d instance names but generated %d: [%s]",
			templateName, replicas, len(instanceNameList), strings.Join(instanceNameList, ", "))
		return instanceNameList, err
	}
	return instanceNameList, nil
}
