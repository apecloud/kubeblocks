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
	if cluster == nil {
		return util.ReqClusterObjError
	}
	// handle replication workload horizontal scaling
	if err := handleReplicationSetHorizontalScale(ctx, cli, cluster, stsList); err != nil {
		return err
	}
	return nil
}

// handleReplicationSetHorizontalScale handles changes of replication workload replicas and synchronizes cluster status.
func handleReplicationSetHorizontalScale(ctx context.Context,
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
		compName := stsObj.Labels[constant.KBAppComponentLabelKey]
		compDef, err := filterReplicationWorkload(ctx, cli, cluster, compName)
		if err != nil {
			return err
		}
		if compDef == nil {
			continue
		}
		compOwnsStsMap[compName] = append(compOwnsStsMap[compName], stsObj)
	}

	// compOwnsPodToSyncMap is used to record the list of component pods to be synchronized to cluster.status except for horizontal scale-in
	compOwnsPodsToSyncMap := make(map[string][]*corev1.Pod)
	// stsToDeleteMap is used to record the count of statefulsets to be deleted when horizontal scale-in
	stsToDeleteMap := make(map[string]int32)
	for compName, compStsObjs := range compOwnsStsMap {
		if int32(len(compOwnsStsMap[compName])) > clusterCompReplicasMap[compName] {
			stsToDeleteMap[compName] = int32(len(compOwnsStsMap[compName])) - clusterCompReplicasMap[compName]
		} else {
			for _, compStsObj := range compStsObjs {
				pod, err := getAndCheckReplicationPodByStatefulSet(ctx, cli, compStsObj)
				if err != nil {
					return err
				}
				compOwnsPodsToSyncMap[compName] = append(compOwnsPodsToSyncMap[compName], pod)
			}
		}
	}
	clusterDeepCopy := cluster.DeepCopy()
	if len(stsToDeleteMap) > 0 {
		if err := doHorizontalScaleDown(ctx, cli, cluster, compOwnsStsMap, clusterCompReplicasMap, stsToDeleteMap); err != nil {
			return err
		}
	}

	// sync cluster status
	for _, compPodList := range compOwnsPodsToSyncMap {
		if err := syncReplicationSetClusterStatus(cli, ctx, cluster, compPodList); err != nil {
			return err
		}
	}

	if reflect.DeepEqual(clusterDeepCopy.Status.Components, cluster.Status.Components) {
		return nil
	}
	return cli.Status().Patch(ctx, cluster, client.MergeFrom(clusterDeepCopy))
}

// handleComponentIsStopped checks the component status is stopped and updates it.
func handleComponentIsStopped(cluster *appsv1alpha1.Cluster) {
	for _, clusterComp := range cluster.Spec.ComponentSpecs {
		if clusterComp.Replicas == int32(0) {
			replicationStatus := cluster.Status.Components[clusterComp.Name]
			replicationStatus.Phase = appsv1alpha1.StoppedPhase
			cluster.Status.SetComponentStatus(clusterComp.Name, replicationStatus)
		}
	}
}

