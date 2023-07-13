/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package components

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

// rebuildReplicationSetClusterStatus syncs replicationSet pod status to cluster.status.component[componentName].ReplicationStatus.
func rebuildReplicationSetClusterStatus(cluster *appsv1alpha1.Cluster,
	workloadType appsv1alpha1.WorkloadType, compName string, podList []corev1.Pod) error {
	if len(podList) == 0 {
		return nil
	}

	var oldReplicationStatus *appsv1alpha1.ReplicationSetStatus
	if v, ok := cluster.Status.Components[compName]; ok {
		oldReplicationStatus = v.ReplicationSetStatus
	}

	newReplicationStatus := &appsv1alpha1.ReplicationSetStatus{}
	if err := genReplicationSetStatus(newReplicationStatus, podList); err != nil {
		return err
	}
	// if status changed, do update
	if !cmp.Equal(newReplicationStatus, oldReplicationStatus) {
		if err := initClusterComponentStatusIfNeed(cluster, compName, workloadType); err != nil {
			return err
		}
		componentStatus := cluster.Status.Components[compName]
		componentStatus.ReplicationSetStatus = newReplicationStatus
		cluster.Status.SetComponentStatus(compName, componentStatus)
	}
	return nil
}

// genReplicationSetStatus generates ReplicationSetStatus from podList.
func genReplicationSetStatus(replicationStatus *appsv1alpha1.ReplicationSetStatus, podList []corev1.Pod) error {
	for _, pod := range podList {
		role := pod.Labels[constant.RoleLabelKey]
		if role == "" {
			return fmt.Errorf("pod %s has no role label", pod.Name)
		}
		switch role {
		case constant.Primary:
			if replicationStatus.Primary.Pod != "" {
				return fmt.Errorf("more than one primary pod found")
			}
			replicationStatus.Primary.Pod = pod.Name
		case constant.Secondary:
			replicationStatus.Secondaries = append(replicationStatus.Secondaries, appsv1alpha1.ReplicationMemberStatus{
				Pod: pod.Name,
			})
		default:
			return fmt.Errorf("unknown role %s", role)
		}
	}
	return nil
}

// getAndCheckReplicationSetPrimaryPod gets and checks the primary Pod of the replication workload.
func getAndCheckReplicationSetPrimaryPod(ctx context.Context, cli client.Client, cluster appsv1alpha1.Cluster, compSpecName string) (*corev1.Pod, error) {
	podList, err := GetComponentPodListWithRole(ctx, cli, cluster, compSpecName, constant.Primary)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) != 1 {
		return nil, fmt.Errorf("the number of current replicationSet primary obj is not 1, pls check")
	}
	return &podList.Items[0], nil
}

// updateObjRoleLabel updates the value of the role label of the object.
func updateObjRoleLabel[T generics.Object, PT generics.PObject[T]](
	ctx context.Context, cli client.Client, obj T, role string) error {
	pObj := PT(&obj)
	patch := client.MergeFrom(PT(pObj.DeepCopy()))
	pObj.GetLabels()[constant.RoleLabelKey] = role
	if err := cli.Patch(ctx, pObj, patch); err != nil {
		return err
	}
	return nil
}

