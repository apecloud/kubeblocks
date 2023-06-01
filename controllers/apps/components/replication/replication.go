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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReplicationSet is a component object used by Cluster, ClusterComponentDefinition and ClusterComponentSpec
type ReplicationSet struct {
	stateful.Stateful
}

var _ internal.ComponentSet = &ReplicationSet{}

func (r *ReplicationSet) getName() string {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Name
	}
	return r.ComponentSpec.Name
}

func (r *ReplicationSet) getWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Replication
}

func (r *ReplicationSet) getReplicas() int32 {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Replicas
	}
	return r.ComponentSpec.Replicas
}

func (r *ReplicationSet) getPrimaryIndex() int32 {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.GetPrimaryIndex()
	}
	return r.ComponentSpec.GetPrimaryIndex()
}

// IsRunning is the implementation of the type Component interface method,
// which is used to check whether the replicationSet component is running normally.
func (r *ReplicationSet) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	var componentStatusIsRunning = true
	sts := util.ConvertToStatefulSet(obj)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	stsIsReady := util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, nil)
	if !stsIsReady {
		return false, nil
	}
	if sts.Status.AvailableReplicas < r.getReplicas() {
		componentStatusIsRunning = false
	}
	return componentStatusIsRunning, nil
}

// PodsReady is the implementation of the type Component interface method,
// which is used to check whether all the pods of replicationSet component are ready.
func (r *ReplicationSet) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	return r.Stateful.PodsReady(ctx, obj)
}

// PodIsAvailable is the implementation of the type Component interface method,
// Check whether the status of a Pod of the replicationSet is ready, including the role label on the Pod
func (r *ReplicationSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

func (r *ReplicationSet) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	return "", nil
}

// GetPhaseWhenPodsNotReady is the implementation of the type Component interface method,
// when the pods of replicationSet are not ready, calculate the component phase is Failed or Abnormal.
// if return an empty phase, means the pods of component are ready and skips it.
func (r *ReplicationSet) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(ctx, r.Cli, *r.Cluster,
		componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", nil, err
	}
	stsObj := stsList.Items[0]
	podCount := len(podList.Items)
	componentReplicas := r.getReplicas()
	if podCount == 0 || stsObj.Status.AvailableReplicas == 0 {
		return util.GetPhaseWithNoAvailableReplicas(componentReplicas), nil, nil
	}
	// get the statefulSet of component
	var (
		existLatestRevisionFailedPod bool
		primaryIsReady               bool
		statusMessages               appsv1alpha1.ComponentMessageMap
	)
	for _, v := range podList.Items {
		// if the pod is terminating, ignore it
		if v.DeletionTimestamp != nil {
			return "", nil, nil
		}
		labelValue := v.Labels[constant.RoleLabelKey]
		if labelValue == string(Primary) && intctrlutil.PodIsReady(&v) {
			primaryIsReady = true
			continue
		}
		if labelValue == "" {
			// REVIEW: this isn't a get function, where r.Cluster.Status.Components is being updated.
			// patch abnormal reason to cluster.status.ComponentDefs.
			if statusMessages == nil {
				statusMessages = appsv1alpha1.ComponentMessageMap{}
			}
			statusMessages.SetObjectMessage(v.Kind, v.Name, "empty label for pod, please check.")
		}
		if !intctrlutil.PodIsReady(&v) && intctrlutil.PodIsControlledByLatestRevision(&v, &stsObj) {
			existLatestRevisionFailedPod = true
		}
	}
	return util.GetCompPhaseByConditions(existLatestRevisionFailedPod, primaryIsReady,
		componentReplicas, int32(podCount), stsObj.Status.AvailableReplicas), statusMessages, nil
}

