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

// ErrAssignmentIncomplete indicates a temporary partial view while workload reconciliation is releasing or
// moving instance names. Callers must not publish the partial view.
var ErrAssignmentIncomplete = errors.New("instance assignment is incomplete")

// IsAssignmentIncomplete reports whether err represents a temporary partial assignment.
func IsAssignmentIncomplete(err error) bool {
	return errors.Is(err, ErrAssignmentIncomplete)
}

// Assignment associates an instance identity with its authoritative template.
type Assignment struct {
	InstanceName string
	TemplateName string
}

// BuildAssignments obtains the authoritative name-to-template assignments for identities that should have Pods.
func BuildAssignments(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) ([]Assignment, []string, error) {
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
			return nil, nil, fmt.Errorf("%w: expected %d names, got %d", ErrAssignmentIncomplete, expected, len(nameMap))
		}
		return nil, nil, fmt.Errorf("incomplete instance assignment: expected %d names, got %d", expected, len(nameMap))
	}

	assignments := make([]Assignment, 0, len(nameMap))
	for name, template := range nameMap {
		if template == nil {
			return nil, nil, fmt.Errorf("instance %q has no authoritative template assignment", name)
		}
		assignments = append(assignments, Assignment{InstanceName: name, TemplateName: template.Name})
	}
	sort.Slice(assignments, func(i, j int) bool { return assignments[i].InstanceName < assignments[j].InstanceName })

	templateNames := make([]string, 0, len(itsExt.InstanceTemplates)+1)
	templateNames = append(templateNames, DefaultTemplateName)
	for name := range itsExt.InstanceTemplates {
		if name != DefaultTemplateName {
			templateNames = append(templateNames, name)
		}
	}
	sort.Strings(templateNames)
	return assignments, templateNames, nil
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

// ResolveHistoricalTemplate resolves an old retained instance only from explicit allocation state or unambiguous
// non-flat naming.
func ResolveHistoricalTemplate(its *workloads.InstanceSet, instanceName string, knownTemplateNames []string) (string, bool, error) {
	parent, ordinal, ok := parseInstanceName(instanceName)
	if !ok {
		return "", false, nil
	}
	if its.Spec.FlatInstanceOrdinal {
		// A flat name does not encode its template. Resolve only from explicit ordinal ownership and reject
		// conflicting owners instead of guessing from the instance name.
		var found *string
		setFound := func(templateName string) error {
			if found != nil && *found != templateName {
				return fmt.Errorf("instance %q has conflicting ordinal templates %q and %q", instanceName, *found, templateName)
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
