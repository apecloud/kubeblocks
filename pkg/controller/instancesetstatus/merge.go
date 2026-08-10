/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instancesetstatus

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// Allocation is an authoritative PodName-to-template assignment.
type Allocation struct {
	PodName      string
	TemplateName string
}

// PodObservation is the current state of an actual Pod object.
type PodObservation struct {
	PodName string
	State   workloads.CurrentPodState
}

// RuntimeStatus contains fields that are meaningful only for a current, non-terminating Pod.
type RuntimeStatus struct {
	Role            string
	Configs         []workloads.InstanceConfigStatus
	VolumeExpansion bool
}

// BuildInput contains the independently produced desired and observed dimensions of InstanceStatus.
type BuildInput struct {
	Previous      []workloads.InstanceStatus
	Active        []Allocation
	Offline       []string
	Pods          []PodObservation
	TemplateHints []Allocation
	Runtime       map[string]RuntimeStatus
}

// BuildResult contains a complete, bounded InstanceStatus view.
type BuildResult struct {
	Statuses             []workloads.InstanceStatus
	UnknownTemplateNames []string
}

// Build merges InstanceStatus by PodName. It never carries current runtime fields forward from Previous.
func Build(input BuildInput) (BuildResult, error) {
	previous, err := indexPrevious(input.Previous)
	if err != nil {
		return BuildResult{}, err
	}
	active, err := indexAllocations("active allocation", input.Active, true)
	if err != nil {
		return BuildResult{}, err
	}
	hints, err := indexAllocations("template hint", input.TemplateHints, false)
	if err != nil {
		return BuildResult{}, err
	}
	offline, err := indexNames("offline instance", input.Offline)
	if err != nil {
		return BuildResult{}, err
	}
	for name := range active {
		if offline[name] {
			return BuildResult{}, fmt.Errorf("instance %q is both Active and Offline", name)
		}
	}

	pods := make(map[string]workloads.CurrentPodState, len(input.Pods))
	for _, observation := range input.Pods {
		if observation.PodName == "" {
			return BuildResult{}, fmt.Errorf("pod observation has an empty PodName")
		}
		if _, ok := pods[observation.PodName]; ok {
			return BuildResult{}, fmt.Errorf("duplicate pod observation for %q", observation.PodName)
		}
		if observation.State != workloads.CurrentPodStatePresent && observation.State != workloads.CurrentPodStateTerminating {
			return BuildResult{}, fmt.Errorf("pod observation for %q has invalid state %q", observation.PodName, observation.State)
		}
		pods[observation.PodName] = observation.State
	}

	names := make(map[string]struct{}, len(active)+len(offline)+len(pods))
	for name := range active {
		names[name] = struct{}{}
	}
	for name := range offline {
		names[name] = struct{}{}
	}
	for name := range pods {
		names[name] = struct{}{}
	}

	result := BuildResult{Statuses: make([]workloads.InstanceStatus, 0, len(names))}
	for name := range names {
		status := workloads.InstanceStatus{PodName: name, CurrentPodState: workloads.CurrentPodStateAbsent}
		if state, ok := pods[name]; ok {
			status.CurrentPodState = state
		}

		switch {
		case active[name] != nil:
			status.DesiredState = workloads.InstanceDesiredStateActive
			status.TemplateName = stringPtr(*active[name])
		case offline[name]:
			status.DesiredState = workloads.InstanceDesiredStateOffline
			status.TemplateName = retainedTemplate(name, previous, hints)
		default:
			status.DesiredState = workloads.InstanceDesiredStateReleased
			status.TemplateName = retainedTemplate(name, previous, hints)
		}

		if status.DesiredState != workloads.InstanceDesiredStateActive {
			if old := previous[name]; old != nil && old.TemplateName != nil && status.TemplateName != nil && *old.TemplateName != *status.TemplateName {
				return BuildResult{}, fmt.Errorf("instance %q has conflicting template assignments %q and %q", name, *old.TemplateName, *status.TemplateName)
			}
			if hint := hints[name]; hint != nil && status.TemplateName != nil && *hint != *status.TemplateName {
				return BuildResult{}, fmt.Errorf("instance %q has conflicting template assignments %q and %q", name, *hint, *status.TemplateName)
			}
		}

		if status.TemplateName == nil {
			result.UnknownTemplateNames = append(result.UnknownTemplateNames, name)
		}
		if status.CurrentPodState == workloads.CurrentPodStatePresent {
			if runtime, ok := input.Runtime[name]; ok {
				status.Role = runtime.Role
				status.Configs = copyConfigs(runtime.Configs)
				status.VolumeExpansion = runtime.VolumeExpansion
			}
		}
		result.Statuses = append(result.Statuses, status)
	}

	sortStatuses(result.Statuses)
	sort.Strings(result.UnknownTemplateNames)
	return result, nil
}

