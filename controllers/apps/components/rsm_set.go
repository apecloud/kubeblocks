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
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	rsmcore "github.com/apecloud/kubeblocks/internal/controller/rsm"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type RSM struct {
	componentSetBase
}

var _ componentSet = &RSM{}

func (r *RSM) getName() string {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Name
	}
	return r.ComponentSpec.Name
}

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
	sts := ConvertRSMToSTS(rsm)

	// whether sts is ready
	isRevisionConsistent, err := isStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	targetReplicas := r.getReplicas()
	stsReady := statefulSetOfComponentIsReady(sts, isRevisionConsistent, &targetReplicas)
	if !stsReady {
		return stsReady, nil
	}

	// whether rsm is ready
	return rsmcore.IsRSMReady(rsm), nil
}

func (r *RSM) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	rsm, ok := obj.(*workloads.ReplicatedStateMachine)
	if !ok {
		return false, nil
	}
	sts := ConvertRSMToSTS(rsm)
	return statefulSetPodsAreReady(sts, r.getReplicas()), nil
}

func (r *RSM) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	switch {
	case pod == nil:
		return false
	case !podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()}):
		return false
	case r.SynthesizedComponent.WorkloadType == appsv1alpha1.Consensus,
		r.SynthesizedComponent.WorkloadType == appsv1alpha1.Replication:
		return intctrlutil.PodIsReadyWithLabel(*pod)
	default:
		return true
	}
}

func (r *RSM) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	if r.SynthesizedComponent.WorkloadType != appsv1alpha1.Consensus {
		return "", nil
	}

	var (
		isAbnormal     bool
		isFailed       = true
		statusMessages appsv1alpha1.ComponentMessageMap
	)
	getProbes := func() *appsv1alpha1.ClusterDefinitionProbes {
		if r.SynthesizedComponent != nil {
			return r.SynthesizedComponent.Probes
		}
		return r.ComponentDef.Probes
	}
	getConsensusSpec := func() *appsv1alpha1.ConsensusSetSpec {
		if r.SynthesizedComponent != nil {
			return r.SynthesizedComponent.ConsensusSpec
		}
		return r.ComponentDef.ConsensusSpec
	}
	compStatus, ok := r.Cluster.Status.Components[r.getName()]
	if !ok || compStatus.PodsReadyTime == nil {
		return "", nil
	}
	if !isProbeTimeout(getProbes(), compStatus.PodsReadyTime) {
		return "", nil
	}
	for _, pod := range pods {
		role := pod.Labels[constant.RoleLabelKey]
		if role == getConsensusSpec().Leader.Name {
			isFailed = false
		}
		if role == "" {
			isAbnormal = true
			statusMessages.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
		}
		// TODO clear up the message of ready pod in component.message.
	}
	switch {
	case isFailed:
		return appsv1alpha1.FailedClusterCompPhase, statusMessages
	case isAbnormal:
		return appsv1alpha1.AbnormalClusterCompPhase, statusMessages
	default:
		return "", statusMessages
	}
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (r *RSM) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string,
	originPhaseIsUpRunning bool) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	rsmList := &workloads.ReplicatedStateMachineList{}
	podList, err := getCompRelatedObjectList(ctx, r.Cli, *r.Cluster, componentName, rsmList)
	if err != nil || len(rsmList.Items) == 0 {
		return "", nil, err
	}
	statusMessages := appsv1alpha1.ComponentMessageMap{}
	// if the failed pod is not controlled by the latest revision
	podIsControlledByLatestRevision := func(pod *corev1.Pod, rsm *workloads.ReplicatedStateMachine) bool {
		return rsm.Status.ObservedGeneration == rsm.Generation && intctrlutil.GetPodRevision(pod) == rsm.Status.UpdateRevision
	}
	checkLeaderIsReady := func(pod *corev1.Pod, workload metav1.Object) bool {
		getLeaderRoleName := func() string {
			switch r.SynthesizedComponent.WorkloadType {
			case appsv1alpha1.Consensus:
				return r.SynthesizedComponent.ConsensusSpec.Leader.Name
			case appsv1alpha1.Replication:
				return constant.Primary
			default:
				return ""
			}
		}
		leaderRoleName := getLeaderRoleName()
		labelValue := pod.Labels[constant.RoleLabelKey]
		return labelValue == leaderRoleName && intctrlutil.PodIsReady(pod)
	}
	checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
		rsm := workload.(*workloads.ReplicatedStateMachine)
		// if component is up running but pod is not ready, this pod should be failed.
		// for example: full disk cause readiness probe failed and serve is not available.
		// but kubelet only sets the container is not ready and pod is also Running.
		if originPhaseIsUpRunning {
			return !intctrlutil.PodIsReady(pod) && podIsControlledByLatestRevision(pod, rsm)
		}
		isFailed, _, message := IsPodFailedAndTimedOut(pod)
		existLatestRevisionFailedPod := isFailed && podIsControlledByLatestRevision(pod, rsm)
		if existLatestRevisionFailedPod {
			statusMessages.SetObjectMessage(pod.Kind, pod.Name, message)
		}
		return existLatestRevisionFailedPod
	}
	rsmObj := rsmList.Items[0]
	return getComponentPhaseWhenPodsNotReady(podList, &rsmObj, r.getReplicas(),
		rsmObj.Status.AvailableReplicas, checkLeaderIsReady, checkExistFailedPodOfLatestRevision), statusMessages, nil
}

