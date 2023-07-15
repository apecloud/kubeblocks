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

package rsm

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type RSM struct {
	internal.ComponentSetBase
}

var _ internal.ComponentSet = &RSM{}

func (r *RSM) getReplicas() int32 {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Replicas
	}
	return r.ComponentSpec.Replicas
}

func (r *RSM) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	rsm, ok := obj.(*workloads.ReplicatedStateMachine)
	if !ok {
		return false, nil
	}
	sts := util.ConvertRSMToSTS(rsm)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	targetReplicas := r.getReplicas()
	return util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, &targetReplicas), nil
}

func (r *RSM) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	rsm, ok := obj.(*workloads.ReplicatedStateMachine)
	if !ok {
		return false, nil
	}
	sts := util.ConvertRSMToSTS(rsm)
	return util.StatefulSetPodsAreReady(sts, r.getReplicas()), nil
}

func (r *RSM) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

func (r *RSM) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	return "", nil
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (r *RSM) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string,
	originPhaseIsUpRunning bool) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	rsmList := &workloads.ReplicatedStateMachineList{}
	podList, err := util.GetCompRelatedObjectList(ctx, r.Cli, *r.Cluster, componentName, rsmList)
	if err != nil || len(rsmList.Items) == 0 {
		return "", nil, err
	}
	statusMessages := appsv1alpha1.ComponentMessageMap{}
	// if the failed pod is not controlled by the latest revision
	podIsControlledByLatestRevision := func(pod *corev1.Pod, rsm *workloads.ReplicatedStateMachine) bool {
		return rsm.Status.ObservedGeneration == rsm.Generation && intctrlutil.GetPodRevision(pod) == rsm.Status.UpdateRevision
	}
	checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
		rsm := workload.(*workloads.ReplicatedStateMachine)
		// if component is up running but pod is not ready, this pod should be failed.
		// for example: full disk cause readiness probe failed and serve is not available.
		// but kubelet only sets the container is not ready and pod is also Running.
		if originPhaseIsUpRunning {
			return !intctrlutil.PodIsReady(pod) && podIsControlledByLatestRevision(pod, rsm)
		}
		isFailed, _, message := internal.IsPodFailedAndTimedOut(pod)
		existLatestRevisionFailedPod := isFailed && podIsControlledByLatestRevision(pod, rsm)
		if existLatestRevisionFailedPod {
			statusMessages.SetObjectMessage(pod.Kind, pod.Name, message)
		}
		return existLatestRevisionFailedPod
	}
	rsmObj := rsmList.Items[0]
	return util.GetComponentPhaseWhenPodsNotReady(podList, &rsmObj, r.getReplicas(),
		rsmObj.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision), statusMessages, nil
}