func indexPrevious(statuses []workloads.InstanceStatus) (map[string]*workloads.InstanceStatus, error) {
	result := make(map[string]*workloads.InstanceStatus, len(statuses))
	for i := range statuses {
		status := &statuses[i]
		if status.PodName == "" {
			return nil, fmt.Errorf("previous InstanceStatus has an empty PodName")
		}
		if _, ok := result[status.PodName]; ok {
			return nil, fmt.Errorf("duplicate previous InstanceStatus for %q", status.PodName)
		}
		result[status.PodName] = status
	}
	return result, nil
}

func indexAllocations(kind string, allocations []Allocation, rejectDuplicate bool) (map[string]*string, error) {
	result := make(map[string]*string, len(allocations))
	for _, allocation := range allocations {
		if allocation.PodName == "" {
			return nil, fmt.Errorf("%s has an empty PodName", kind)
		}
		if template, ok := result[allocation.PodName]; ok {
			if *template != allocation.TemplateName {
				return nil, fmt.Errorf("instance %q has conflicting %s templates %q and %q", allocation.PodName, kind, *template, allocation.TemplateName)
			}
			if rejectDuplicate {
				return nil, fmt.Errorf("duplicate %s for %q", kind, allocation.PodName)
			}
			continue
		}
		result[allocation.PodName] = stringPtr(allocation.TemplateName)
	}
	return result, nil
}

func indexNames(kind string, names []string) (map[string]bool, error) {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" {
			return nil, fmt.Errorf("%s has an empty PodName", kind)
		}
		if result[name] {
			return nil, fmt.Errorf("duplicate %s %q", kind, name)
		}
		result[name] = true
	}
	return result, nil
}

func retainedTemplate(name string, previous map[string]*workloads.InstanceStatus, hints map[string]*string) *string {
	if old := previous[name]; old != nil && old.TemplateName != nil {
		return stringPtr(*old.TemplateName)
	}
	if hint := hints[name]; hint != nil {
		return stringPtr(*hint)
	}
	return nil
}

func copyConfigs(configs []workloads.InstanceConfigStatus) []workloads.InstanceConfigStatus {
	if configs == nil {
		return nil
	}
	result := make([]workloads.InstanceConfigStatus, len(configs))
	copy(result, configs)
	for i := range result {
		if configs[i].ConfigHash != nil {
			value := *configs[i].ConfigHash
			result[i].ConfigHash = &value
		}
	}
	return result
}

func stringPtr(value string) *string {
	return &value
}

func sortStatuses(statuses []workloads.InstanceStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		leftParent, leftOrdinal := parentAndOrdinal(statuses[i].PodName)
		rightParent, rightOrdinal := parentAndOrdinal(statuses[j].PodName)
		if leftParent != rightParent {
			return leftParent < rightParent
		}
		if leftOrdinal != rightOrdinal {
			return leftOrdinal < rightOrdinal
		}
		return statuses[i].PodName < statuses[j].PodName
	})
}

func parentAndOrdinal(name string) (string, int) {
	index := strings.LastIndex(name, "-")
	if index < 0 {
		return name, -1
	}
	ordinal, err := strconv.Atoi(name[index+1:])
	if err != nil {
		return name, -1
	}
	return name[:index], ordinal
}