func (r *RSM) HandleRestart(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (r *RSM) HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	if r.SynthesizedComponent.WorkloadType != appsv1alpha1.Consensus &&
		r.SynthesizedComponent.WorkloadType != appsv1alpha1.Replication {
		return nil, nil
	}

	// update cluster.status.component.consensusSetStatus based on the existences for all pods
	componentName := r.getName()
	rsmObj, _ := obj.(*workloads.ReplicatedStateMachine)
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
		setConsensusSetStatusRolesByRSM(newConsensusSetStatus, rsmObj)
		// if status changed, do update
		if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
			if err := initClusterComponentStatusIfNeed(r.Cluster, componentName, appsv1alpha1.Consensus); err != nil {
				return nil, err
			}
			componentStatus := r.Cluster.Status.Components[componentName]
			componentStatus.ConsensusSetStatus = newConsensusSetStatus
			r.Cluster.Status.SetComponentStatus(componentName, componentStatus)

			return nil, nil
		}
	case appsv1alpha1.Replication:
		sts := ConvertRSMToSTS(rsmObj)
		podList, err := getRunningPods(ctx, r.Cli, sts)
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
					return nil, fmt.Errorf("the number of primary pod is not equal to 1, primary pods: %v, emptyRole pods: %v", primaryPods, emptyRolePods)
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
		if err := rebuildReplicationSetClusterStatus(r.Cluster, appsv1alpha1.Replication, componentName, podList); err != nil {
			return nil, err
		}
		return vertexes, nil
	}

	return nil, nil
}

func setConsensusSetStatusRolesByRSM(newConsensusSetStatus *appsv1alpha1.ConsensusSetStatus, rsmObj *workloads.ReplicatedStateMachine) {
	for _, memberStatus := range rsmObj.Status.MembersStatus {
		status := appsv1alpha1.ConsensusMemberStatus{
			Name:       memberStatus.Name,
			Pod:        memberStatus.PodName,
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

func newRSM(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *RSM {
	reqCtx := intctrlutil.RequestCtx{Log: log.FromContext(ctx).WithValues("rsm-test", def.Name)}
	synthesizedComponent, _ := component.BuildComponent(reqCtx, nil, cluster, clusterDef, &def, spec)
	return &RSM{
		componentSetBase: componentSetBase{
			Cli:                  cli,
			Cluster:              cluster,
			SynthesizedComponent: synthesizedComponent,
		},
	}
}
