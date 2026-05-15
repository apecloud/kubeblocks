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

package cluster

import (
	"context"
	"fmt"
	"slices"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type clusterStatusTransformer struct{}

var _ graph.Transformer = &clusterStatusTransformer{}

func (t *clusterStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	origCluster := transCtx.OrigCluster
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	defer func() { t.markClusterDagStatusAction(graphCli, dag, origCluster, cluster) }()
	if err := t.reconcileClusterStatus(transCtx.Context, transCtx.Client, cluster); err != nil {
		return err
	}
	return nil
}

func (t *clusterStatusTransformer) markClusterDagStatusAction(graphCli model.GraphClient, dag *graph.DAG, origCluster, cluster *appsv1.Cluster) {
	if v := graphCli.FindMatchedVertex(dag, cluster); v == nil {
		graphCli.Status(dag, origCluster, cluster)
	}
}

func (t *clusterStatusTransformer) reconcileClusterStatus(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster) error {
	if len(cluster.Status.Components) == 0 && len(cluster.Status.Shardings) == 0 {
		return nil
	}
	t.reconcileClusterPhase(cluster)
	return t.syncClusterConditions(ctx, cli, cluster)
}

func (t *clusterStatusTransformer) reconcileClusterPhase(cluster *appsv1.Cluster) appsv1.ClusterPhase {
	statusList := make([]appsv1.ClusterComponentStatus, 0)
	if cluster.Status.Components != nil {
		statusList = append(statusList, maps.Values(cluster.Status.Components)...)
	}
	if cluster.Status.Shardings != nil {
		statusList = append(statusList, maps.Values(t.shardingToCompStatus(cluster.Status.Shardings))...)
	}
	newPhase := composeClusterPhase(statusList)

	phase := cluster.Status.Phase
	if newPhase != "" {
		cluster.Status.Phase = newPhase
	}
	cluster.Status.ObservedGeneration = slices.MinFunc(statusList, func(a, b appsv1.ClusterComponentStatus) int {
		diff := a.ObservedGeneration - b.ObservedGeneration
		if diff == 0 {
			return 0
		}
		if diff < 0 {
			return -1
		}
		return 1
	}).ObservedGeneration
	return phase
}

func (t *clusterStatusTransformer) syncClusterConditions(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster) error {
	if cluster.Status.Phase == appsv1.RunningClusterPhase {
		meta.SetStatusCondition(&cluster.Status.Conditions, newClusterReadyCondition(cluster.Name))
	} else {
		kindNames := map[string][]string{}
		for kind, statusMap := range map[string]map[string]appsv1.ClusterComponentStatus{
			"component": cluster.Status.Components,
			"sharding":  t.shardingToCompStatus(cluster.Status.Shardings),
		} {
			for name, status := range statusMap {
				if status.Phase == appsv1.FailedComponentPhase {
					if _, ok := kindNames[kind]; !ok {
						kindNames[kind] = []string{}
					}
					kindNames[kind] = append(kindNames[kind], name)
				}
			}
		}
		if len(kindNames) > 0 {
			meta.SetStatusCondition(&cluster.Status.Conditions, newClusterNotReadyCondition(cluster.Name, kindNames))
		}
	}

	setAvailableCondition := func() error {
		comps, shardingComps, err := listClusterComponents(ctx, cli, cluster)
		if err != nil {
			return err
		}
		available := true
		aggregatedMessage := ""
		defer func() {
			var condition metav1.Condition
			if available {
				condition = metav1.Condition{
					Type:    appsv1.ConditionTypeAvailable,
					Status:  metav1.ConditionTrue,
					Message: "All components are available",
					Reason:  "Available",
				}
			} else {
				condition = metav1.Condition{
					Type:    appsv1.ConditionTypeAvailable,
					Status:  metav1.ConditionFalse,
					Message: aggregatedMessage,
					Reason:  "Unavailable",
				}
			}

			meta.SetStatusCondition(&cluster.Status.Conditions, condition)
		}()

		if len(comps) == 0 && len(shardingComps) == 0 {
			available = false
			aggregatedMessage = "no component exists; "
			return nil
		}

		for _, comp := range comps {
			compCond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
			if compCond != nil {
				if compCond.Status != metav1.ConditionTrue {
					available = false
					message := fmt.Sprintf("component %s is not available", comp.Name)
					aggregatedMessage += message + "; "
				}
			} else {
				available = false
				message := fmt.Sprintf("component %s has no available condition", comp.Name)
				aggregatedMessage += message + "; "
			}
		}

		for shardingName, comps := range shardingComps {
			for _, comp := range comps {
				compCond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				if compCond != nil {
					if compCond.Status != metav1.ConditionTrue {
						available = false
						message := fmt.Sprintf("component %s of sharding %s is not available", comp.Name, shardingName)
						aggregatedMessage += message + "; "
					}
				} else {
					available = false
					message := fmt.Sprintf("component %s of sharding %s has no available condition", comp.Name, shardingName)
					aggregatedMessage += message + "; "
				}
			}
		}

		return nil
	}

	if err := setAvailableCondition(); err != nil {
		return err
	}
	return t.setRestoreCondition(ctx, cli, cluster)
}

func (t *clusterStatusTransformer) setRestoreCondition(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster) error {
	restoreCond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	if cluster.Spec.Restore == nil && restoreCond == nil {
		return nil
	}
	if restoreCond != nil && (restoreCond.Status == metav1.ConditionTrue || restoreCond.Status == metav1.ConditionFalse) {
		return nil
	}
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(ctx, pvcList, client.InNamespace(cluster.Namespace), client.MatchingLabels(constant.GetClusterLabels(cluster.Name))); err != nil {
		return err
	}
	componentList := &appsv1.ComponentList{}
	if err := cli.List(ctx, componentList, client.InNamespace(cluster.Namespace), client.MatchingLabels(constant.GetClusterLabels(cluster.Name))); err != nil {
		return err
	}
	expectedComponents := expectedRestoreComponentCount(cluster)
	existingComponents := countExistingRestoreComponents(componentList)
	expectedPVCs := expectedRestorePVCCount(componentList)
	total := 0
	completed := 0
	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]
		if pvc.Annotations[constant.RestoreSourceKindAnnotationKey] == "" {
			continue
		}
		total++
		cond := findPVCCondition(pvc, appsv1.ConditionTypeRestore)
		if cond == nil {
			continue
		}
		switch cond.Status {
		case corev1.ConditionTrue:
			completed++
		case corev1.ConditionFalse:
			meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
				Type:    appsv1.ConditionTypeRestore,
				Status:  metav1.ConditionFalse,
				Reason:  ReasonRestoreFailed,
				Message: fmt.Sprintf("PVC %s restore failed: %s", pvc.Name, cond.Message),
			})
			return nil
		}
	}
	if expectedComponents > 0 && existingComponents < expectedComponents {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    appsv1.ConditionTypeRestore,
			Status:  metav1.ConditionUnknown,
			Reason:  ReasonRestoreRunning,
			Message: "Waiting for initial restore components to be created",
		})
		return nil
	}
	if total > 0 && total == completed && total >= expectedPVCs {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    appsv1.ConditionTypeRestore,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonRestoreCompleted,
			Message: "All initial restore PVCs have completed",
		})
		return nil
	}
	if total == 0 && cluster.Spec.Restore != nil {
		if expectedRestoreVCTCount(cluster) > 0 || expectedPVCs > 0 {
			meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
				Type:    appsv1.ConditionTypeRestore,
				Status:  metav1.ConditionUnknown,
				Reason:  ReasonRestoreRunning,
				Message: "Waiting for initial restore PVCs to be created",
			})
			return nil
		}
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    appsv1.ConditionTypeRestore,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonRestoreCompleted,
			Message: "No restore PVCs are required",
		})
		return nil
	}
	if cluster.Spec.Restore != nil {
		meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    appsv1.ConditionTypeRestore,
			Status:  metav1.ConditionUnknown,
			Reason:  ReasonRestoreRunning,
			Message: "Waiting for initial restore PVCs to complete",
		})
	}
	return nil
}

