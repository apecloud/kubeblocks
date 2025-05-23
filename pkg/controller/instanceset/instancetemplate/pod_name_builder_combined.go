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
	// initialize variables
	allOrdinalSet := sets.New[int32]()
	defaultTemplateUnavailableOrdinalSet := sets.New[int32]()
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

	offlineOrdinals := sets.New[int32]()
	for _, instance := range itsExt.InstanceSet.Spec.OfflineInstances {
		ordinal, err := GetOrdinal(instance)
		if err != nil {
			return nil, err
		}
		offlineOrdinals.Insert(ordinal)
	}

	defaultTemplateUnavailableOrdinalSet = defaultTemplateUnavailableOrdinalSet.Union(offlineOrdinals)
	for _, instanceTemplate := range instanceTemplatesList {
		availableOrdinalSet := ConvertOrdinalsToSet(instanceTemplate.Ordinals)
		defaultTemplateUnavailableOrdinalSet = defaultTemplateUnavailableOrdinalSet.Union(availableOrdinalSet)
	}

	for podName, status := range itsExt.InstanceSet.Status.InstanceStatus {
		ordinal, err := GetOrdinal(podName)
		if err != nil {
			return nil, err
		}
		template2OrdinalSetMap[status.TemplateName].Insert(ordinal)
		allOrdinalSet.Insert(ordinal)
		ordinalToTemplateMap[ordinal] = status.TemplateName
	}

	// main calculation
	// ordinals amount instance templates are guaranteed not to overlap
	for _, instanceTemplate := range instanceTemplatesList {
		currentOrdinalSet := template2OrdinalSetMap[instanceTemplate.Name]
		availableOrdinalSet := ConvertOrdinalsToSet(instanceTemplate.Ordinals)
		ordinalsDefined := false
		if availableOrdinalSet.Len() > 0 {
			ordinalsDefined = true
		}

		if ordinalsDefined {
			// delete any ordinal that doesn't in the availableOrdinalSet
			toDelete := currentOrdinalSet.Difference(availableOrdinalSet)
			for _, ordinal := range toDelete.UnsortedList() {
				currentOrdinalSet.Delete(ordinal)
				allOrdinalSet.Delete(ordinal)
			}
		}

		// delete any offlined ordinals, delete a non-exist item does nothing
		for _, ordinal := range offlineOrdinals.UnsortedList() {
			currentOrdinalSet.Delete(ordinal)
			availableOrdinalSet.Delete(ordinal)
			allOrdinalSet.Delete(ordinal)
		}

		// if currentOrdinalSet is too much, delete some
		if int(*instanceTemplate.Replicas) < len(currentOrdinalSet) {
			// delete from high to low
			l := convertOrdinalSetToSortedList(currentOrdinalSet)
			for i := len(currentOrdinalSet) - 1; i >= int(*instanceTemplate.Replicas); i-- {
				currentOrdinalSet.Delete(l[i])
				allOrdinalSet.Delete(l[i])
			}
			continue
		}

		// if currentOrdinalSet is too little, add some
		addNum := int(*instanceTemplate.Replicas) - len(currentOrdinalSet)
		// for those who define ordinals, use it
		if ordinalsDefined {
			available := convertOrdinalSetToSortedList(availableOrdinalSet)
			if len(available) < addNum {
				return nil, fmt.Errorf("available ordinals (len:%v) are too little (target len: %v)", len(available), addNum)
			}
			for i := 0; i < addNum; i++ {
				currentOrdinalSet.Insert(available[i])
				allOrdinalSet.Insert(available[i])
			}
		} else {
			// for those who do not define ordinals, use a virtual ordinal set which does not overlap with any other templates' ordinals
			var cur int32 = 0
			for i := 0; i < addNum; i++ {
				// find the next available ordinal
				for {
					if !allOrdinalSet.Has(cur) && !defaultTemplateUnavailableOrdinalSet.Has(cur) {
						currentOrdinalSet.Insert(cur)
						allOrdinalSet.Insert(cur)
						break
					}
					cur++
				}
			}
		}

		if currentOrdinalSet.Len() != int(*instanceTemplate.Replicas) {
			return nil, fmt.Errorf("generated ordinals' length (%v) is not equal to template replicas (%v)", currentOrdinalSet.Len(), int(*instanceTemplate.Replicas))
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
	return nil
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
