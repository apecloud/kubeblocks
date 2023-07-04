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

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type consensusSet struct {
	stateful
}

var _ componentSet = &consensusSet{}

func (r *consensusSet) getName() string {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Name
	}
	return r.ComponentSpec.Name
}

func (r *consensusSet) getDefName() string {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.ComponentDef
	}
	return r.ComponentDef.Name
}

func (r *consensusSet) getWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Consensus
}

func (r *consensusSet) getReplicas() int32 {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Replicas
	}
	return r.ComponentSpec.Replicas
}

func (r *consensusSet) getConsensusSpec() *appsv1alpha1.ConsensusSetSpec {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.ConsensusSpec
	}
	return r.ComponentDef.ConsensusSpec
}

func (r *consensusSet) getProbes() *appsv1alpha1.ClusterDefinitionProbes {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Probes
	}
	return r.ComponentDef.Probes
}

func (r *consensusSet) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := convertToStatefulSet(obj)
	isRevisionConsistent, err := isStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	pods, err := GetPodListByStatefulSet(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			return false, nil
		}
	}

	targetReplicas := r.getReplicas()
	return statefulSetOfComponentIsReady(sts, isRevisionConsistent, &targetReplicas), nil
}

func (r *consensusSet) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	return r.stateful.PodsReady(ctx, obj)
}

func (r *consensusSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

func (r *consensusSet) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	var (
		isAbnormal     bool
		isFailed       = true
		statusMessages appsv1alpha1.ComponentMessageMap
	)
	compStatus, ok := r.Cluster.Status.Components[r.getName()]
	if !ok || compStatus.PodsReadyTime == nil {
		return "", nil
	}
	if !isProbeTimeout(r.getProbes(), compStatus.PodsReadyTime) {
		return "", nil
	}
	for _, pod := range pods {
		role := pod.Labels[constant.RoleLabelKey]
		if role == r.getConsensusSpec().Leader.Name {
			isFailed = false
		}
		if role == "" {
			isAbnormal = true
			statusMessages.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
		}
		// TODO clear up the message of ready pod in component.message.
	}
	if isFailed {
		return appsv1alpha1.FailedClusterCompPhase, statusMessages
	}
	if isAbnormal {
		return appsv1alpha1.AbnormalClusterCompPhase, statusMessages
	}
	return "", statusMessages
}

func (r *consensusSet) GetPhaseWhenPodsNotReady(ctx context.Context,
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
		leaderIsReady                bool
		consensusSpec                = r.getConsensusSpec()
		statusMessages               = appsv1alpha1.ComponentMessageMap{}
	)
	for _, v := range podList.Items {
		// if the pod is terminating, ignore it
		if v.DeletionTimestamp != nil {
			return "", nil, nil
		}
		labelValue := v.Labels[constant.RoleLabelKey]
		if consensusSpec != nil && labelValue == consensusSpec.Leader.Name && intctrlutil.PodIsReady(&v) {
			leaderIsReady = true
			continue
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
	return getCompPhaseByConditions(existLatestRevisionFailedPod, leaderIsReady,
		componentReplicas, int32(podCount), stsObj.Status.AvailableReplicas), statusMessages, nil
}

func (r *consensusSet) HandleRestart(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	if r.getWorkloadType() != appsv1alpha1.Consensus {
		return nil, nil
	}
	priorityMapperFn := func(component *appsv1alpha1.ClusterComponentDefinition) map[string]int {
		return ComposeRolePriorityMap(component.ConsensusSpec)
	}
	return r.HandleUpdateWithStrategy(ctx, obj, nil, priorityMapperFn, generateConsensusSerialPlan, generateConsensusBestEffortParallelPlan, generateConsensusParallelPlan)
}

// HandleRoleChange is the implementation of the type Component interface method, which is used to handle the role change of the Consensus workload.
func (r *consensusSet) HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	if r.getWorkloadType() != appsv1alpha1.Consensus {
		return nil, nil
	}

	stsObj := convertToStatefulSet(obj)
	pods, err := GetPodListByStatefulSet(ctx, r.Cli, stsObj)
	if err != nil {
		return nil, err
	}

	// update cluster.status.component.consensusSetStatus based on the existences for all pods
	componentName := r.getName()

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
	setConsensusSetStatusRoles(newConsensusSetStatus, r.getConsensusSpec(), pods)
	// if status changed, do update
	if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
		if err = initClusterComponentStatusIfNeed(r.Cluster, componentName, r.getWorkloadType()); err != nil {
			return nil, err
		}
		componentStatus := r.Cluster.Status.Components[componentName]
		componentStatus.ConsensusSetStatus = newConsensusSetStatus
		r.Cluster.Status.SetComponentStatus(componentName, componentStatus)

		// TODO: does the update order between cluster and env configmap matter?

		// add consensus role info to pod env
		return updateConsensusRoleInfo(ctx, r.Cli, r.Cluster, r.getConsensusSpec(), r.getDefName(), componentName, pods)
	}
	return nil, nil
}

// newConsensusSet is the constructor of the type consensusSet.
func newConsensusSet(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *consensusSet {
	return &consensusSet{
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
