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
	"time"

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

// updateObjRoleChangedInfo updates the value of the role label and annotation of the object.
func updateObjRoleChangedInfo[T generics.Object, PT generics.PObject[T]](
	ctx context.Context, cli client.Client, event *corev1.Event, obj T, role string) error {
	pObj := PT(&obj)
	patch := client.MergeFrom(PT(pObj.DeepCopy()))
	pObj.GetLabels()[constant.RoleLabelKey] = role
	if pObj.GetAnnotations() == nil {
		pObj.SetAnnotations(map[string]string{})
	}
	pObj.GetAnnotations()[constant.LastRoleChangedEventTimestampAnnotationKey] = event.EventTime.Time.Format(time.RFC3339Nano)
	if err := cli.Patch(ctx, pObj, patch); err != nil {
		return err
	}
	return nil
}

// HandleReplicationSetRoleChangeEvent handles the role change event of the replication workload when switchPolicy is Noop.
func HandleReplicationSetRoleChangeEvent(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	event *corev1.Event,
	cluster *appsv1alpha1.Cluster,
	compName string,
	pod *corev1.Pod,
	newRole string) error {
	reqCtx.Log.Info("receive role change event", "podName", pod.Name, "current pod role label", pod.Labels[constant.RoleLabelKey], "new role", newRole)
	// if newRole is not Primary or Secondary, ignore it.
	if !slices.Contains([]string{constant.Primary, constant.Secondary}, newRole) {
		reqCtx.Log.Info("replicationSet new role is invalid, please check", "new role", newRole)
		return nil
	}

	// if switchPolicy is not Noop, return
	clusterCompSpec := getClusterComponentSpecByName(*cluster, compName)
	if clusterCompSpec == nil || clusterCompSpec.SwitchPolicy == nil || clusterCompSpec.SwitchPolicy.Type != appsv1alpha1.Noop {
		reqCtx.Log.Info("cluster switchPolicy is not Noop, does not support handling role change event", "cluster", cluster.Name)
		return nil
	}

	// update pod role label with newRole
	if err := updateObjRoleChangedInfo(reqCtx.Ctx, cli, event, *pod, newRole); err != nil {
		reqCtx.Log.Info("failed to update pod role label", "podName", pod.Name, "newRole", newRole, "err", err)
		return err
	}
	reqCtx.Log.Info("succeed to update pod role label", "podName", pod.Name, "newRole", newRole)
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
