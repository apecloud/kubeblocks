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
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func sourceAssignmentsForWorkload(last opsv1alpha1.LastComponentConfiguration, workloadName string) map[string]string {
	result := map[string]string{}
	for _, assignment := range last.SourceInstanceAssignments {
		if assignment.WorkloadName == workloadName && assignment.DesiredState == workloadsv1.InstanceDesiredStateActive {
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

func horizontalDiffMatchesOperation(horizontalScaling opsv1alpha1.HorizontalScaling,
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

// rollbackInstanceSets retains checks for previously affected identities even
// after the desired allocation has returned to the source. Desired identity
// membership alone does not imply that creation/deletion has finished.
func rollbackInstanceSets(source map[string]string, workload Workload, explicitOffline []string,
	details []opsv1alpha1.ProgressStatusDetail, componentName string) (map[string]string, map[string]string) {
	created, deleted := map[string]string{}, map[string]string{}
	for _, detail := range details {
		if !strings.HasPrefix(detail.Group, componentName+"/") {
			continue
		}
		name, ok := strings.CutPrefix(detail.ObjectKey, "Pod/")
		if !ok {
			continue
		}
		if template, ok := source[name]; ok {
			created[name] = template
		} else {
			deleted[name] = ""
		}
	}
	// Cover cancellation before any forward progress has been recorded.
	offline := sets.New(explicitOffline...)
	current := workload.GetCurrentRevisionMap()
	for name := range current {
		if _, ok := source[name]; !ok && !offline.Has(name) {
			deleted[name] = ""
		}
	}
	for name, template := range source {
		if _, ok := current[name]; !ok {
			created[name] = template
		}
	}
	return created, deleted
}

func captureHScaleSourceAssignments(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	compOps componentOpsHelper, requiredOffline map[string]sets.Set[string]) error {
	remainingOffline := map[string]sets.Set[string]{}
	for componentName, names := range requiredOffline {
		remainingOffline[componentName] = names.Clone()
	}
	capture := func(logicalComponentName, fullComponentName string, op ComponentOpsInterface,
		component *appsv1.ClusterComponentSpec) error {
		if component == nil || op.(opsv1alpha1.HorizontalScaling).Shards != nil || scaleOutFromBackup(op.(opsv1alpha1.HorizontalScaling)) {
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
		active, err := activeInstanceTemplates(workload.GetInstanceStatuses())
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
		if names := remainingOffline[logicalComponentName]; names.Len() > 0 {
			for _, status := range workload.GetInstanceStatuses() {
				name := status.PodName
				if !names.Has(name) {
					continue
				}
				if status.EffectiveDesiredState() != workloadsv1.InstanceDesiredStateOffline || status.TemplateName == nil {
					return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
						"waiting for InstanceSet %q to publish offline instance %q and its template", workloadName, name)
				}
				last.SourceInstanceAssignments = append(last.SourceInstanceAssignments, opsv1alpha1.InstanceTemplateAssignment{
					WorkloadName: workloadName, PodName: name, TemplateName: *status.TemplateName,
					DesiredState: workloadsv1.InstanceDesiredStateOffline,
				})
				names.Delete(name)
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
			if err := capture(component.Name, component.Name, op, component); err != nil {
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
		components, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, shardingSpec.Name)
		if err != nil {
			return err
		}
		for _, component := range components {
			fullName := component.Labels[constant.KBAppComponentLabelKey]
			if err := capture(shardingSpec.Name, fullName, op, &shardingSpec.Template); err != nil {
				return err
			}
		}
	}
	for componentName, names := range remainingOffline {
		if names.Len() > 0 {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
				"waiting for InstanceSet to publish offline instances %v of component %q and their templates",
				sets.List(names), componentName)
		}
	}
	return nil
}
