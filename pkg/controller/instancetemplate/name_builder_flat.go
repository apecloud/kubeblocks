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
	"errors"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// ErrOrdinalsNotEnough is considered temporary, e.g. some old ordinals are being deleted
var ErrOrdinalsNotEnough = errors.New("available ordinals are not enough")

type flatNameBuilder struct {
	itsExt      *InstanceSetExt
	eventLogger record.EventRecorder
}

func (c *flatNameBuilder) BuildInstanceName2TemplateMap() (map[string]*InstanceTemplateExt, error) {
	template2OrdinalSetMap, err := generateTemplateName2OrdinalMap(c.itsExt)
	if err != nil {
		if errors.Is(err, ErrOrdinalsNotEnough) {
			if c.eventLogger != nil {
				c.eventLogger.Event(c.itsExt.InstanceSet, corev1.EventTypeWarning, "OrdinalsNotEnough", err.Error())
			}
		} else {
			return nil, err
		}
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

func (c *flatNameBuilder) GenerateAllInstanceNames() ([]string, error) {
	template2OrdinalSetMap, err := generateTemplateName2OrdinalMap(c.itsExt)
	if err != nil {
		if errors.Is(err, ErrOrdinalsNotEnough) {
			if c.eventLogger != nil {
				c.eventLogger.Event(c.itsExt.InstanceSet, corev1.EventTypeWarning, "OrdinalsNotEnough", err.Error())
			}
		} else {
			return nil, err
		}
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

// Validate checks if the instance set spec is valid
// Ordinals should be unique globally.
func (c *flatNameBuilder) Validate() error {
	ordinals := sets.New[int32]()
	offlineOrdinals := sets.New[int32]()
	for _, instance := range c.itsExt.InstanceSet.Spec.OfflineInstances {
		ordinal, err := getOrdinal(instance)
		if err != nil {
			return err
		}
		offlineOrdinals.Insert(ordinal)
	}

	for _, tpl := range c.itsExt.InstanceTemplates {
		tplOrdinals := sets.New[int32]()
		check := func(ordinal int32) error {
			if ordinals.Has(ordinal) && !tplOrdinals.Has(ordinal) {
				return fmt.Errorf("duplicate ordinal(%v)", ordinal)
			}
			ordinals.Insert(ordinal)
			tplOrdinals.Insert(ordinal)
			return nil
		}
		for _, item := range tpl.Ordinals.Discrete {
			if item < 0 {
				return fmt.Errorf("ordinal(%v) must >= 0", item)
			}
			if err := check(item); err != nil {
				return err
			}
		}
		for _, item := range tpl.Ordinals.Ranges {
			start, end := item.Start, item.End
			if start < 0 {
				return fmt.Errorf("ordinal's start(%v) must >= 0", start)
			}
			if start > end {
				return fmt.Errorf("range's end(%v) must >= start(%v)", end, start)
			}
			for ordinal := start; ordinal <= end; ordinal++ {
				if err := check(ordinal); err != nil {
					return err
				}
			}
		}
		available := tplOrdinals.Difference(offlineOrdinals)
		if (tpl.Ordinals.Ranges != nil || tpl.Ordinals.Discrete != nil) && available.Len() < int(*tpl.Replicas) {
			return fmt.Errorf("template(%v) has available ordinals less than replicas", tpl.Name)
		}
	}
	return nil
}

// generateTemplateName2OrdinalMap returns a map from template name to sorted ordinals
// it relies on the instance set's status to generate desired pod names
// it may not be updated, but it should converge eventually
//
// template ordinals are assumed to be valid at this time
func generateTemplateName2OrdinalMap(itsExt *InstanceSetExt) (map[string]sets.Set[int32], error) {
	// globalUsedOrdinalSet won't decrease, so that one ordinal couldn't suddenly change its template
	globalUsedOrdinalSet := sets.New[int32]()
	defaultTemplateUnavailableOrdinalSet := sets.New[int32]()
	template2OrdinalSetMap := map[string]sets.Set[int32]{}
	instanceTemplatesList := make([]*workloads.InstanceTemplate, 0, len(itsExt.InstanceTemplates))
	for _, instanceTemplate := range itsExt.InstanceTemplates {
		instanceTemplatesList = append(instanceTemplatesList, instanceTemplate)
		template2OrdinalSetMap[instanceTemplate.Name] = sets.New[int32]()
	}
	slices.SortFunc(instanceTemplatesList, func(a, b *workloads.InstanceTemplate) int {
		return strings.Compare(a.Name, b.Name)
	})

	offlineOrdinalSet := sets.New[int32]()
	for _, instance := range itsExt.InstanceSet.Spec.OfflineInstances {
		ordinal, err := getOrdinal(instance)
		if err != nil {
			return nil, err
		}
		offlineOrdinalSet.Insert(ordinal)
	}

	defaultTemplateUnavailableOrdinalSet = defaultTemplateUnavailableOrdinalSet.Union(offlineOrdinalSet)
	for _, instanceTemplate := range instanceTemplatesList {
		availableOrdinalSet := convertOrdinalsToSet(instanceTemplate.Ordinals)
		defaultTemplateUnavailableOrdinalSet = defaultTemplateUnavailableOrdinalSet.Union(availableOrdinalSet)
	}

	if _, ok := template2OrdinalSetMap[""]; ok {
		template2OrdinalSetMap[""].Insert(itsExt.InstanceSet.Status.Ordinals...)
	}
	globalUsedOrdinalSet.Insert(itsExt.InstanceSet.Status.Ordinals...)
	for _, status := range itsExt.InstanceSet.Status.TemplatesStatus {
		if _, ok := template2OrdinalSetMap[status.Name]; ok {
			template2OrdinalSetMap[status.Name].Insert(status.Ordinals...)
		}
		globalUsedOrdinalSet.Insert(status.Ordinals...)
	}

	generateWithOrdinalsDefined := func(
		current, available sets.Set[int32], instanceTemplate *workloads.InstanceTemplate,
	) (sets.Set[int32], error) {
		// delete any ordinal that doesn't in the available
		current = current.Intersection(available)

		// delete any offline ordinals
		current = current.Difference(offlineOrdinalSet)
		available = available.Difference(offlineOrdinalSet)

		diff := int(*instanceTemplate.Replicas) - len(current)
		if diff < 0 {
			// delete from high to low
			l := convertOrdinalSetToSortedList(current)
			for i := len(current) - 1; i >= int(*instanceTemplate.Replicas); i-- {
				current.Delete(l[i])
			}
			return current, nil
		}

		// if current is too little, add some
		availableWithoutCurrent := available.Difference(current)
		availableList := convertOrdinalSetToSortedList(availableWithoutCurrent)
		cur := 0
		for i := 0; i < diff; i++ {
			for {
				if cur >= len(availableWithoutCurrent) {
					return current, ErrOrdinalsNotEnough
				}
				ordinal := availableList[cur]
				if !globalUsedOrdinalSet.Has(ordinal) || (!current.Has(ordinal) && template2OrdinalSetMap[""].Has(ordinal)) {
					globalUsedOrdinalSet.Insert(ordinal)
					current.Insert(ordinal)
					break
				}
				cur++
			}
		}
		return current, nil
	}

	generateWithoutOrdinalsDefined := func(
		current sets.Set[int32], instanceTemplate *workloads.InstanceTemplate,
	) (sets.Set[int32], error) {
		// delete any ordinal that is taken by an template with ordinals defined
		current = current.Difference(defaultTemplateUnavailableOrdinalSet)
		// delete any offline ordinals
		current = current.Difference(offlineOrdinalSet)

		diff := int(*instanceTemplate.Replicas) - len(current)
		if diff < 0 {
			// delete from high to low
			l := convertOrdinalSetToSortedList(current)
			for i := len(current) - 1; i >= int(*instanceTemplate.Replicas); i-- {
				current.Delete(l[i])
			}
			return current, nil
		}

		// for those who do not define ordinals, use a virtual ordinal set which does not overlap with any other templates' ordinals
		var cur int32 = 0
		for i := 0; i < diff; i++ {
			// find the next available ordinal
			for {
				if !globalUsedOrdinalSet.Has(cur) && !defaultTemplateUnavailableOrdinalSet.Has(cur) {
					current.Insert(cur)
					globalUsedOrdinalSet.Insert(cur)
					break
				}
				cur++
			}
		}

		return current, nil
	}

	// ordinals amount instance templates are guaranteed not to overlap
	hasErrOrdinalsNotEnough := false
	template2Ordinals := map[string]sets.Set[int32]{}
	for _, instanceTemplate := range instanceTemplatesList {
		currentOrdinalSet := template2OrdinalSetMap[instanceTemplate.Name]
		availableOrdinalSet := convertOrdinalsToSet(instanceTemplate.Ordinals)
		var err error
		if availableOrdinalSet.Len() > 0 {
			currentOrdinalSet, err = generateWithOrdinalsDefined(currentOrdinalSet, availableOrdinalSet, instanceTemplate)
		} else {
			currentOrdinalSet, err = generateWithoutOrdinalsDefined(currentOrdinalSet, instanceTemplate)
		}

		// ignore ErrOrdinalsNotEnough
		if err != nil {
			if errors.Is(err, ErrOrdinalsNotEnough) {
				hasErrOrdinalsNotEnough = true
			} else {
				return nil, err
			}
		}

		template2Ordinals[instanceTemplate.Name] = currentOrdinalSet
	}

	var err error
	if hasErrOrdinalsNotEnough {
		err = ErrOrdinalsNotEnough
	}
	return template2Ordinals, err
}