func expectedRestoreComponentCount(cluster *appsv1.Cluster) int {
	total := len(cluster.Spec.ComponentSpecs)
	for i := range cluster.Spec.Shardings {
		total += int(cluster.Spec.Shardings[i].Shards)
	}
	return total
}

func countExistingRestoreComponents(componentList *appsv1.ComponentList) int {
	total := 0
	for i := range componentList.Items {
		if componentList.Items[i].DeletionTimestamp.IsZero() {
			total++
		}
	}
	return total
}

func expectedRestorePVCCount(componentList *appsv1.ComponentList) int {
	total := 0
	for i := range componentList.Items {
		comp := &componentList.Items[i]
		if !comp.DeletionTimestamp.IsZero() {
			continue
		}
		total += expectedRestorePVCCountForComponentSpec(comp.Spec.Replicas, comp.Spec.VolumeClaimTemplates, comp.Spec.Instances)
	}
	return total
}

func expectedRestorePVCCountForComponentSpec(replicas int32, vcts []appsv1.PersistentVolumeClaimTemplate, instances []appsv1.InstanceTemplate) int {
	total := 0
	replicasInTemplates := int32(0)
	for i := range instances {
		instanceReplicas := int32(1)
		if instances[i].Replicas != nil {
			instanceReplicas = *instances[i].Replicas
		}
		if instanceReplicas < 0 {
			instanceReplicas = 0
		}
		replicasInTemplates += instanceReplicas
		total += int(instanceReplicas) * mergedRestoreVCTCount(vcts, instances[i].VolumeClaimTemplates)
	}
	defaultReplicas := replicas - replicasInTemplates
	if defaultReplicas < 0 {
		defaultReplicas = 0
	}
	return total + int(defaultReplicas)*len(vcts)
}

