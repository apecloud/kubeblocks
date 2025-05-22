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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

type combinedPodNameBuilder struct {
	itsExt *InstanceSetExt
}

func (c *combinedPodNameBuilder) BuildInstanceName2TemplateMap() (map[string]*InstanceTemplateExt, error) {
	template2OrdinalSetMap, err := GenerateTemplateName2OrdinalMap(c.itsExt)
	if err != nil {
		return nil, err
	}

	allNameTemplateMap := make(map[string]*InstanceTemplateExt)
	instanceTemplatesMap := c.itsExt.InstanceTemplates
	for templateName, ordinalSet := range template2OrdinalSetMap {
		tpl := instanceTemplatesMap[templateName]
		tplExt := buildInstanceTemplateExt(tpl, c.itsExt.InstanceSet)
		for ordinal := range ordinalSet {
			instanceName := fmt.Sprintf("%v-%v", c.itsExt.InstanceSet.Name, ordinal)
			allNameTemplateMap[instanceName] = tplExt
		}
	}

	return allNameTemplateMap, nil
}

// GenerateTemplateName2OrdinalMap returns a map from template name to sorted ordinals
// it rely on the instanceset's status to generate desired pod names
// it may not be updated, but it should converge eventually
//
// template ordianls are assumed to be valid at this time
func GenerateTemplateName2OrdinalMap(itsExt *InstanceSetExt) (map[string]sets.Set[int32], error) {
	allOrdinalSet := sets.New[int32]()
	template2OrdinalSetMap := map[string]sets.Set[int32]{}
	ordinalToTemplateMap := map[int32]string{}
	instanceTemplatesList := make([]*workloads.InstanceTemplate, 0, len(itsExt.InstanceTemplates))
	for _, instanceTemplate := range itsExt.InstanceTemplates {
		instanceTemplatesList = append(instanceTemplatesList, instanceTemplate)
		template2OrdinalSetMap[instanceTemplate.Name] = sets.New[int32]()
	}
	slices.SortFunc(instanceTemplatesList, func(a, b *workloads.InstanceTemplate) int {
		return strings.Compare(a.Name, b.Name)
	})

	for podName, status := range itsExt.InstanceSet.Status.InstanceStatus {
		ordinal, err := GetOrdinal(podName)
		if err != nil {
			return nil, err
		}
		template2OrdinalSetMap[status.TemplateName].Insert(ordinal)
		allOrdinalSet.Insert(ordinal)
		ordinalToTemplateMap[ordinal] = status.TemplateName
	}

	// 1. handle those who have ordinals specified
	for _, instanceTemplate := range instanceTemplatesList {
		currentOrdinalSet := template2OrdinalSetMap[instanceTemplate.Name]
		desiredOrdinalSet := ConvertOrdinalsToSet(instanceTemplate.Ordinals)
		if len(desiredOrdinalSet) == 0 {
			continue
		}
		toDelete := currentOrdinalSet.Difference(desiredOrdinalSet)
		toCreate := desiredOrdinalSet.Difference(currentOrdinalSet)
		for _, ordinal := range toDelete.UnsortedList() {
			allOrdinalSet.Delete(ordinal)
			template2OrdinalSetMap[ordinalToTemplateMap[ordinal]].Delete(ordinal)
		}
		for _, ordinal := range toCreate.UnsortedList() {
			if templateName, ok := ordinalToTemplateMap[ordinal]; ok {
				// if the ordinal is already in the current instance, replace it with the new one
				template2OrdinalSetMap[templateName].Delete(ordinal)
			}
			template2OrdinalSetMap[instanceTemplate.Name].Insert(ordinal)
			allOrdinalSet.Insert(ordinal)
		}
	}

	offlineOrdinals := sets.New[int32]()
	for _, instance := range itsExt.InstanceSet.Spec.OfflineInstances {
		ordinal, err := GetOrdinal(instance)
		if err != nil {
			return nil, err
		}
		offlineOrdinals.Insert(ordinal)
	}
	// 2. handle those who have decreased replicas
	for _, instanceTemplate := range instanceTemplatesList {
		currentOrdinals := template2OrdinalSetMap[instanceTemplate.Name]
		if toOffline := currentOrdinals.Intersection(offlineOrdinals); toOffline.Len() > 0 {
			for ordinal := range toOffline {
				allOrdinalSet.Delete(ordinal)
				template2OrdinalSetMap[instanceTemplate.Name].Delete(ordinal)
			}
		}
		// replicas must be non-nil
		if int(*instanceTemplate.Replicas) < len(currentOrdinals) {
			// delete in the name set from high to low
			l := convertOrdinalSetToSortedList(currentOrdinals)
			for i := len(currentOrdinals) - 1; i >= int(*instanceTemplate.Replicas); i-- {
				allOrdinalSet.Delete(l[i])
				template2OrdinalSetMap[instanceTemplate.Name].Delete(l[i])
			}
		}
	}

	// 3. handle those who have increased replicas
	var cur int32 = 0
	for _, instanceTemplate := range instanceTemplatesList {
		currentOrdinals := template2OrdinalSetMap[instanceTemplate.Name]
		for i := len(currentOrdinals); i < int(*instanceTemplate.Replicas); i++ {
			// find the next available ordinal
			for {
				if !allOrdinalSet.Has(cur) && !offlineOrdinals.Has(cur) {
					allOrdinalSet.Insert(cur)
					template2OrdinalSetMap[instanceTemplate.Name].Insert(cur)
					break
				}
				cur++
			}
		}
	}

	return template2OrdinalSetMap, nil
}

