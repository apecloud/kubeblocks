/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

package operations

import (
	"fmt"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func validateInstanceIdentities(statuses []workloadsv1.InstanceStatus) error {
	names := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		if status.PodName == "" {
			return fmt.Errorf("InstanceSet published an empty instance identity")
		}
		if _, ok := names[status.PodName]; ok {
			return fmt.Errorf("InstanceSet published duplicate instance identity %q", status.PodName)
		}
		names[status.PodName] = struct{}{}
	}
	return nil
}

func activeInstanceTemplates(statuses []workloadsv1.InstanceStatus) (map[string]string, error) {
	return instanceTemplatesByState(statuses, workloadsv1.InstanceDesiredStateActive, nil)
}

func instanceTemplatesByState(statuses []workloadsv1.InstanceStatus, desired workloadsv1.InstanceDesiredState,
	include func(workloadsv1.InstanceStatus) bool) (map[string]string, error) {
	if err := validateInstanceIdentities(statuses); err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, status := range statuses {
		if status.EffectiveDesiredState() != desired || include != nil && !include(status) {
			continue
		}
		if status.TemplateName == nil {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
				"waiting for InstanceSet to publish the template assignment of instance %q", status.PodName)
		}
		result[status.PodName] = *status.TemplateName
	}
	return result, nil
}

func expectedTemplateReplicas(component *appsv1.ClusterComponentSpec) (map[string]int32, bool) {
	if component == nil {
		return nil, false
	}
	expected := map[string]int32{}
	defaultReplicas := component.Replicas
	for _, template := range component.Instances {
		replicas := template.GetReplicas()
		if replicas > 0 {
			expected[template.Name] += replicas
		}
		defaultReplicas -= replicas
	}
	if defaultReplicas < 0 {
		return nil, false
	}
	if defaultReplicas > 0 {
		expected[""] = defaultReplicas
	}
	return expected, true
}

func assignmentsMatchComponent(assignments map[string]string, component *appsv1.ClusterComponentSpec) bool {
	if component == nil || int32(len(assignments)) != component.Replicas {
		return false
	}
	expected, ok := expectedTemplateReplicas(component)
	if !ok {
		return false
	}
	actual := map[string]int32{}
	for _, templateName := range assignments {
		actual[templateName]++
	}
	if len(actual) != len(expected) {
		return false
	}
	for templateName, count := range expected {
		if actual[templateName] != count {
			return false
		}
	}
	return true
}

func activeAssignmentsForTarget(workload Workload, component *appsv1.ClusterComponentSpec) (map[string]string, bool, error) {
	assignments, err := activeInstanceTemplates(workload.GetInstanceStatuses())
	if err != nil {
		return nil, false, err
	}
	return assignments, assignmentsMatchComponent(assignments, component), nil
}

func sourceAssignments(assignments []opsv1alpha1.InstanceTemplateAssignment, workloadName string,
	state workloadsv1.InstanceDesiredState) map[string]string {
	result := map[string]string{}
	for _, assignment := range assignments {
		if assignment.DesiredState == state && (workloadName == "" || assignment.WorkloadName == workloadName) {
			result[assignment.PodName] = assignment.TemplateName
		}
	}
	return result
}

func assignmentsIncludeOffline(assignments map[string]string, offline []string) bool {
	for _, name := range offline {
		if _, ok := assignments[name]; ok {
			return true
		}
	}
	return false
}

func diffInstanceAssignments(source, target map[string]string) (created, deleted, updated map[string]string) {
	created, deleted, updated = map[string]string{}, map[string]string{}, map[string]string{}
	for name, template := range target {
		if old, ok := source[name]; !ok {
			created[name] = template
		} else if old != template {
			updated[name] = template
		}
	}
	for name, template := range source {
		if _, ok := target[name]; !ok {
			deleted[name] = template
		}
	}
	return
}
