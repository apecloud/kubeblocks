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
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func instanceParticipants(statuses []workloads.InstanceStatus, includeOffline bool) ([]opsv1alpha1.InstanceParticipant, error) {
	participants := make([]opsv1alpha1.InstanceParticipant, 0, len(statuses))
	seen := sets.New[string]()
	for i := range statuses {
		status := &statuses[i]
		desiredState := status.EffectiveDesiredState()
		if desiredState != workloads.InstanceDesiredStateActive &&
			(!includeOffline || desiredState != workloads.InstanceDesiredStateOffline) {
			continue
		}
		if status.PodName == "" || seen.Has(status.PodName) {
			return nil, fmt.Errorf("invalid duplicate or empty active instance identity %q", status.PodName)
		}
		seen.Insert(status.PodName)
		participant := opsv1alpha1.InstanceParticipant{
			PodName: status.PodName,
			Active:  desiredState == workloads.InstanceDesiredStateActive,
		}
		if status.TemplateName != nil {
			templateName := *status.TemplateName
			participant.TemplateName = &templateName
		}
		participants = append(participants, participant)
	}
	sortParticipants(participants)
	return participants, nil
}

func activeParticipants(statuses []workloads.InstanceStatus) ([]opsv1alpha1.InstanceParticipant, error) {
	return instanceParticipants(statuses, false)
}

func retainedParticipants(statuses []workloads.InstanceStatus) ([]opsv1alpha1.InstanceParticipant, error) {
	return instanceParticipants(statuses, true)
}

func onlyActiveParticipants(participants []opsv1alpha1.InstanceParticipant) []opsv1alpha1.InstanceParticipant {
	result := make([]opsv1alpha1.InstanceParticipant, 0, len(participants))
	for _, participant := range participants {
		if participant.Active {
			result = append(result, participant)
		}
	}
	return result
}

func sortParticipants(participants []opsv1alpha1.InstanceParticipant) {
	slices.SortFunc(participants, func(a, b opsv1alpha1.InstanceParticipant) int {
		if a.PodName < b.PodName {
			return -1
		}
		if a.PodName > b.PodName {
			return 1
		}
		return 0
	})
}

func participantMap(participants []opsv1alpha1.InstanceParticipant) map[string]opsv1alpha1.InstanceParticipant {
	result := make(map[string]opsv1alpha1.InstanceParticipant, len(participants))
	for _, participant := range participants {
		if participant.Active {
			result[participant.PodName] = participant
		}
	}
	return result
}

func participantsHaveKnownTemplates(participants []opsv1alpha1.InstanceParticipant) bool {
	for _, participant := range participants {
		if participant.TemplateName == nil {
			return false
		}
	}
	return true
}

func participantsToSet(participants []opsv1alpha1.InstanceParticipant) map[string]string {
	result := make(map[string]string, len(participants))
	for _, participant := range participants {
		if participant.TemplateName != nil {
			result[participant.PodName] = *participant.TemplateName
		} else {
			result[participant.PodName] = ""
		}
	}
	return result
}

