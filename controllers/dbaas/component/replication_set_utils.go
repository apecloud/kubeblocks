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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	replicationSetStatusDefaultPodName = "Unknown"
)

// HandleReplicationSet Handling replicationSet component replica count changes, and sync cluster status
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

	podRoleMap := make(map[string]string)
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

		podRoleMap[targetPodList[0].Name] = targetPodList[0].Labels[intctrlutil.ReplicationSetRoleLabelKey]

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
		// list all statefulSets by componentKey label
		allStsList := &appsv1.StatefulSetList{}
		selector, err := labels.Parse(intctrlutil.AppComponentLabelKey + "=" + compKey)
		if err != nil {
			return err
		}
		if err := cli.List(reqCtx.Ctx, allStsList,
			&client.ListOptions{Namespace: cluster.Namespace},
			client.MatchingLabelsSelector{Selector: selector}); err != nil {
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

		// sort the statefulSets by their ordinals
		sort.Sort(descendingOrdinalSts(dos))

		// delete statefulSets and svc, etc
		for i := 0; i < stsToDelNum; i++ {
			if err := cli.Delete(reqCtx.Ctx, dos[i]); err != nil {
				return err
			}
			svc := &corev1.Service{}
			svcKey := types.NamespacedName{
				Namespace: cluster.Namespace,
				Name:      fmt.Sprintf("%s-%d", dos[i].Name, 0),
			}
			if err := cli.Get(reqCtx.Ctx, svcKey, svc); err != nil {
				return err
			}
			if err := cli.Delete(reqCtx.Ctx, svc); err != nil {
				return err
			}
		}
	}

	// sync cluster status
	for podName, role := range podRoleMap {
		podNSObj := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      podName,
		}
		err := SyncReplicationSetClusterStatus(cli, reqCtx.Ctx, podNSObj, role)
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncReplicationSetClusterStatus Sync replicationSet status to cluster.status.component[componentName].ReplicationStatus
func SyncReplicationSetClusterStatus(cli client.Client, ctx context.Context, podName types.NamespacedName, role string) error {
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, podName, pod); err != nil {
		return err
	}
	// update cluster status
	cluster := &dbaasv1alpha1.Cluster{}
	err := cli.Get(ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Labels[intctrlutil.AppInstanceLabelKey],
	}, cluster)
	if err != nil {
		return err
	}

	componentName := pod.Labels[intctrlutil.AppComponentLabelKey]
	typeName := GetComponentTypeName(*cluster, componentName)
	componentDef, err := GetComponentFromClusterDefinition(ctx, cli, cluster, typeName)
	if err != nil {
		return err
	}

	patch := client.MergeFrom(cluster.DeepCopy())
	InitClusterComponentStatusIfNeed(cluster, componentName, componentDef)
	replicationSetStatus := cluster.Status.Components[componentName].ReplicationSetStatus
	needUpdate := needUpdateReplicationSetStatus(replicationSetStatus, role, pod.Name)
	if !needUpdate {
		return nil
	}
	if err := cli.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	return nil
}

func needUpdateReplicationSetStatus(replicationStatus *dbaasv1alpha1.ReplicationSetStatus, role, podName string) bool {
	if role == string(dbaasv1alpha1.Primary) {
		if replicationStatus.Primary.Pod == podName && replicationStatus.Primary.Role == role {
			return false
		}
		replicationStatus.Primary.Pod = podName
		replicationStatus.Primary.Role = role
		return true
	} else {
		var exist = false
		for _, secondary := range replicationStatus.Secondaries {
			if secondary.Pod == podName {
				if secondary.Role == role {
					return false
				}
				exist = true
				secondary.Role = role
			}
		}
		if !exist {
			replicationStatus.Secondaries = append(replicationStatus.Secondaries, dbaasv1alpha1.ReplicationMemberStatus{
				Pod:  podName,
				Role: role,
			})
		}
		return true
	}
}
