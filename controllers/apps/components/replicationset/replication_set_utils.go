/*
Copyright ApeCloud, Inc.

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
	"reflect"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
)

type ReplicationRole string

const (
	Primary                ReplicationRole = "primary"
	Secondary              ReplicationRole = "secondary"
	DBClusterFinalizerName                 = "cluster.kubeblocks.io/finalizer"
)

// HandleReplicationSet handles the replication workload life cycle process, including horizontal scaling, etc.
func HandleReplicationSet(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	stsList []*appsv1.StatefulSet) error {

	// handle replication workload horizontal scaling
	if err := HandleReplicationSetHorizontalScale(ctx, cli, cluster, stsList); err != nil {
		return err
	}

	return nil
}

// HandleReplicationSetHorizontalScale handles changes of replication workload replicas and synchronizes cluster status.
func HandleReplicationSetHorizontalScale(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	stsList []*appsv1.StatefulSet) error {

	// handle StatefulSets including delete sts when pod number larger than cluster.component[i].replicas
	// delete the StatefulSets with the largest sequence number which is not the primary role
	clusterCompReplicasMap := make(map[string]int32, len(cluster.Spec.ComponentSpecs))
	for _, clusterComp := range cluster.Spec.ComponentSpecs {
		clusterCompReplicasMap[clusterComp.Name] = clusterComp.Replicas
	}

	// compOwnsStsMap is used to divide stsList into sts list under each replicationSet component according to componentLabelKey
	compOwnsStsMap := make(map[string][]*appsv1.StatefulSet)
	for _, stsObj := range stsList {
		compDef, err := filterReplicationWorkload(ctx, cli, cluster, stsObj.Labels[constant.KBAppComponentLabelKey])
		if err != nil {
			return err
		}
		if compDef == nil {
			continue
		}
		compOwnsStsMap[stsObj.Labels[constant.KBAppComponentLabelKey]] = append(compOwnsStsMap[stsObj.Labels[constant.KBAppComponentLabelKey]], stsObj)
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
				pod, err := GetAndCheckReplicationPodByStatefulSet(ctx, cli, compStsObj)
				if err != nil {
					return err
				}
				compOwnsPodsToSyncMap[compKey] = append(compOwnsPodsToSyncMap[compKey], pod)
			}
		}
	}

	// remove cluster status and delete sts when horizontal scale-in
	for compKey, stsToDelCount := range stsToDeleteMap {
		// list all statefulSets by cluster and componentKey label
		var componentStsList = &appsv1.StatefulSetList{}
		err := util.GetObjectListByComponentName(ctx, cli, cluster, componentStsList, compKey)
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
			stsIsPrimary, err := checkObjRoleLabelIsPrimary(&sts)
			if err != nil {
				return err
			}
			if int32(util.GetOrdinalSts(&sts)) >= partition && stsIsPrimary {
				return fmt.Errorf("current primary statefulset ordinal is larger than target number replicas, can not be reduce, please switchover first")
			}
			dos = append(dos, sts.DeepCopy())
		}

		// sort the statefulSets by their ordinals desc
		sort.Sort(util.DescendingOrdinalSts(dos))

		if err := RemoveReplicationSetClusterStatus(cli, ctx, dos[:stsToDelCount]); err != nil {
			return err
		}
		for i := int32(0); i < stsToDelCount; i++ {
			err := cli.Delete(ctx, dos[i])
			if err == nil {
				patch := client.MergeFrom(dos[i].DeepCopy())
				controllerutil.RemoveFinalizer(dos[i], DBClusterFinalizerName)
				if err := cli.Patch(ctx, dos[i], patch); err != nil {
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
		if err := SyncReplicationSetClusterStatus(cli, ctx, compPodList); err != nil {
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
	cluster := &appsv1alpha1.Cluster{}
	err := cli.Get(ctx, types.NamespacedName{
		Namespace: podList[0].Namespace,
		Name:      podList[0].Labels[constant.AppInstanceLabelKey],
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
	cluster := &appsv1alpha1.Cluster{}
	err := cli.Get(ctx, types.NamespacedName{
		Namespace: stsList[0].Namespace,
		Name:      stsList[0].Labels[constant.AppInstanceLabelKey],
	}, cluster)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	componentName := stsList[0].Labels[constant.KBAppComponentLabelKey]
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
func needUpdateReplicationSetStatus(replicationStatus *appsv1alpha1.ReplicationSetStatus, podList []*corev1.Pod) bool {
	needUpdate := false
	for _, pod := range podList {
		role := pod.Labels[constant.RoleLabelKey]
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
				replicationStatus.Secondaries = append(replicationStatus.Secondaries, appsv1alpha1.ReplicationMemberStatus{
					Pod: pod.Name,
				})
				needUpdate = true
			}
		}
	}
	return needUpdate
}

// needRemoveReplicationSetStatus checks if the target pod node needs to be removed from cluster.status.
func needRemoveReplicationSetStatus(replicationStatus *appsv1alpha1.ReplicationSetStatus, podList []corev1.Pod) (bool, error) {
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

// checkObjRoleLabelIsPrimary checks whether it is the primary obj(statefulSet or pod) through the label tag on obj.
func checkObjRoleLabelIsPrimary[T intctrlutil.Object, PT intctrlutil.PObject[T]](obj PT) (bool, error) {
	if obj == nil || obj.GetLabels() == nil {
		return false, fmt.Errorf("obj %s or obj's labels is nil, pls check", obj.GetName())
	}
	if _, ok := obj.GetLabels()[constant.RoleLabelKey]; !ok {
		return false, fmt.Errorf("obj %s or obj labels key is nil, pls check", obj.GetName())
	}
	return obj.GetLabels()[constant.RoleLabelKey] == string(Primary), nil
}

// GetReplicationSetPrimaryObj gets the primary obj(statefulSet or pod) of the replication workload.
func GetReplicationSetPrimaryObj[T intctrlutil.Object, PT intctrlutil.PObject[T], L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](
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
		return nil, fmt.Errorf("the number of current replicationSet primary obj is not 1, pls check")
	}
	return &objListItems[0], nil
}

// updateObjRoleLabel updates the value of the role label of the object.
func updateObjRoleLabel[T intctrlutil.Object, PT intctrlutil.PObject[T]](
	ctx context.Context, cli client.Client, obj T, role string) error {
	pObj := PT(&obj)
	patch := client.MergeFrom(PT(pObj.DeepCopy()))
	pObj.GetLabels()[constant.RoleLabelKey] = role
	if err := cli.Patch(ctx, pObj, patch); err != nil {
		return err
	}
	return nil
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

// filterReplicationWorkload filters workload which workloadType is not Replication.
func filterReplicationWorkload(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	compSpecName string) (*appsv1alpha1.ClusterComponentDefinition, error) {
	if compSpecName == "" {
		return nil, fmt.Errorf("cluster's compSpecName is nil, pls check")
	}
	compDefName := cluster.GetComponentDefRefName(compSpecName)
	compDef, err := util.GetComponentDefByCluster(ctx, cli, cluster, compDefName)
	if err != nil {
		return compDef, err
	}
	if compDef == nil || compDef.WorkloadType != appsv1alpha1.Replication {
		return nil, nil
	}
	return compDef, nil
}

// GetAndCheckReplicationPodByStatefulSet checks the number of replication statefulSet equal 1 and returns it.
func GetAndCheckReplicationPodByStatefulSet(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet) (*corev1.Pod, error) {
	podList, err := util.GetPodListByStatefulSet(ctx, cli, stsObj)
	if err != nil {
		return nil, err
	}
	if len(podList) != 1 {
		return nil, fmt.Errorf("pod number in statefulset %s is not 1", stsObj.Name)
	}
	return &podList[0], nil
}