func (r *RSM) HandleRestart(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (r *RSM) HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	if r.SynthesizedComponent.WorkloadType != appsv1alpha1.Consensus &&
		r.SynthesizedComponent.WorkloadType != appsv1alpha1.Replication {
		return nil, nil
	}

	rsmObj, _ := obj.(*workloads.ReplicatedStateMachine)
	// update cluster.status.component.consensusSetStatus based on the existences for all pods
	componentName := rsmObj.Name

	switch r.SynthesizedComponent.WorkloadType {
	case appsv1alpha1.Consensus:
		// first, get the old status
		var oldConsensusSetStatus *appsv1alpha1.ConsensusSetStatus
		if v, ok := r.Cluster.Status.Components[componentName]; ok {
			oldConsensusSetStatus = v.ConsensusSetStatus
		}
		// create the initial status
		newConsensusSetStatus := &appsv1alpha1.ConsensusSetStatus{
			Leader: appsv1alpha1.ConsensusMemberStatus{
				Name:       "",
				Pod:        constant.ComponentStatusDefaultPodName,
				AccessMode: appsv1alpha1.None,
			},
		}
		// then, set the new status
		setConsensusSetStatusRoles(newConsensusSetStatus, rsmObj)
		// if status changed, do update
		if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
			if err := util.InitClusterComponentStatusIfNeed(r.Cluster, componentName, appsv1alpha1.Consensus); err != nil {
				return nil, err
			}
			componentStatus := r.Cluster.Status.Components[componentName]
			componentStatus.ConsensusSetStatus = newConsensusSetStatus
			r.Cluster.Status.SetComponentStatus(componentName, componentStatus)

			return nil, nil
		}
	case appsv1alpha1.Replication:
		sts := util.ConvertRSMToSTS(rsmObj)
		podList, err := util.GetRunningPods(ctx, r.Cli, sts)
		if err != nil {
			return nil, err
		}
		if len(podList) == 0 {
			return nil, nil
		}
		primary := ""
		vertexes := make([]graph.Vertex, 0)
		for _, pod := range podList {
			role, ok := pod.Labels[constant.RoleLabelKey]
			if !ok || role == "" {
				continue
			}
			if role == constant.Primary {
				primary = pod.Name
			}
		}

		for _, pod := range podList {
			needUpdate := false
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			switch {
			case primary == "":
				// if not exists primary pod, it means that the component is newly created, and we take the pod with index=0 as the primary by default.
				needUpdate = handlePrimaryNotExistPod(&pod)
			default:
				needUpdate = handlePrimaryExistPod(&pod, primary)
			}
			if needUpdate {
				vertexes = append(vertexes, &ictrltypes.LifecycleVertex{
					Obj:    &pod,
					Action: ictrltypes.ActionUpdatePtr(),
				})
			}
		}
		// rebuild cluster.status.components.replicationSet.status
		if err := rebuildReplicationSetClusterStatus(r.Cluster, appsv1alpha1.Replication, componentName, podList); err != nil {
			return nil, err
		}
		return vertexes, nil
	}

	return nil, nil
}

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
		if err := util.InitClusterComponentStatusIfNeed(cluster, compName, workloadType); err != nil {
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

func setConsensusSetStatusRoles(newConsensusSetStatus *appsv1alpha1.ConsensusSetStatus, rsmObj *workloads.ReplicatedStateMachine) {
	for _, memberStatus := range rsmObj.Status.MembersStatus {
		status := appsv1alpha1.ConsensusMemberStatus{
			Name: memberStatus.Name,
			Pod: memberStatus.PodName,
			AccessMode: appsv1alpha1.AccessMode(memberStatus.AccessMode),
		}
		switch {
		case memberStatus.IsLeader:
			newConsensusSetStatus.Leader = status
		case memberStatus.CanVote:
			newConsensusSetStatus.Followers = append(newConsensusSetStatus.Followers, status)
		default:
			newConsensusSetStatus.Learner = &status
		}
	}
}

// handlePrimaryNotExistPod is used to handle the pod which is not exists primary pod.
func handlePrimaryNotExistPod(pod *corev1.Pod) bool {
	parent, o := util.ParseParentNameAndOrdinal(pod.Name)
	defaultRole := DefaultRole(o)
	pod.GetLabels()[constant.RoleLabelKey] = defaultRole
	pod.Annotations[constant.PrimaryAnnotationKey] = fmt.Sprintf("%s-%d", parent, 0)
	return true
}

// DefaultRole is used to get the default role of the Pod of the Replication workload.
func DefaultRole(i int32) string {
	role := constant.Secondary
	if i == 0 {
		role = constant.Primary
	}
	return role
}

// handlePrimaryExistPod is used to handle the pod which is exists primary pod.
func handlePrimaryExistPod(pod *corev1.Pod, primary string) bool {
	needPatch := false
	if pod.Name != primary {
		role, ok := pod.Labels[constant.RoleLabelKey]
		if !ok || role != constant.Secondary {
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

func newRSM(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *RSM {
	return &RSM{
		ComponentSetBase: internal.ComponentSetBase{
			Cli:                  cli,
			Cluster:              cluster,
			SynthesizedComponent: nil,
			ComponentSpec:        spec,
			ComponentDef:         &def,
		},
	}
}