func (c *combinedPodNameBuilder) GenerateAllInstanceNames() ([]string, error) {
	template2OrdinalSetMap, err := GenerateTemplateName2OrdinalMap(c.itsExt)
	if err != nil {
		return nil, err
	}
	allOrdinalSet := sets.New[int32]()
	for _, ordinalSet := range template2OrdinalSetMap {
		allOrdinalSet = allOrdinalSet.Union(ordinalSet)
	}
	instanceNames := make([]string, 0, len(allOrdinalSet))
	allOrdinalList := convertOrdinalSetToSortedList(allOrdinalSet)
	for _, ordinal := range allOrdinalList {
		instanceNames = append(instanceNames, fmt.Sprintf("%v-%v", c.itsExt.InstanceSet.Name, ordinal))
	}
	return instanceNames, nil
}

// Validate checks if the instanceset spec is valid
// Ordinals should be unique globally.
func (c *combinedPodNameBuilder) Validate() error {
	ordinalSet := sets.New[int32]()
	offlineOrdinals := sets.New[int32]()
	for _, name := range c.itsExt.InstanceSet.Spec.OfflineInstances {
		ordinal, err := GetOrdinal(name)
		if err != nil {
			return err
		}
		offlineOrdinals.Insert(ordinal)
	}
	for _, tmpl := range c.itsExt.InstanceTemplates {
		ordinals := tmpl.Ordinals
		tmplOrdinalSet := sets.New[int32]()
		for _, item := range ordinals.Discrete {
			if item < 0 {
				return fmt.Errorf("ordinal(%v) must >= 0", item)
			}
			if ordinalSet.Has(item) {
				return fmt.Errorf("duplicate ordinal(%v)", item)
			}
			ordinalSet.Insert(item)
			tmplOrdinalSet.Insert(item)
		}

		for _, item := range ordinals.Ranges {
			start := item.Start
			end := item.End

			if start < 0 {
				return fmt.Errorf("ordinal's start(%v) must >= 0", start)
			}

			if start > end {
				return fmt.Errorf("range's end(%v) must >= start(%v)", end, start)
			}

			for ordinal := start; ordinal <= end; ordinal++ {
				if ordinalSet.Has(ordinal) {
					return fmt.Errorf("duplicate ordinal(%v)", item)
				}
				ordinalSet.Insert(ordinal)
				tmplOrdinalSet.Insert(ordinal)
			}
		}
		if (ordinals.Ranges != nil || ordinals.Discrete != nil) && tmplOrdinalSet.Len() < int(*tmpl.Replicas) {
			return fmt.Errorf("template(%v) has length of ordinals < replicas", tmpl.Name)
		}
	}
	_, err := c.GenerateAllInstanceNames()
	return err
}

// SetInstanceStatus sets template name in InstanceStatus
func (c *combinedPodNameBuilder) SetInstanceStatus(pods []*corev1.Pod) error {
	instanceStatus := c.itsExt.InstanceSet.Status.InstanceStatus
	for _, pod := range pods {
		templateName, ok := pod.Labels[TemplateNameLabelKey]
		if !ok {
			return fmt.Errorf("unknown pod %v", klog.KObj(pod))
		}
		status := instanceStatus[pod.Name]
		status.TemplateName = templateName
		instanceStatus[pod.Name] = status
	}
	return nil
}
