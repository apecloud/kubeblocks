/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package component

import (
	"context"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	replicationSetStatusDefaultPodName = "Unknown"
)

// HandleReplicationSet Handle changes in the number of replication component replicas and synchronize cluster status
// TODO(xingran) if the probe event detects an abnormal replication relationship or unavailable, it needs to be repaired in another process
func HandleReplicationSet(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	stsList []*appsv1.StatefulSet) error {

	filter := func(stsObj *appsv1.StatefulSet) (bool, error) {
		typeName := GetComponentTypeName(*cluster, stsObj.Labels[intctrlutil.AppComponentLabelKey])
		component, err := GetComponentFromClusterDefinition(reqCtx.Ctx, cli, cluster, typeName)
		if err != nil {
			return false, err
		}
		if component.ComponentType != dbaasv1alpha1.Replication {
			return true, nil
		}
		return false, nil
	}

	// handle StatefulSets including delete sts when pod number larger than cluster.component[i].replicas
	// delete the StatefulSets with the largest sequence number which is not the primary role
	clusterCompReplicasMap := make(map[string]int, len(cluster.Spec.Components))
	for _, clusterComp := range cluster.Spec.Components {
		clusterCompReplicasMap[clusterComp.Name] = int(clusterComp.Replicas)
	}

	var podList []*corev1.Pod
	compOwnsStsMap := make(map[string]int)
	stsToDeleteMap := make(map[string]int)
	for _, stsObj := range stsList {
		skip, err := filter(stsObj)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		targetPodList, err := GetPodListByStatefulSet(reqCtx.Ctx, cli, stsObj)
		if err != nil {
			return err
		}
		if len(targetPodList) != 1 {
			return fmt.Errorf("pod number in statefulset %s is not equal one", stsObj.Name)
		}
		podList = append(podList, &targetPodList[0])
		if _, ok := compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]]; !ok {
			compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] = 0
			stsToDeleteMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] = 0
		}
		compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] += 1
		if compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] > clusterCompReplicasMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] {
			stsToDeleteMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] += 1
		}
	}

	for compKey, stsToDelNum := range stsToDeleteMap {
		if stsToDelNum == 0 {
			break
		}
		// list all statefulSets by cluster and componentKey label
		allStsList, err := ListStatefulSetByClusterAndComponentLabels(reqCtx.Ctx, cli, cluster, compKey)
		if err != nil {
			return err
		}
		if compOwnsStsMap[compKey] != len(allStsList.Items) {
			return fmt.Errorf("statefulset total number has changed")
		}
		dos := make([]*appsv1.StatefulSet, 0)
		partition := len(allStsList.Items) - stsToDelNum
		for _, sts := range allStsList.Items {
			// if current primary statefulSet ordinal is larger than target number replica, return err
			if getOrdinalSts(&sts) > partition && checkStsIsPrimary(&sts) {
				return fmt.Errorf("current primary statefulset ordinal is larger than target number replicas, can not be reduce, please switchover first")
			}
			dos = append(dos, sts.DeepCopy())
		}

		// sort the statefulSets by their ordinals desc
		sort.Sort(descendingOrdinalSts(dos))

		// remove cluster status and delete sts
		err = RemoveReplicationSetClusterStatus(cli, reqCtx.Ctx, dos[:stsToDelNum])
		if err != nil {
			return err
		}
		for i := 0; i < stsToDelNum; i++ {
			if err := cli.Delete(reqCtx.Ctx, dos[i]); err != nil {
				return err
			}
		}

		return nil
	}

	// sync cluster status
	err := SyncReplicationSetClusterStatus(cli, reqCtx.Ctx, podList)
	if err != nil {
		return err
	}
	return nil
}