func participantsMatchComponent(participants []opsv1alpha1.InstanceParticipant, component *appsv1.ClusterComponentSpec) bool {
	active := onlyActiveParticipants(participants)
	if component == nil || int32(len(active)) != component.Replicas {
		return false
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
		return false
	}
	if defaultReplicas > 0 {
		expected[""] = defaultReplicas
	}
	actual := map[string]int32{}
	for _, participant := range active {
		if participant.TemplateName == nil {
			return false
		}
		actual[*participant.TemplateName]++
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

func findParticipantSnapshot(compStatus *opsv1alpha1.OpsRequestComponentStatus,
	workloadName string) *opsv1alpha1.InstanceParticipantSnapshot {
	for i := range compStatus.InstanceParticipants {
		if compStatus.InstanceParticipants[i].WorkloadName == workloadName {
			return &compStatus.InstanceParticipants[i]
		}
	}
	return nil
}

func sourceParticipantsForComponent(opsRes *OpsResource, componentName string) []opsv1alpha1.InstanceParticipant {
	if opsRes == nil || opsRes.OpsRequest == nil {
		return nil
	}
	compStatus, ok := opsRes.OpsRequest.Status.Components[componentName]
	if !ok {
		return nil
	}
	var result []opsv1alpha1.InstanceParticipant
	for _, snapshot := range compStatus.InstanceParticipants {
		result = append(result, snapshot.Source...)
	}
	return result
}

func hasParticipantSnapshotsForComponent(opsRes *OpsResource, componentName string) bool {
	if opsRes == nil || opsRes.OpsRequest == nil {
		return false
	}
	compStatus, ok := opsRes.OpsRequest.Status.Components[componentName]
	return ok && len(compStatus.InstanceParticipants) > 0
}

// captureSourceParticipants records the Active identities before Action mutates the Cluster spec.
func captureSourceParticipants(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource,
	compOps componentOpsHelper) error {
	if opsRes.OpsRequest.Status.Components == nil {
		opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	capture := func(logicalComponentName, fullComponentName string, sourceComponent *appsv1.ClusterComponentSpec,
		expectedActive int32) error {
		runtime, err := opsRes.GetRuntime(logicalComponentName)
		if err != nil {
			return err
		}
		workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, fullComponentName)
		if err != nil {
			return err
		}
		if workload.GetUID() == "" {
			// Preserve the existing Pod/PVC based operation path while the
			// InstanceSet (and therefore richer InstanceStatus) is unavailable.
			return nil
		}
		compStatus := opsRes.OpsRequest.Status.Components[logicalComponentName]
		if snapshot := findParticipantSnapshot(&compStatus, workload.GetName()); snapshot != nil {
			if snapshot.WorkloadUID != workload.GetUID() {
				return fmt.Errorf("InstanceSet %q was recreated while capturing operation participants", workload.GetName())
			}
			return nil
		}
		source, err := retainedParticipants(workload.GetInstanceStatuses())
		if err != nil {
			return err
		}
		if !workload.HasInstanceStatus() {
			plan, err := runtime.GenerateInstanceNamePlan(opsRes.Cluster.Namespace, opsRes.Cluster.Name,
				fullComponentName, *sourceComponent)
			if err != nil {
				return err
			}
			// Before the richer InstanceStatus is published, preserve the existing
			// name-planning semantics. In particular, planned Active identities may
			// not have Pods yet and Offline identities may no longer have Pods.
			source = make([]opsv1alpha1.InstanceParticipant, 0,
				len(plan.TemplateByName)+len(plan.OfflineTemplateByName))
			for name, templateName := range plan.TemplateByName {
				source = append(source, opsv1alpha1.InstanceParticipant{
					PodName:      name,
					TemplateName: &templateName,
					Active:       true,
				})
			}
			for name, templateName := range plan.OfflineTemplateByName {
				if _, active := plan.TemplateByName[name]; active {
					return fmt.Errorf("instance identity %q is both active and offline", name)
				}
				source = append(source, opsv1alpha1.InstanceParticipant{
					PodName:      name,
					TemplateName: &templateName,
				})
			}
			sortParticipants(source)
		}
		if int32(len(onlyActiveParticipants(source))) != expectedActive {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
				"waiting for InstanceSet %q to publish all %d active instance identities", workload.GetName(), expectedActive)
		}
		if !participantsHaveKnownTemplates(source) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
				"waiting for InstanceSet %q to publish instance template assignments", workload.GetName())
		}
		if expectedActive > 0 && !participantsMatchComponent(source, sourceComponent) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting,
				"waiting for InstanceSet %q to publish the current instance template allocation", workload.GetName())
		}
		compStatus.InstanceParticipants = append(compStatus.InstanceParticipants, opsv1alpha1.InstanceParticipantSnapshot{
			WorkloadName:     workload.GetName(),
			WorkloadUID:      workload.GetUID(),
			SourceGeneration: workload.GetGeneration(),
			Source:           source,
		})
		opsRes.OpsRequest.Status.Components[logicalComponentName] = compStatus
		return nil
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		componentName := opsRes.Cluster.Spec.ComponentSpecs[i].Name
		componentOps, ok := compOps.getComponentOps(componentName)
		if !ok {
			continue
		}
		if horizontalScaling, ok := componentOps.(opsv1alpha1.HorizontalScaling); ok && horizontalScaling.Shards != nil {
			continue
		}
		expectedActive := opsRes.Cluster.Spec.ComponentSpecs[i].Replicas
		if err := capture(componentName, componentName, &opsRes.Cluster.Spec.ComponentSpecs[i], expectedActive); err != nil {
			return err
		}
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		shardingName := opsRes.Cluster.Spec.Shardings[i].Name
		componentOps, ok := compOps.getComponentOps(shardingName)
		if !ok {
			continue
		}
		if horizontalScaling, ok := componentOps.(opsv1alpha1.HorizontalScaling); ok && horizontalScaling.Shards != nil {
			continue
		}
		components, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, shardingName)
		if err != nil {
			return err
		}
		expectedActive := opsRes.Cluster.Spec.Shardings[i].Template.Replicas
		for _, component := range components {
			fullComponentName := component.Labels[constant.KBAppComponentLabelKey]
			if err := capture(shardingName, fullComponentName, &opsRes.Cluster.Spec.Shardings[i].Template, expectedActive); err != nil {
				return err
			}
		}
	}
	return nil
}

