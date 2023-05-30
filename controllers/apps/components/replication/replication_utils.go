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

package replication

import (
	"context"
	"fmt"
	"reflect"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type ReplicationRole string

const (
	Primary   ReplicationRole = "primary"
	Secondary ReplicationRole = "secondary"
)

// syncReplicationSetClusterStatus syncs replicationSet pod status to cluster.status.component[componentName].ReplicationStatus.
func syncReplicationSetClusterStatus(cluster *appsv1alpha1.Cluster,
	workloadType appsv1alpha1.WorkloadType, compName string, podList []*corev1.Pod) error {
	if len(podList) == 0 {
		return nil
	}

	replicationStatus := cluster.Status.Components[compName].ReplicationSetStatus
	if replicationStatus == nil {
		if err := util.InitClusterComponentStatusIfNeed(cluster, compName, workloadType); err != nil {
			return err
		}
		replicationStatus = cluster.Status.Components[compName].ReplicationSetStatus
	}
	return syncReplicationSetStatus(replicationStatus, podList)
}

// syncReplicationSetStatus syncs the target pod info in cluster.status.components.
func syncReplicationSetStatus(replicationStatus *appsv1alpha1.ReplicationSetStatus, podList []*corev1.Pod) error {
	for _, pod := range podList {
		role := pod.Labels[constant.RoleLabelKey]
		if role == "" {
			return fmt.Errorf("pod %s has no role label", pod.Name)
		}
		if role == string(Primary) {
			if replicationStatus.Primary.Pod == pod.Name {
				continue
			}
			replicationStatus.Primary.Pod = pod.Name
			// if current primary pod in secondary list, it means the primary pod has been switched, remove it.
			for index, secondary := range replicationStatus.Secondaries {
				if secondary.Pod == pod.Name {
					replicationStatus.Secondaries = append(replicationStatus.Secondaries[:index], replicationStatus.Secondaries[index+1:]...)
					break
				}
			}
		} else {
			var exist = false
			for _, secondary := range replicationStatus.Secondaries {
				if secondary.Pod == pod.Name {
					exist = true
					break
				}
			}
			if !exist {
				replicationStatus.Secondaries = append(replicationStatus.Secondaries, appsv1alpha1.ReplicationMemberStatus{
					Pod: pod.Name,
				})
			}
		}
	}
	return nil
}

// removeTargetPodsInfoInStatus removes the target pod info from cluster.status.components.
func removeTargetPodsInfoInStatus(replicationStatus *appsv1alpha1.ReplicationSetStatus,
	targetPodList []*corev1.Pod,
	componentReplicas int32) error {
	if replicationStatus == nil {
		return nil
	}
	targetPodNameMap := make(map[string]struct{})
	for _, pod := range targetPodList {
		targetPodNameMap[pod.Name] = struct{}{}
	}
	if _, ok := targetPodNameMap[replicationStatus.Primary.Pod]; ok {
		if componentReplicas != 0 {
			return fmt.Errorf("primary pod cannot be removed")
		}
		replicationStatus.Primary = appsv1alpha1.ReplicationMemberStatus{
			Pod: constant.ComponentStatusDefaultPodName,
		}
	}
	newSecondaries := make([]appsv1alpha1.ReplicationMemberStatus, 0)
	for _, secondary := range replicationStatus.Secondaries {
		if _, ok := targetPodNameMap[secondary.Pod]; ok {
			continue
		}
		// add pod that do not need to be removed to newSecondaries slice.
		newSecondaries = append(newSecondaries, secondary)
	}
	replicationStatus.Secondaries = newSecondaries
	return nil
}

// checkObjRoleLabelIsPrimary checks whether it is the primary obj(statefulSet or pod) by the label tag on obj.
func checkObjRoleLabelIsPrimary[T generics.Object, PT generics.PObject[T]](obj PT) (bool, error) {
	if obj == nil || obj.GetLabels() == nil {
		// REVIEW/TODO: need avoid using dynamic error string, this is bad for
		// error type checking (errors.Is)
		return false, fmt.Errorf("obj %s or obj's labels is nil, pls check", obj.GetName())
	}
	if _, ok := obj.GetLabels()[constant.RoleLabelKey]; !ok {
		// REVIEW/TODO: need avoid using dynamic error string, this is bad for
		// error type checking (errors.Is)
		return false, fmt.Errorf("obj %s or obj labels key is nil, pls check", obj.GetName())
	}
	return obj.GetLabels()[constant.RoleLabelKey] == string(Primary), nil
}

// getReplicationSetPrimaryObj gets the primary obj(statefulSet or pod) of the replication workload.
func getReplicationSetPrimaryObj[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](
	ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, _ func(T, L), compSpecName string) (PT, error) {
	var (
		objList L
	)
	matchLabels := client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.KBAppComponentLabelKey: compSpecName,
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.RoleLabelKey:           string(Primary),
	}
	if err := cli.List(ctx, PL(&objList), client.InNamespace(cluster.Namespace), matchLabels); err != nil {
		return nil, err
	}
	objListItems := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
	if len(objListItems) != 1 {
		// TODO:(xingran) Temporary modification to fix the issue where the cluster state cannot reach the final state
		// due to the update order of the role label. Subsequent PR will immediately reconstruct this part.
		return nil, nil
		// return nil, fmt.Errorf("the number of current replicationSet primary obj is not 1, pls check")
	}
	return &objListItems[0], nil
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

// filterReplicationWorkload filters workload which workloadType is not Replication.
func filterReplicationWorkload(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	compSpecName string) (*appsv1alpha1.ClusterComponentDefinition, error) {
	if compSpecName == "" {
		return nil, fmt.Errorf("cluster's compSpecName is nil, pls check")
	}
	compDefName := cluster.Spec.GetComponentDefRefName(compSpecName)
	compDef, err := util.GetComponentDefByCluster(ctx, cli, *cluster, compDefName)
	if err != nil {
		return compDef, err
	}
	if compDef == nil || compDef.WorkloadType != appsv1alpha1.Replication {
		return nil, nil
	}
	return compDef, nil
}

// HandleReplicationSetRoleChangeEvent handles the role change event of the replication workload when switchPolicy is Noop.
func HandleReplicationSetRoleChangeEvent(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	cluster *appsv1alpha1.Cluster,
	compName string,
	pod *corev1.Pod,
	newRole string) error {
	// if newRole is not Primary or Secondary, ignore it.
	if !slices.Contains([]string{string(Primary), string(Secondary)}, newRole) {
		reqCtx.Log.Info("replicationSet new role is invalid, please check", "new role", newRole)
		return nil
	}
	// if pod current role label equals to newRole, return
	if pod.Labels[constant.RoleLabelKey] == newRole {
		reqCtx.Log.Info("pod current role label equals to new role, ignore it", "new role", newRole)
		return nil
	}
	// if switchPolicy is not Noop, return
	clusterCompSpec := util.GetClusterComponentSpecByName(*cluster, compName)
	if clusterCompSpec == nil || clusterCompSpec.SwitchPolicy == nil || clusterCompSpec.SwitchPolicy.Type != appsv1alpha1.Noop {
		reqCtx.Log.Info("cluster switchPolicy is not Noop, does not support handling role change event", "cluster", cluster.Name)
		return nil
	}

	oldPrimaryPod, err := getReplicationSetPrimaryObj(reqCtx.Ctx, cli, cluster, generics.PodSignature, compName)
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
	if err := updateObjRoleLabel(reqCtx.Ctx, cli, *oldPrimaryPod, string(Secondary)); err != nil {
		return err
	}

	// update secondary pod to primary
	if err := updateObjRoleLabel(reqCtx.Ctx, cli, *pod, string(Primary)); err != nil {
		return err
	}
	return nil
}