func mergedRestoreVCTCount(base []appsv1.PersistentVolumeClaimTemplate, overrides []appsv1.PersistentVolumeClaimTemplate) int {
	names := map[string]struct{}{}
	for i := range base {
		names[base[i].Name] = struct{}{}
	}
	for i := range overrides {
		names[overrides[i].Name] = struct{}{}
	}
	return len(names)
}

func expectedRestoreVCTCount(cluster *appsv1.Cluster) int {
	total := 0
	for i := range cluster.Spec.ComponentSpecs {
		total += restoreTemplateCount(cluster.Spec.ComponentSpecs[i].VolumeClaimTemplates, cluster.Spec.ComponentSpecs[i].Instances)
	}
	for i := range cluster.Spec.Shardings {
		sharding := &cluster.Spec.Shardings[i]
		total += restoreTemplateCount(sharding.Template.VolumeClaimTemplates, sharding.Template.Instances)
		for j := range sharding.ShardTemplates {
			total += restoreTemplateCount(sharding.ShardTemplates[j].VolumeClaimTemplates, sharding.ShardTemplates[j].Instances)
		}
	}
	return total
}

func restoreTemplateCount(vcts []appsv1.PersistentVolumeClaimTemplate, instances []appsv1.InstanceTemplate) int {
	total := len(vcts)
	for i := range instances {
		total += len(instances[i].VolumeClaimTemplates)
	}
	return total
}

func findPVCCondition(pvc *corev1.PersistentVolumeClaim, conditionType string) *corev1.PersistentVolumeClaimCondition {
	for i := range pvc.Status.Conditions {
		if string(pvc.Status.Conditions[i].Type) == conditionType {
			return &pvc.Status.Conditions[i]
		}
	}
	return nil
}

func (t *clusterStatusTransformer) shardingToCompStatus(shardingStatus map[string]appsv1.ClusterShardingStatus) map[string]appsv1.ClusterComponentStatus {
	result := make(map[string]appsv1.ClusterComponentStatus)
	for name, status := range shardingStatus {
		result[name] = appsv1.ClusterComponentStatus{
			Phase:              status.Phase,
			Message:            status.Message,
			ObservedGeneration: status.ObservedGeneration,
			UpToDate:           status.UpToDate,
		}
	}
	return result
}

func composeClusterPhase(statusList []appsv1.ClusterComponentStatus) appsv1.ClusterPhase {
	var (
		isAllComponentCreating         = true
		isAllComponentWorking          = true
		hasComponentStarting           = false
		hasComponentStopping           = false
		isAllComponentStopped          = true
		isAllComponentFailed           = true
		hasComponentFailed             = false
		isAllComponentRunningOrStopped = true
	)
	isPhaseIn := func(phase appsv1.ComponentPhase, phases ...appsv1.ComponentPhase) bool {
		for _, p := range phases {
			if p == phase {
				return true
			}
		}
		return false
	}
	for _, status := range statusList {
		phase := status.Phase
		if !isPhaseIn(phase, appsv1.CreatingComponentPhase, "") {
			isAllComponentCreating = false
		}
		if !isPhaseIn(phase, appsv1.RunningComponentPhase, appsv1.StoppedComponentPhase) {
			isAllComponentRunningOrStopped = false
		}
		if !isPhaseIn(phase, appsv1.CreatingComponentPhase, appsv1.RunningComponentPhase, appsv1.UpdatingComponentPhase) {
			isAllComponentWorking = false
		}
		if isPhaseIn(phase, appsv1.StartingComponentPhase) {
			hasComponentStarting = true
		}
		if isPhaseIn(phase, appsv1.StoppingComponentPhase) {
			hasComponentStopping = true
		}
		if !isPhaseIn(phase, appsv1.StoppedComponentPhase) {
			isAllComponentStopped = false
		}
		if !isPhaseIn(phase, appsv1.FailedComponentPhase) {
			isAllComponentFailed = false
		}
		if isPhaseIn(phase, appsv1.FailedComponentPhase) {
			hasComponentFailed = true
		}

	}

	switch {
	case isAllComponentStopped:
		return appsv1.StoppedClusterPhase
	case isAllComponentRunningOrStopped:
		return appsv1.RunningClusterPhase
	case isAllComponentCreating:
		return appsv1.CreatingClusterPhase
	case isAllComponentWorking || hasComponentStarting:
		return appsv1.UpdatingClusterPhase
	case hasComponentStopping:
		return appsv1.StoppingClusterPhase
	case isAllComponentFailed:
		return appsv1.FailedClusterPhase
	case hasComponentFailed:
		return appsv1.AbnormalClusterPhase
	default:
		return ""
	}
}