func freezeCreatedSourceParticipants(opsRes *OpsResource, compOps componentOpsHelper) {
	componentNames := sets.New[string]()
	if len(compOps.componentOpsSet) == 0 {
		for componentName := range opsRes.OpsRequest.Status.Components {
			componentNames.Insert(componentName)
		}
	} else {
		for componentName := range compOps.componentOpsSet {
			componentNames.Insert(componentName)
		}
	}
	for componentName := range componentNames {
		compStatus := opsRes.OpsRequest.Status.Components[componentName]
		for i := range compStatus.InstanceParticipants {
			snapshot := &compStatus.InstanceParticipants[i]
			if snapshot.Frozen {
				continue
			}
			snapshot.Created = onlyActiveParticipants(snapshot.Source)
			snapshot.Frozen = true
		}
		opsRes.OpsRequest.Status.Components[componentName] = compStatus
	}
}

func freezeDeletedSourceParticipants(opsRes *OpsResource, compOps componentOpsHelper) {
	componentNames := sets.New[string]()
	if len(compOps.componentOpsSet) == 0 {
		for componentName := range opsRes.OpsRequest.Status.Components {
			componentNames.Insert(componentName)
		}
	} else {
		for componentName := range compOps.componentOpsSet {
			componentNames.Insert(componentName)
		}
	}
	for componentName := range componentNames {
		compStatus := opsRes.OpsRequest.Status.Components[componentName]
		for i := range compStatus.InstanceParticipants {
			snapshot := &compStatus.InstanceParticipants[i]
			if snapshot.Frozen {
				continue
			}
			snapshot.Deleted = onlyActiveParticipants(snapshot.Source)
			snapshot.Frozen = true
		}
		opsRes.OpsRequest.Status.Components[componentName] = compStatus
	}
}

// freezeTargetParticipants compares the current Active identities with the captured source. It refuses
// to publish a partial target view: targetCount must match the complete desired allocation first.
func freezeTargetParticipants(compStatus *opsv1alpha1.OpsRequestComponentStatus, workload Workload,
	targetComponent *appsv1.ClusterComponentSpec, updatedTemplates sets.Set[string], includeUpdates bool) (*opsv1alpha1.InstanceParticipantSnapshot, bool, error) {
	snapshot := findParticipantSnapshot(compStatus, workload.GetName())
	if snapshot == nil {
		return nil, false, fmt.Errorf("source participant snapshot for InstanceSet %q is missing", workload.GetName())
	}
	if snapshot.WorkloadUID != workload.GetUID() {
		return nil, false, fmt.Errorf("InstanceSet %q was recreated during the operation", workload.GetName())
	}
	if snapshot.Frozen {
		return snapshot, true, nil
	}
	if workload.HasInstanceStatus() && workload.GetGeneration() <= snapshot.SourceGeneration {
		return snapshot, false, nil
	}
	target, err := activeParticipants(workload.GetInstanceStatuses())
	if err != nil {
		return nil, false, err
	}
	if !participantsHaveKnownTemplates(target) || !participantsMatchComponent(target, targetComponent) {
		return snapshot, false, nil
	}
	sourceByName := participantMap(snapshot.Source)
	targetByName := participantMap(target)
	for name, participant := range targetByName {
		if _, ok := sourceByName[name]; !ok {
			snapshot.Created = append(snapshot.Created, participant)
		}
	}
	for name, participant := range sourceByName {
		if _, ok := targetByName[name]; !ok {
			snapshot.Deleted = append(snapshot.Deleted, participant)
		}
	}
	if includeUpdates {
		for name, participant := range targetByName {
			if _, ok := sourceByName[name]; !ok {
				continue
			}
			templateName := ""
			if participant.TemplateName != nil {
				templateName = *participant.TemplateName
			}
			if updatedTemplates == nil || updatedTemplates.Has(templateName) {
				snapshot.Updated = append(snapshot.Updated, participant)
			}
		}
	}
	sortParticipants(snapshot.Created)
	sortParticipants(snapshot.Deleted)
	sortParticipants(snapshot.Updated)
	snapshot.TargetGeneration = workload.GetGeneration()
	snapshot.Frozen = true
	return snapshot, true, nil
}

func workloadForProgress(opsRes *OpsResource, logicalComponentName, fullComponentName string) (Workload, error) {
	runtime, err := opsRes.GetRuntime(logicalComponentName)
	if err != nil {
		return nil, err
	}
	return runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, fullComponentName)
}
