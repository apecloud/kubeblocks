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
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
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

var _ types.ComponentSet = &ReplicationSet{}

func (r *ReplicationSet) getName() string {
	if r.Component != nil {
		return r.Component.GetName()
	}
	return r.ComponentSpec.Name
}

func (r *ReplicationSet) getWorkloadType() appsv1alpha1.WorkloadType {
	if r.Component != nil {
		return r.Component.GetWorkloadType()
	}
	return r.ComponentDef.WorkloadType
}

func (r *ReplicationSet) getReplicas() int32 {
	if r.Component != nil {
		return r.Component.GetReplicas()
	}
	return r.ComponentSpec.Replicas
}

func (r *ReplicationSet) SetComponent(comp types.Component) {
	r.Component = comp
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
		if labelValue == constant.Primary && intctrlutil.PodIsReady(&v) {
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

// HandleRestart is the implementation of the type Component interface method, which is used to handle the restart of the Replication workload.
// TODO(xingran): handle the restart of the Replication workload with rolling update by Pod role.
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

// HandleRoleChange is the implementation of the type Component interface method, which is used to handle the role change of the Replication workload.
func (r *ReplicationSet) HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	podList, err := getRunningPods(ctx, r.Cli, obj)
	if err != nil {
		return nil, err
	}
	if len(podList) == 0 {
		return nil, nil
	}
	// sync pod role label and annotations
	vertexes, err := r.asyncReplicationPodRoleLabelAndAnnotations(podList)
	if err != nil {
		return nil, err
	}
	// async cluster.status.components.replicationSet.status
	if err := asyncReplicationSetClusterStatus(r.Cluster, r.getWorkloadType(), r.getName(), podList); err != nil {
		return nil, err
	}
	return vertexes, nil
}

// asyncReplicationPodRoleLabelAndAnnotations is used to async the role label and annotations of the Pod of the Replication workload.
func (r *ReplicationSet) asyncReplicationPodRoleLabelAndAnnotations(podList []corev1.Pod) ([]graph.Vertex, error) {
	primary := ""
	vertexes := make([]graph.Vertex, 0)
	var updateRolePodList []corev1.Pod
	for _, pod := range podList {
		// if there is no role label on the Pod, it needs to be patch role label.
		if v, ok := pod.Labels[constant.RoleLabelKey]; !ok || v == "" {
			updateRolePodList = append(updateRolePodList, pod)
		} else if v == constant.Primary {
			primary = pod.Name
		}
	}
	// sync pod role label
	if len(updateRolePodList) > 0 {
		for _, pod := range updateRolePodList {
			// if exists primary Pod, it means that the Pod without a role label is a new secondary Pod created by h-scale.
			if primary != "" {
				pod.GetLabels()[constant.RoleLabelKey] = constant.Secondary
				vertexes = append(vertexes, &ictrltypes.LifecycleVertex{
					Obj:    &pod,
					Action: ictrltypes.ActionUpdatePtr(), // update or patch?
				})
			} else {
				// if not exists primary Pod, it means that the component is newly created, and we take the pod with index=0 as the primary by default.
				_, o := util.ParseParentNameAndOrdinal(pod.Name)
				role := DefaultRole(o)
				pod.GetLabels()[constant.RoleLabelKey] = role
				vertexes = append(vertexes, &ictrltypes.LifecycleVertex{
					Obj:    &pod,
					Action: ictrltypes.ActionUpdatePtr(), // update or patch?
				})
			}
		}
	}
	// sync pods primary annotations
	if primary != "" {
		vertexesPatchAnnotation, err := patchPodsPrimaryAnnotation(podList, primary)
		if err != nil {
			return nil, err
		}
		vertexes = append(vertexes, vertexesPatchAnnotation...)
	}
	return vertexes, nil
}

// HandleSwitchover is the implementation of the type Component interface method, which is used to handle the switchover of the Replication workload.
func (r *ReplicationSet) HandleSwitchover(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	// check if all Pods have role label
	podList, err := getRunningPods(ctx, r.Cli, obj)
	for _, pod := range podList {
		if v, ok := pod.Labels[constant.RoleLabelKey]; !ok || v == "" {
			return nil, nil
		}
	}

	// check if the switchover is needed
	needSwitchover, err := util.NeedDeaWithSwitchover(ctx, r.Cli, r.Cluster, r.Component.GetSynthesizedComponent())
	if err != nil {
		return nil, err
	}
	if needSwitchover {
		// create a job to do switchover and check the result
		if err := util.DoSwitchover(ctx, r.Cli, r.Cluster, r.Component.GetSynthesizedComponent()); err != nil {
			return nil, err
		}
	} else {
		// if the switchover is not needed, it means that the switchover has been completed, and the switchover job can be deleted.
		if err := util.PostOpsSwitchover(ctx, r.Cli, r.Cluster, r.Component.GetSynthesizedComponent()); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// HandleFailover is the implementation of the type Component interface method, which is used to handle the failover of the Replication workload.
func (r *ReplicationSet) HandleFailover(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

// DefaultRole is used to get the default role of the Pod of the Replication workload.
func DefaultRole(i int32) string {
	role := constant.Secondary
	if i == 0 {
		role = constant.Primary
	}
	return role
}

// newReplicationSet is the constructor of the type ReplicationSet.
func newReplicationSet(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *ReplicationSet {
	return &ReplicationSet{
		Stateful: stateful.Stateful{
			ComponentSetBase: types.ComponentSetBase{
				Cli:           cli,
				Cluster:       cluster,
				ComponentSpec: spec,
				ComponentDef:  &def,
				Component:     nil,
			},
		},
	}
}