func (r *ReplicationSet) HandleRestart(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	sts := util.ConvertToStatefulSet(obj)
	if sts.Generation != sts.Status.ObservedGeneration {
		return nil, nil
	}
	vertexes := make([]graph.Vertex, 0)
	pods, err := util.GetPods4Delete(ctx, r.Cli, sts)
	if err != nil {
		return nil, err
	}
	for _, pod := range pods {
		vertexes = append(vertexes, &ictrltypes.LifecycleVertex{
			Obj:    pod,
			Action: ictrltypes.ActionDeletePtr(),
			Orphan: true,
		})
	}
	return vertexes, nil
}

func (r *ReplicationSet) HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	podList, err := r.getRunningPods(ctx, obj)
	if err != nil {
		return nil, err
	}
	if len(podList) == 0 {
		return nil, nil
	}
	vertexes := make([]graph.Vertex, 0)
	podsToSyncStatus := make([]*corev1.Pod, 0)
	for i := range podList {
		pod := &podList[i]
		// if there is no role label on the Pod, it needs to be updated with statefulSet's role label.
		if v, ok := pod.Labels[constant.RoleLabelKey]; !ok || v == "" {
			_, o := util.ParseParentNameAndOrdinal(pod.Name)
			role := string(Secondary)
			if o == r.getPrimaryIndex() {
				role = string(Primary)
			}
			pod.GetLabels()[constant.RoleLabelKey] = role
			vertexes = append(vertexes, &ictrltypes.LifecycleVertex{
				Obj:    pod,
				Action: ictrltypes.ActionUpdatePtr(), // update or patch?
			})
		}
		// else {
		//	podsToSyncStatus = append(podsToSyncStatus, pod)
		// }
		podsToSyncStatus = append(podsToSyncStatus, pod)
	}
	// // REVIEW/TODO: (Y-Rookie)
	// //  1. should employ rolling deletion as default strategy instead of delete them all.
	// if err := util.DeleteStsPods(ctx, r.Cli, sts); err != nil {
	// 	return err
	// }
	// sync cluster.spec.componentSpecs.[x].primaryIndex when failover occurs and switchPolicy is Noop.
	// TODO(refactor): syncPrimaryIndex will update cluster spec, resolve it.
	if err := syncPrimaryIndex(ctx, r.Cli, r.Cluster, r.getName()); err != nil {
		return nil, err
	}
	// sync cluster.status.components.replicationSet.status
	if err := syncReplicationSetClusterStatus(r.Cluster, r.getWorkloadType(), r.getName(), podsToSyncStatus); err != nil {
		return nil, err
	}
	return vertexes, nil
}

// TODO(refactor): imple HandleHA asynchronously

func (r *ReplicationSet) HandleHA(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	pods, err := r.getRunningPods(ctx, obj)
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		return nil, nil
	}
	// If the Pods already exists, check whether there is a HA switching and the HA process is prioritized to handle.
	// TODO(xingran) After refactoring, HA switching will be handled in the replicationSet controller.
	primaryIndexChanged, _, err := CheckPrimaryIndexChanged(ctx, r.Cli, r.Cluster, r.getName(), r.getPrimaryIndex())
	if err != nil {
		return nil, err
	}
	if primaryIndexChanged {
		compSpec := util.GetClusterComponentSpecByName(*r.Cluster, r.getName())
		if err := HandleReplicationSetHASwitch(ctx, r.Cli, r.Cluster, compSpec); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (r *ReplicationSet) getRunningPods(ctx context.Context, obj client.Object) ([]corev1.Pod, error) {
	sts := util.ConvertToStatefulSet(obj)
	if sts.Generation != sts.Status.ObservedGeneration {
		return nil, nil
	}
	return util.GetPodListByStatefulSet(ctx, r.Cli, sts)
}

func newReplicationSet(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *ReplicationSet {
	return &ReplicationSet{
		Stateful: stateful.Stateful{
			ComponentSetBase: internal.ComponentSetBase{
				Cli:                  cli,
				Cluster:              cluster,
				SynthesizedComponent: nil,
				ComponentSpec:        spec,
				ComponentDef:         &def,
			},
		},
	}
}

func DefaultRole(i int32) string {
	role := string(Secondary)
	if i == 0 {
		role = string(Primary)
	}
	return role
}
