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
	"sort"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func instanceStatusByName(statuses []workloadsv1.InstanceStatus) (map[string]workloadsv1.InstanceStatus, error) {
	result := make(map[string]workloadsv1.InstanceStatus, len(statuses))
	for _, status := range statuses {
		if status.PodName == "" {
			return nil, fmt.Errorf("InstanceSet published an empty instance identity")
		}
		if _, ok := result[status.PodName]; ok {
			return nil, fmt.Errorf("InstanceSet published duplicate instance identity %q", status.PodName)
		}
		result[status.PodName] = status
	}
	return result, nil
}

func statusesToPodSet(statuses []workloadsv1.InstanceStatus, desired workloadsv1.InstanceDesiredState,
	include func(workloadsv1.InstanceStatus) bool, requireTemplate bool) (map[string]string, error) {
	if _, err := instanceStatusByName(statuses); err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, status := range statuses {
		if status.EffectiveDesiredState() != desired || include != nil && !include(status) {
			continue
		}
		if status.TemplateName == nil {
			if requireTemplate {
				return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
					"waiting for InstanceSet to publish the template assignment of instance %q", status.PodName)
			}
			result[status.PodName] = ""
			continue
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
		expected[template.Name] += replicas
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
	assignments, err := statusesToPodSet(workload.GetInstanceStatuses(), workloadsv1.InstanceDesiredStateActive, nil, true)
	if err != nil {
		return nil, false, err
	}
	return assignments, assignmentsMatchComponent(assignments, component), nil
}

func sourceAssignmentsForWorkload(last opsv1alpha1.LastComponentConfiguration, workloadName string,
	desired workloadsv1.InstanceDesiredState) map[string]string {
	result := map[string]string{}
	for _, assignment := range last.SourceInstanceAssignments {
		if assignment.WorkloadName == workloadName && assignment.DesiredState == desired {
			result[assignment.PodName] = assignment.TemplateName
		}
	}
	return result
}

func diffAssignments(source, target map[string]string) (created, deleted map[string]string) {
	created = map[string]string{}
	deleted = map[string]string{}
	for name, templateName := range target {
		if _, ok := source[name]; !ok {
			created[name] = templateName
		}
	}
	for name, templateName := range source {
		if _, ok := target[name]; !ok {
			deleted[name] = templateName
		}
	}
	return created, deleted
}

func flatHorizontalDiffMatchesOperation(horizontalScaling opsv1alpha1.HorizontalScaling,
	created, deleted map[string]string) bool {
	if horizontalScaling.ScaleOut == nil && len(created) > 0 {
		return false
	}
	if horizontalScaling.ScaleIn == nil && len(deleted) > 0 {
		return false
	}
	if horizontalScaling.ScaleIn != nil {
		for _, name := range horizontalScaling.ScaleIn.OnlineInstancesToOffline {
			if _, ok := deleted[name]; !ok {
				return false
			}
		}
	}
	if horizontalScaling.ScaleOut != nil {
		for _, name := range horizontalScaling.ScaleOut.OfflineInstancesToOnline {
			if _, ok := created[name]; !ok {
				return false
			}
		}
	}
	return true
}

func captureFlatSourceAssignments(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	compOps componentOpsHelper, requiredOffline map[string]sets.Set[string]) error {
	capture := func(logicalComponentName, fullComponentName string, component *appsv1.ClusterComponentSpec) error {
		if component == nil || !component.FlatInstanceOrdinal {
			return nil
		}
		runtime, err := opsRes.GetRuntime(logicalComponentName)
		if err != nil {
			return err
		}
		workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, fullComponentName)
		if err != nil {
			return err
		}
		active, err := statusesToPodSet(workload.GetInstanceStatuses(), workloadsv1.InstanceDesiredStateActive, nil, true)
		if err != nil {
			return err
		}
		if !assignmentsMatchComponent(active, component) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
				"waiting for InstanceSet %q to publish its active instance allocation", constant.GenerateClusterComponentName(opsRes.Cluster.Name, fullComponentName))
		}
		workloadName := constant.GenerateClusterComponentName(opsRes.Cluster.Name, fullComponentName)
		last := opsRes.OpsRequest.Status.LastConfiguration.Components[logicalComponentName]
		kept := last.SourceInstanceAssignments[:0]
		for _, assignment := range last.SourceInstanceAssignments {
			if assignment.WorkloadName != workloadName {
				kept = append(kept, assignment)
			}
		}
		last.SourceInstanceAssignments = kept
		for podName, templateName := range active {
			last.SourceInstanceAssignments = append(last.SourceInstanceAssignments, opsv1alpha1.InstanceTemplateAssignment{
				WorkloadName: workloadName, PodName: podName, TemplateName: templateName,
				DesiredState: workloadsv1.InstanceDesiredStateActive,
			})
		}
		if names := requiredOffline[logicalComponentName]; names.Len() > 0 {
			byName, err := instanceStatusByName(workload.GetInstanceStatuses())
			if err != nil {
				return err
			}
			for name := range names {
				status, ok := byName[name]
				if !ok || status.EffectiveDesiredState() != workloadsv1.InstanceDesiredStateOffline || status.TemplateName == nil {
					return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
						"waiting for InstanceSet %q to publish offline instance %q and its template", workloadName, name)
				}
				last.SourceInstanceAssignments = append(last.SourceInstanceAssignments, opsv1alpha1.InstanceTemplateAssignment{
					WorkloadName: workloadName, PodName: name, TemplateName: *status.TemplateName,
					DesiredState: workloadsv1.InstanceDesiredStateOffline,
				})
			}
		}
		sort.Slice(last.SourceInstanceAssignments, func(i, j int) bool {
			a, b := last.SourceInstanceAssignments[i], last.SourceInstanceAssignments[j]
			if a.WorkloadName != b.WorkloadName {
				return a.WorkloadName < b.WorkloadName
			}
			return a.PodName < b.PodName
		})
		opsRes.OpsRequest.Status.LastConfiguration.Components[logicalComponentName] = last
		return nil
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		component := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if op, ok := compOps.getComponentOps(component.Name); ok {
			if horizontalScaling, ok := op.(opsv1alpha1.HorizontalScaling); ok && horizontalScaling.Shards != nil {
				continue
			}
			if err := capture(component.Name, component.Name, component); err != nil {
				return err
			}
		}
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		shardingSpec := &opsRes.Cluster.Spec.Shardings[i]
		op, ok := compOps.getComponentOps(shardingSpec.Name)
		if !ok {
			continue
		}
		if horizontalScaling, ok := op.(opsv1alpha1.HorizontalScaling); ok && horizontalScaling.Shards != nil {
			continue
		}
		components, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, shardingSpec.Name)
		if err != nil {
			return err
		}
		for _, component := range components {
			fullName := component.Labels[constant.KBAppComponentLabelKey]
			if err := capture(shardingSpec.Name, fullName, &shardingSpec.Template); err != nil {
				return err
			}
		}
	}
	return nil
}