// HandleReplicationSetRoleChangeEvent handles the role change event of the replication workload when switchPolicy is Noop.
func HandleReplicationSetRoleChangeEvent(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	compName string,
	pod *corev1.Pod,
	newRole string) error {
	// if newRole is not Primary or Secondary, ignore it.
	if !slices.Contains([]string{constant.Primary, constant.Secondary}, newRole) {
		reqCtx.Log.Info("replicationSet new role is invalid, please check", "new role", newRole)
		return nil
	}
	// if pod current role label equals to newRole, return
	if pod.Labels[constant.RoleLabelKey] == newRole {
		reqCtx.Log.Info("pod current role label equals to new role, ignore it", "new role", newRole)
		return nil
	}
	// if switchPolicy is not Noop, return
	clusterCompSpec := getClusterComponentSpecByName(*cluster, compName)
	if clusterCompSpec == nil || clusterCompSpec.SwitchPolicy == nil || clusterCompSpec.SwitchPolicy.Type != appsv1alpha1.Noop {
		reqCtx.Log.Info("cluster switchPolicy is not Noop, does not support handling role change event", "cluster", cluster.Name)
		return nil
	}

	oldPrimaryPod, err := getAndCheckReplicationSetPrimaryPod(reqCtx.Ctx, cli, *cluster, compName)
	if err != nil {
		reqCtx.Log.Info("handleReplicationSetRoleChangeEvent gets old primary pod failed", "error", err)
		return err
	}
	if oldPrimaryPod == nil {
		return nil
	}
	// pod is old primary and newRole is secondary, it means that the old primary needs to be changed to secondary,
	// we do not deal with this situation here, the demote labeling process of old primary to secondary is handled
	// in another reconciliation triggered by role change event from secondary -> new primary,
	// this is to avoid simultaneous occurrence of two primary or no primary at the same time
	if oldPrimaryPod.Name == pod.Name {
		reqCtx.Log.Info("pod is old primary and new role is secondary, do not deal with this situation",
			"podName", pod.Name, "newRole", newRole)
		return nil
	}

	// update old primary pod to secondary
	if err := updateObjRoleLabel(reqCtx.Ctx, cli, *oldPrimaryPod, constant.Secondary); err != nil {
		return err
	}
	reqCtx.Log.Info("update old primary pod to secondary success", "old primary podName", oldPrimaryPod.Name)

	// update secondary pod to primary
	if err := updateObjRoleLabel(reqCtx.Ctx, cli, *pod, constant.Primary); err != nil {
		return err
	}
	reqCtx.Log.Info("update secondary pod to primary success", "new primary podName", pod.Name)

	return nil
}

// composeReplicationRolePriorityMap generates a priority map based on roles.
func composeReplicationRolePriorityMap() map[string]int {
	return map[string]int{
		"":                 emptyReplicationPriority,
		constant.Primary:   primaryPriority,
		constant.Secondary: secondaryPriority,
	}
}

// generateReplicationParallelPlan generates a parallel plan for the replication workload.
// unknown & empty & secondary & primary
func generateReplicationParallelPlan(plan *Plan, pods []corev1.Pod, rolePriorityMap map[string]int) {
	start := plan.Start
	for _, pod := range pods {
		nextStep := &Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
	}
}

// generateReplicationSerialPlan generates a serial plan for the replication workload.
// unknown -> empty -> secondary -> primary
func generateReplicationSerialPlan(plan *Plan, pods []corev1.Pod, rolePriorityMap map[string]int) {
	start := plan.Start
	for _, pod := range pods {
		nextStep := &Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
		start = nextStep
	}
}

// generateReplicationBestEffortParallelPlan generates a best effort parallel plan for the replication workload.
// unknown & empty & 1/2 secondaries -> 1/2 secondaries -> primary
func generateReplicationBestEffortParallelPlan(plan *Plan, pods []corev1.Pod, rolePriorityMap map[string]int) {
	start := plan.Start
	l := len(pods)
	unknownEmptySteps := make([]*Step, 0, l)
	secondarySteps := make([]*Step, 0, l)
	primarySteps := make([]*Step, 0, l)

	for _, pod := range pods {
		role := pod.Labels[constant.RoleLabelKey]
		nextStep := &Step{Obj: pod}
		switch {
		case rolePriorityMap[role] <= emptyReplicationPriority:
			unknownEmptySteps = append(unknownEmptySteps, nextStep)
		case rolePriorityMap[role] < primaryPriority:
			secondarySteps = append(secondarySteps, nextStep)
		default:
			primarySteps = append(primarySteps, nextStep)
		}
	}

	// append unknown, empty
	if len(unknownEmptySteps) > 0 {
		start.NextSteps = append(start.NextSteps, unknownEmptySteps...)
		start = start.NextSteps[0]
	}

	//  append 1/2 secondaries
	end := len(secondarySteps) / 2
	if end > 0 {
		start.NextSteps = append(start.NextSteps, secondarySteps[:end]...)
		start = start.NextSteps[0]
	}

	// append the other 1/2 secondaries
	if len(secondarySteps) > end {
		start.NextSteps = append(start.NextSteps, secondarySteps[end:]...)
		start = start.NextSteps[0]
	}

	// append primary
	if len(primarySteps) > 0 {
		start.NextSteps = append(start.NextSteps, primarySteps...)
	}
}
