/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
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
	"sort"
	"strconv"
	"strings"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

// ErrActiveAllocationIncomplete indicates a temporary partial view while workload reconciliation is releasing or
// moving instance names. Callers must not publish the partial view.
var ErrActiveAllocationIncomplete = errors.New("active instance allocation is incomplete")

// IsActiveAllocationIncomplete reports whether err represents a temporary partial allocation.
func IsActiveAllocationIncomplete(err error) bool {
	return errors.Is(err, ErrActiveAllocationIncomplete)
}

// InstanceAllocation is an authoritative PodName-to-template assignment.
type InstanceAllocation struct {
	PodName      string
	TemplateName string
}

// BuildActiveAllocations obtains the authoritative active name-to-template view used by InstanceSet reconciliation.
func BuildActiveAllocations(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) ([]InstanceAllocation, []string, error) {
	itsExt, err := BuildInstanceSetExt(its, tree)
	if err != nil {
		return nil, nil, err
	}
	builder, err := NewPodNameBuilder(itsExt, nil)
	if err != nil {
		return nil, nil, err
	}
	if err := builder.Validate(); err != nil {
		return nil, nil, err
	}
	nameMap, err := builder.BuildInstanceName2TemplateMap()
	if err != nil {
		return nil, nil, err
	}

	expected := int32(1)
	if its.Spec.Replicas != nil {
		expected = *its.Spec.Replicas
	}
	if len(nameMap) != int(expected) {
		if its.Spec.FlatInstanceOrdinal {
			return nil, nil, fmt.Errorf("%w: expected %d names, got %d", ErrActiveAllocationIncomplete, expected, len(nameMap))
		}
		return nil, nil, fmt.Errorf("incomplete active instance allocation: expected %d names, got %d", expected, len(nameMap))
	}

	allocations := make([]InstanceAllocation, 0, len(nameMap))
	for name, template := range nameMap {
		if template == nil {
			return nil, nil, fmt.Errorf("active instance %q has no authoritative template", name)
		}
		allocations = append(allocations, InstanceAllocation{PodName: name, TemplateName: template.Name})
	}
	sort.Slice(allocations, func(i, j int) bool { return allocations[i].PodName < allocations[j].PodName })

	templateNames := make([]string, 0, len(itsExt.InstanceTemplates)+1)
	templateNames = append(templateNames, DefaultTemplateName)
	for name := range itsExt.InstanceTemplates {
		if name != DefaultTemplateName {
			templateNames = append(templateNames, name)
		}
	}
	sort.Strings(templateNames)
	return allocations, templateNames, nil
}

// TemplateNameFromLabels returns an explicitly published template label, preserving the empty default template value.
func TemplateNameFromLabels(labels map[string]string) (string, bool) {
	if labels == nil {
		return "", false
	}
	if templateName, ok := labels[constant.KBAppInstanceTemplateLabelKey]; ok {
		return templateName, true
	}
	if templateName, ok := labels[TemplateNameLabelKey]; ok {
		return templateName, true
	}
	return "", false
}

// HistoricalTemplateHint resolves an old retained instance only from explicit allocation state or unambiguous
// non-flat naming.
func HistoricalTemplateHint(its *workloads.InstanceSet, podName string, knownTemplateNames []string) (string, bool, error) {
	parent, ordinal, ok := parseInstanceName(podName)
	if !ok {
		return "", false, nil
	}
	if its.Spec.FlatInstanceOrdinal {
		var found *string
		setFound := func(templateName string) error {
			if found != nil && *found != templateName {
				return fmt.Errorf("instance %q has conflicting ordinal templates %q and %q", podName, *found, templateName)
			}
			value := templateName
			found = &value
			return nil
		}
		for templateName, ordinals := range its.Status.AssignedOrdinals {
			for _, candidate := range ordinals.Discrete {
				if candidate != int32(ordinal) {
					continue
				}
				if err := setFound(templateName); err != nil {
					return "", false, err
				}
			}
		}
		if ordinalIsExplicit(its.Spec.Ordinals, int32(ordinal)) {
			if err := setFound(""); err != nil {
				return "", false, err
			}
		}
		for _, template := range its.Spec.Instances {
			if ordinalIsExplicit(template.Ordinals, int32(ordinal)) {
				if err := setFound(template.Name); err != nil {
					return "", false, err
				}
			}
		}
		if found != nil {
			return *found, true, nil
		}
		return "", false, nil
	}

	for _, templateName := range knownTemplateNames {
		expectedParent := its.Name
		if templateName != "" {
			expectedParent += "-" + templateName
		}
		if parent == expectedParent {
			return templateName, true, nil
		}
	}
	return "", false, nil
}

func ordinalIsExplicit(ordinals workloads.Ordinals, ordinal int32) bool {
	for _, candidate := range ordinals.Discrete {
		if candidate == ordinal {
			return true
		}
	}
	for _, ordinalRange := range ordinals.Ranges {
		if ordinal >= ordinalRange.Start && ordinal <= ordinalRange.End {
			return true
		}
	}
	return false
}

func parseInstanceName(name string) (string, int, bool) {
	index := strings.LastIndex(name, "-")
	if index < 0 {
		return "", 0, false
	}
	ordinal, err := strconv.Atoi(name[index+1:])
	if err != nil {
		return "", 0, false
	}
	return name[:index], ordinal, true
}