// SyncReplicationSetClusterStatus Sync replicationSet pod status to cluster.status.component[componentName].ReplicationStatus
func SyncReplicationSetClusterStatus(cli client.Client,
	ctx context.Context,
	podList []*corev1.Pod) error {
	if len(podList) == 0 {
		return nil
	}

	// update cluster status
	cluster := &dbaasv1alpha1.Cluster{}
	err := cli.Get(ctx, types.NamespacedName{
		Namespace: podList[0].Namespace,
		Name:      podList[0].Labels[intctrlutil.AppInstanceLabelKey],
	}, cluster)
	if err != nil {
		return err
	}

	componentName := podList[0].Labels[intctrlutil.AppComponentLabelKey]
	typeName := GetComponentTypeName(*cluster, componentName)
	componentDef, err := GetComponentFromClusterDefinition(ctx, cli, cluster, typeName)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	oldReplicationSetStatus := cluster.Status.Components[componentName].ReplicationSetStatus
	if oldReplicationSetStatus == nil {
		InitClusterComponentStatusIfNeed(cluster, componentName, componentDef)
		oldReplicationSetStatus = cluster.Status.Components[componentName].ReplicationSetStatus
	}
	needUpdate := needUpdateReplicationSetStatus(oldReplicationSetStatus, podList)
	if needUpdate {
		if err := cli.Status().Patch(ctx, cluster, patch); err != nil {
			return err
		}
	}
	return nil
}

// RemoveReplicationSetClusterStatus Remove replicationSet pod status from cluster.status.component[componentName].ReplicationStatus
func RemoveReplicationSetClusterStatus(cli client.Client, ctx context.Context, stsList []*appsv1.StatefulSet) error {
	if len(stsList) == 0 {
		return nil
	}
	var allPodList []corev1.Pod
	for _, stsObj := range stsList {
		podList, err := GetPodListByStatefulSet(ctx, cli, stsObj)
		if err != nil {
			return err
		}
		allPodList = append(allPodList, podList...)
	}
	cluster := &dbaasv1alpha1.Cluster{}
	err := cli.Get(ctx, types.NamespacedName{
		Namespace: stsList[0].Namespace,
		Name:      stsList[0].Labels[intctrlutil.AppInstanceLabelKey],
	}, cluster)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	componentName := stsList[0].Labels[intctrlutil.AppComponentLabelKey]
	replicationSetStatus := cluster.Status.Components[componentName].ReplicationSetStatus
	needRemove, err := needRemoveReplicationSetStatus(replicationSetStatus, allPodList)
	if err != nil {
		return err
	}
	if needRemove {
		if err := cli.Status().Patch(ctx, cluster, patch); err != nil {
			return err
		}
	}
	return nil
}

func needUpdateReplicationSetStatus(replicationStatus *dbaasv1alpha1.ReplicationSetStatus, podList []*corev1.Pod) bool {
	needUpdate := false
	for _, pod := range podList {
		role := pod.Labels[intctrlutil.ReplicationSetRoleLabelKey]
		if role == string(dbaasv1alpha1.Primary) {
			if replicationStatus.Primary.Pod == pod.Name && replicationStatus.Primary.Role == role {
				continue
			}
			replicationStatus.Primary.Pod = pod.Name
			replicationStatus.Primary.Role = role
			needUpdate = true
		} else {
			var exist = false
			for _, secondary := range replicationStatus.Secondaries {
				if secondary.Pod == pod.Name {
					exist = true
					if secondary.Role == role {
						continue
					}
					secondary.Role = role
					needUpdate = true
					break
				}
			}
			if !exist {
				replicationStatus.Secondaries = append(replicationStatus.Secondaries, dbaasv1alpha1.ReplicationMemberStatus{
					Pod:  pod.Name,
					Role: role,
				})
				needUpdate = true
			}
		}
	}
	return needUpdate
}

func needRemoveReplicationSetStatus(replicationStatus *dbaasv1alpha1.ReplicationSetStatus, podList []corev1.Pod) (bool, error) {
	needRemove := false
	for _, pod := range podList {
		if replicationStatus.Primary.Pod == pod.Name {
			return false, fmt.Errorf("primary pod cannot be removed")
		}
		for index, secondary := range replicationStatus.Secondaries {
			if secondary.Pod == pod.Name {
				replicationStatus.Secondaries = append(replicationStatus.Secondaries[:index], replicationStatus.Secondaries[index+1:]...)
				needRemove = true
				break
			}
		}
	}
	return needRemove, nil
}
