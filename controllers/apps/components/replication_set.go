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

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	emptyReplicationPriority = iota
	secondaryPriority
	primaryPriority
)

// replicationSet is a component object used by Cluster, ClusterComponentDefinition and ClusterComponentSpec
type replicationSet struct {
	stateful
}

var _ componentSet = &replicationSet{}

func (r *replicationSet) getName() string {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Name
	}
	return r.ComponentSpec.Name
}

func (r *replicationSet) getWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Replication
}

func (r *replicationSet) getReplicas() int32 {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Replicas
	}
	return r.ComponentSpec.Replicas
}

// IsRunning is the implementation of the type Component interface method,
// which is used to check whether the replicationSet component is running normally.
func (r *replicationSet) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	var componentStatusIsRunning = true
	sts := convertToStatefulSet(obj)
	isRevisionConsistent, err := isStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	stsIsReady := statefulSetOfComponentIsReady(sts, isRevisionConsistent, nil)
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
func (r *replicationSet) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	return r.stateful.PodsReady(ctx, obj)
}

// PodIsAvailable is the implementation of the type Component interface method,
// Check whether the status of a Pod of the replicationSet is ready, including the role label on the Pod
func (r *replicationSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

func (r *replicationSet) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	return "", nil
}

// GetPhaseWhenPodsNotReady is the implementation of the type Component interface method,
// when the pods of replicationSet are not ready, calculate the component phase is Failed or Abnormal.
// if return an empty phase, means the pods of component are ready and skips it.
func (r *replicationSet) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string,
	originPhaseIsUpRunning bool) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := getCompRelatedObjectList(ctx, r.Cli, *r.Cluster,
		componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", nil, err
	}
	stsObj := stsList.Items[0]
	podCount := len(podList.Items)
	componentReplicas := r.getReplicas()
	if podCount == 0 || stsObj.Status.AvailableReplicas == 0 {
		return getPhaseWithNoAvailableReplicas(componentReplicas), nil, nil
	}
	// get the statefulSet of component
	var (
		existLatestRevisionFailedPod bool
		primaryIsReady               bool
		statusMessages               = appsv1alpha1.ComponentMessageMap{}
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
			statusMessages.SetObjectMessage(v.Kind, v.Name, "empty label for pod, please check.")
		}
		// if component is up running but pod is not ready, this pod should be failed.
		// for example: full disk cause readiness probe failed and serve is not available.
		// but kubelet only sets the container is not ready and pod is also Running.
		if originPhaseIsUpRunning && !intctrlutil.PodIsReady(&v) && intctrlutil.PodIsControlledByLatestRevision(&v, &stsObj) {
			existLatestRevisionFailedPod = true
			continue
		}
		isFailed, _, message := IsPodFailedAndTimedOut(&v)
		if isFailed && intctrlutil.PodIsControlledByLatestRevision(&v, &stsObj) {
			existLatestRevisionFailedPod = true
			statusMessages.SetObjectMessage(v.Kind, v.Name, message)
		}
	}
	return getCompPhaseByConditions(existLatestRevisionFailedPod, primaryIsReady,
		componentReplicas, int32(podCount), stsObj.Status.AvailableReplicas), statusMessages, nil
}

// HandleRestart is the implementation of the type Component interface method, which is used to handle the restart of the Replication workload.
func (r *replicationSet) HandleRestart(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	if r.getWorkloadType() != appsv1alpha1.Replication {
		return nil, nil
	}
	priorityMapperFn := func(component *appsv1alpha1.ClusterComponentDefinition) map[string]int {
		return composeReplicationRolePriorityMap()
	}
	return r.HandleUpdateWithStrategy(ctx, obj, nil, priorityMapperFn, generateReplicationSerialPlan, generateReplicationBestEffortParallelPlan, generateReplicationParallelPlan)
}

// HandleRoleChange is the implementation of the type Component interface method, which is used to handle the role change of the Replication workload.
func (r *replicationSet) HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	podList, err := getRunningPods(ctx, r.Cli, obj)
	if err != nil {
		return nil, err
	}
	if len(podList) == 0 {
		return nil, nil
	}
	primaryPods := make([]string, 0)
	emptyRolePods := make([]string, 0)
	vertexes := make([]graph.Vertex, 0)
	for _, pod := range podList {
		role, ok := pod.Labels[constant.RoleLabelKey]
		if !ok || role == "" {
			emptyRolePods = append(emptyRolePods, pod.Name)
			continue
		}
		if role == constant.Primary {
			primaryPods = append(primaryPods, pod.Name)
		}
	}

	for i := range podList {
		pod := &podList[i]
		needUpdate := false
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		switch {
		case len(emptyRolePods) == len(podList):
			// if the workload is newly created, and the role label is not set, we set the pod with index=0 as the primary by default.
			needUpdate = handlePrimaryNotExistPod(pod)
		default:
			if len(primaryPods) != 1 {
				return nil, errors.New(fmt.Sprintf("the number of primary pod is not equal to 1, primary pods: %v, emptyRole pods: %v", primaryPods, emptyRolePods))
			}
			needUpdate = handlePrimaryExistPod(pod, primaryPods[0])
		}
		if needUpdate {
			vertexes = append(vertexes, &ictrltypes.LifecycleVertex{
				Obj:    pod,
				Action: ictrltypes.ActionPatchPtr(),
			})
		}
	}
	// rebuild cluster.status.components.replicationSet.status
	if err := rebuildReplicationSetClusterStatus(r.Cluster, r.getWorkloadType(), r.getName(), podList); err != nil {
		return nil, err
	}
	return vertexes, nil
}

// handlePrimaryNotExistPod is used to handle the pod which is not exists primary pod.
func handlePrimaryNotExistPod(pod *corev1.Pod) bool {
	parent, o := ParseParentNameAndOrdinal(pod.Name)
	defaultRole := DefaultRole(o)
	pod.GetLabels()[constant.RoleLabelKey] = defaultRole
	pod.Annotations[constant.PrimaryAnnotationKey] = fmt.Sprintf("%s-%d", parent, 0)
	return true
}

// handlePrimaryExistPod is used to handle the pod which is exists primary pod.
func handlePrimaryExistPod(pod *corev1.Pod, primary string) bool {
	needPatch := false
	if pod.Name != primary {
		role, ok := pod.Labels[constant.RoleLabelKey]
		if !ok || role == "" {
			pod.GetLabels()[constant.RoleLabelKey] = constant.Secondary
			needPatch = true
		}
	}
	pk, ok := pod.Annotations[constant.PrimaryAnnotationKey]
	if !ok || pk != primary {
		pod.Annotations[constant.PrimaryAnnotationKey] = primary
		needPatch = true
	}
	return needPatch
}

// DefaultRole is used to get the default role of the Pod of the Replication workload.
func DefaultRole(i int32) string {
	role := constant.Secondary
	if i == 0 {
		role = constant.Primary
	}
	return role
}

// newReplicationSet is the constructor of the type replicationSet.
func newReplicationSet(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *replicationSet {
	return &replicationSet{
		stateful: stateful{
			componentSetBase: componentSetBase{
				Cli:                  cli,
				Cluster:              cluster,
				SynthesizedComponent: nil,
				ComponentSpec:        spec,
				ComponentDef:         &def,
			},
		},
	}
}