func doHorizontalScaleDown(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	compOwnsStsMap map[string][]*appsv1.StatefulSet,
	clusterCompReplicasMap map[string]int32,
	stsToDeleteMap map[string]int32) error {
	// remove cluster status and delete sts when horizontal scale-in
	for compName, stsToDelCount := range stsToDeleteMap {
		// list all statefulSets by cluster and componentKey label
		var componentStsList = &appsv1.StatefulSetList{}
		err := util.GetObjectListByComponentName(ctx, cli, *cluster, componentStsList, compName)
		if err != nil {
			return err
		}
		if int32(len(compOwnsStsMap[compName])) != int32(len(componentStsList.Items)) {
			return fmt.Errorf("statefulset total number has changed")
		}
		dos := make([]*appsv1.StatefulSet, 0)
		partition := int32(len(componentStsList.Items)) - stsToDelCount
		componentReplicas := clusterCompReplicasMap[compName]
		for _, sts := range componentStsList.Items {
			// if current primary statefulSet ordinal is larger than target number replica, return err
			stsIsPrimary, err := checkObjRoleLabelIsPrimary(&sts)
			if err != nil {
				return err
			}
			// check if the current primary statefulSet ordinal is larger than target replicas number of component when the target number is not 0.
			if int32(util.GetOrdinalSts(&sts)) >= partition && stsIsPrimary && componentReplicas != 0 {
				return fmt.Errorf("current primary statefulset ordinal is larger than target number replicas, can not be reduce, please switchover first")
			}
			dos = append(dos, sts.DeepCopy())
		}

		// sort the statefulSets by their ordinals desc
		sort.Sort(util.DescendingOrdinalSts(dos))

		if err = RemoveReplicationSetClusterStatus(cli, ctx, cluster, dos[:stsToDelCount], componentReplicas); err != nil {
			return err
		}
		for i := int32(0); i < stsToDelCount; i++ {
			err = cli.Delete(ctx, dos[i])
			if err == nil {
				patch := client.MergeFrom(dos[i].DeepCopy())
				controllerutil.RemoveFinalizer(dos[i], DBClusterFinalizerName)
				if err = cli.Patch(ctx, dos[i], patch); err != nil {
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

	// if component replicas is 0, handle replication component status after scaling down the replicas.
	handleComponentIsStopped(cluster)
	return nil
}

// syncReplicationSetClusterStatus syncs replicationSet pod status to cluster.status.component[componentName].ReplicationStatus.
func syncReplicationSetClusterStatus(
	cli client.Client,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	podList []*corev1.Pod) error {
	if len(podList) == 0 {
		return nil
	}

	// update cluster status
	componentName, componentDef, err := util.GetComponentInfoByPod(ctx, cli, *cluster, podList[0])
	if err != nil {
		return err
	}
	if componentDef == nil {
		return nil
	}
	oldReplicationSetStatus := cluster.Status.Components[componentName].ReplicationSetStatus
	if oldReplicationSetStatus == nil {
		util.InitClusterComponentStatusIfNeed(cluster, componentName, *componentDef)
		oldReplicationSetStatus = cluster.Status.Components[componentName].ReplicationSetStatus
	}
	if err := syncReplicationSetStatus(oldReplicationSetStatus, podList); err != nil {
		return err
	}
	return nil
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

// RemoveReplicationSetClusterStatus removes replicationSet pod status from cluster.status.component[componentName].ReplicationStatus.
func RemoveReplicationSetClusterStatus(cli client.Client,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	stsList []*appsv1.StatefulSet,
	componentReplicas int32) error {
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
	componentName := stsList[0].Labels[constant.KBAppComponentLabelKey]
	replicationSetStatus := cluster.Status.Components[componentName].ReplicationSetStatus
	return removeTargetPodsInfoInStatus(replicationSetStatus, allPodList, componentReplicas)
}

// removeTargetPodsInfoInStatus remove the target pod info from cluster.status.components.
func removeTargetPodsInfoInStatus(replicationStatus *appsv1alpha1.ReplicationSetStatus,
	targetPodList []corev1.Pod,
	componentReplicas int32) error {
	targetPodNameMap := make(map[string]struct{})
	for _, pod := range targetPodList {
		targetPodNameMap[pod.Name] = struct{}{}
	}
	if _, ok := targetPodNameMap[replicationStatus.Primary.Pod]; ok {
		if componentReplicas != 0 {
			return fmt.Errorf("primary pod cannot be removed")
		}
		replicationStatus.Primary = appsv1alpha1.ReplicationMemberStatus{
			Pod: util.ComponentStatusDefaultPodName,
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

// checkObjRoleLabelIsPrimary checks whether it is the primary obj(statefulSet or pod) through the label tag on obj.
func checkObjRoleLabelIsPrimary[T intctrlutil.Object, PT intctrlutil.PObject[T]](obj PT) (bool, error) {
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
func getReplicationSetPrimaryObj[T intctrlutil.Object, PT intctrlutil.PObject[T], L intctrlutil.ObjList[T], PL intctrlutil.PObjList[T, L]](
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
	compDef, err := util.GetComponentDefByCluster(ctx, cli, *cluster, compDefName)
	if err != nil {
		return compDef, err
	}
	if compDef == nil || compDef.WorkloadType != appsv1alpha1.Replication {
		return nil, nil
	}
	return compDef, nil
}

// getAndCheckReplicationPodByStatefulSet checks the number of replication statefulSet equal 1 and returns it.
func getAndCheckReplicationPodByStatefulSet(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet) (*corev1.Pod, error) {
	podList, err := util.GetPodListByStatefulSet(ctx, cli, stsObj)
	if err != nil {
		return nil, err
	}
	if len(podList) != 1 {
		return nil, fmt.Errorf("pod number in statefulset %s is not 1", stsObj.Name)
	}
	return &podList[0], nil
}
