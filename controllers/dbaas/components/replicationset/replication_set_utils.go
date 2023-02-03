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

package replicationset

import (
	"context"
	"fmt"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ReplicationRole string

const (
	Primary                ReplicationRole = "primary"
	Secondary              ReplicationRole = "secondary"
	DBClusterFinalizerName                 = "cluster.kubeblocks.io/finalizer"
)

// HandleReplicationSet handles changes of replication component replicas and synchronizes cluster status.
// TODO(xingran) if the probe event detects an abnormal replication relationship or unavailable, it needs to be repaired in another process.
func HandleReplicationSet(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	stsList []*appsv1.StatefulSet) error {

	filter := func(stsObj *appsv1.StatefulSet) (bool, error) {
		typeName := util.GetComponentTypeName(*cluster, stsObj.Labels[intctrlutil.AppComponentLabelKey])
		component, err := util.GetComponentDefByCluster(reqCtx.Ctx, cli, cluster, typeName)
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
	clusterCompReplicasMap := make(map[string]int32, len(cluster.Spec.Components))
	for _, clusterComp := range cluster.Spec.Components {
		if clusterComp.Replicas == nil {
			defaultReplicas, err := util.GetComponentDefaultReplicas(reqCtx.Ctx, cli, cluster, clusterComp.Type)
			if err != nil {
				return err
			}
			clusterComp.Replicas = &defaultReplicas
		}
		clusterCompReplicasMap[clusterComp.Name] = *clusterComp.Replicas
	}

	// compOwnsStsMap is used to divide stsList into sts list under each replicationSet component according to componentLabelKey
	compOwnsStsMap := make(map[string][]*appsv1.StatefulSet)
	for _, stsObj := range stsList {
		skip, err := filter(stsObj)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]] = append(compOwnsStsMap[stsObj.Labels[intctrlutil.AppComponentLabelKey]], stsObj)
	}

	// compOwnsPodToSyncMap is used to record the list of component pods to be synchronized to cluster.status except for horizontal scale-in
	compOwnsPodsToSyncMap := make(map[string][]*corev1.Pod)
	// stsToDeleteMap is used to record the count of statefulsets to be deleted when horizontal scale-in
	stsToDeleteMap := make(map[string]int32)
	for compKey, compStsObjs := range compOwnsStsMap {
		if int32(len(compOwnsStsMap[compKey])) > clusterCompReplicasMap[compKey] {
			stsToDeleteMap[compKey] = int32(len(compOwnsStsMap[compKey])) - clusterCompReplicasMap[compKey]
		} else {
			for _, compStsObj := range compStsObjs {
				targetPodList, err := util.GetPodListByStatefulSet(reqCtx.Ctx, cli, compStsObj)
				if err != nil {
					return err
				}
				if len(targetPodList) != 1 {
					return fmt.Errorf("pod number in statefulset %s is not 1", compStsObj.Name)
				}
				compOwnsPodsToSyncMap[compKey] = append(compOwnsPodsToSyncMap[compKey], &targetPodList[0])
			}
		}
	}

	// remove cluster status and delete sts when horizontal scale-in
	for compKey, stsToDelCount := range stsToDeleteMap {
		// list all statefulSets by cluster and componentKey label
		var componentStsList = &appsv1.StatefulSetList{}
		err := util.GetObjectListByComponentName(reqCtx.Ctx, cli, cluster, componentStsList, compKey)
		if err != nil {
			return err
		}
		if int32(len(compOwnsStsMap[compKey])) != int32(len(componentStsList.Items)) {
			return fmt.Errorf("statefulset total number has changed")
		}
		dos := make([]*appsv1.StatefulSet, 0)
		partition := int32(len(componentStsList.Items)) - stsToDelCount
		for _, sts := range componentStsList.Items {
			// if current primary statefulSet ordinal is larger than target number replica, return err
			if int32(util.GetOrdinalSts(&sts)) >= partition && CheckStsIsPrimary(&sts) {
				return fmt.Errorf("current primary statefulset ordinal is larger than target number replicas, can not be reduce, please switchover first")
			}
			dos = append(dos, sts.DeepCopy())
		}

		// sort the statefulSets by their ordinals desc
		sort.Sort(util.DescendingOrdinalSts(dos))

		if err := RemoveReplicationSetClusterStatus(cli, reqCtx.Ctx, dos[:stsToDelCount]); err != nil {
			return err
		}
		for i := int32(0); i < stsToDelCount; i++ {
			err := cli.Delete(reqCtx.Ctx, dos[i])
			if err == nil {
				patch := client.MergeFrom(dos[i].DeepCopy())
				controllerutil.RemoveFinalizer(dos[i], DBClusterFinalizerName)
				if err := cli.Patch(reqCtx.Ctx, dos[i], patch); err != nil {
					return err
				}
				continue
			}
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
	}

	// sync cluster status
	for _, compPodList := range compOwnsPodsToSyncMap {
		if err := SyncReplicationSetClusterStatus(cli, reqCtx.Ctx, compPodList); err != nil {
			return err
		}
	}
	return nil
}

// SyncReplicationSetClusterStatus syncs replicationSet pod status to cluster.status.component[componentName].ReplicationStatus.
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
	componentName, componentDef, err := util.GetComponentInfoByPod(ctx, cli, cluster, podList[0])
	if err != nil {
		return err
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	oldReplicationSetStatus := cluster.Status.Components[componentName].ReplicationSetStatus
	if oldReplicationSetStatus == nil {
		util.InitClusterComponentStatusIfNeed(cluster, componentName, componentDef)
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

// RemoveReplicationSetClusterStatus removes replicationSet pod status from cluster.status.component[componentName].ReplicationStatus.
func RemoveReplicationSetClusterStatus(cli client.Client, ctx context.Context, stsList []*appsv1.StatefulSet) error {
	if len(stsList) == 0 {
		return nil
	}
	var allPodList []corev1.Pod
	for _, stsObj := range stsList {
		podList, err := util.GetPodListByStatefulSet(ctx, cli, stsObj)
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

// needUpdateReplicationSetStatus checks if the target pod node needs to be updated in cluster.status.
func needUpdateReplicationSetStatus(replicationStatus *dbaasv1alpha1.ReplicationSetStatus, podList []*corev1.Pod) bool {
	needUpdate := false
	for _, pod := range podList {
		role := pod.Labels[intctrlutil.RoleLabelKey]
		if role == string(Primary) {
			if replicationStatus.Primary.Pod == pod.Name {
				continue
			}
			replicationStatus.Primary.Pod = pod.Name
			needUpdate = true
		} else {
			var exist = false
			for _, secondary := range replicationStatus.Secondaries {
				if secondary.Pod == pod.Name {
					exist = true
					break
				}
			}
			if !exist {
				replicationStatus.Secondaries = append(replicationStatus.Secondaries, dbaasv1alpha1.ReplicationMemberStatus{
					Pod: pod.Name,
				})
				needUpdate = true
			}
		}
	}
	return needUpdate
}

// needRemoveReplicationSetStatus checks if the target pod node needs to be removed from cluster.status.
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

// CheckStsIsPrimary checks whether it is the primary statefulSet through the label tag on sts.
func CheckStsIsPrimary(sts *appsv1.StatefulSet) bool {
	if sts != nil && sts.Labels != nil {
		return sts.Labels[intctrlutil.RoleLabelKey] == string(Primary)
	}
	return false
}

// GeneratePVCFromVolumeClaimTemplates generates the required pvc object according to the name of statefulSet and volumeClaimTemplates.
func GeneratePVCFromVolumeClaimTemplates(sts *appsv1.StatefulSet, vctList []corev1.PersistentVolumeClaimTemplate) map[string]*corev1.PersistentVolumeClaim {
	claims := make(map[string]*corev1.PersistentVolumeClaim, len(vctList))
	for index := range vctList {
		claim := &corev1.PersistentVolumeClaim{
			Spec: vctList[index].Spec,
		}
		// The replica of replicationSet statefulSet defaults to 1, so the ordinal here is 0
		claim.Name = GetPersistentVolumeClaimName(sts, &vctList[index], 0)
		claim.Namespace = sts.Namespace
		claims[vctList[index].Name] = claim
	}
	return claims
}

// GetPersistentVolumeClaimName gets the name of PersistentVolumeClaim for a replicationSet pod with an ordinal.
// claimTpl must be a PersistentVolumeClaimTemplate from the VolumeClaimsTemplate in the Cluster API.
func GetPersistentVolumeClaimName(sts *appsv1.StatefulSet, claimTpl *corev1.PersistentVolumeClaimTemplate, ordinal int) string {
	return fmt.Sprintf("%s-%s-%d", claimTpl.Name, sts.Name, ordinal)
}
