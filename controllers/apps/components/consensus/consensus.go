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

package consensus

import (
	"context"

	"github.com/google/go-cmp/cmp"
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

type ConsensusSet struct {
	stateful.Stateful
}

var _ types.ComponentSet = &ConsensusSet{}

func (r *ConsensusSet) getName() string {
	if r.Component != nil {
		return r.Component.GetName()
	}
	return r.ComponentSpec.Name
}

func (r *ConsensusSet) getDefName() string {
	if r.Component != nil {
		return r.Component.GetDefinitionName()
	}
	return r.ComponentDef.Name
}

func (r *ConsensusSet) getWorkloadType() appsv1alpha1.WorkloadType {
	if r.Component != nil {
		return r.Component.GetWorkloadType()
	}
	return r.ComponentDef.WorkloadType
}

func (r *ConsensusSet) getReplicas() int32 {
	if r.Component != nil {
		return r.Component.GetReplicas()
	}
	return r.ComponentSpec.Replicas
}

func (r *ConsensusSet) getConsensusSpec() *appsv1alpha1.ConsensusSetSpec {
	if r.Component != nil {
		return r.Component.GetConsensusSpec()
	}
	return r.ComponentDef.ConsensusSpec
}

func (r *ConsensusSet) SetComponent(comp types.Component) {
	r.Component = comp
}

func (r *ConsensusSet) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	pods, err := util.GetPodListByStatefulSet(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			return false, nil
		}
	}

	targetReplicas := r.getReplicas()
	return util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, &targetReplicas), nil
}

func (r *ConsensusSet) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	return r.Stateful.PodsReady(ctx, obj)
}

func (r *ConsensusSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

func (r *ConsensusSet) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	var (
		isAbnormal     bool
		isFailed       = true
		statusMessages appsv1alpha1.ComponentMessageMap
	)
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

func (r *ConsensusSet) GetPhaseWhenPodsNotReady(ctx context.Context,
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
		leaderIsReady                bool
		consensusSpec                = r.getConsensusSpec()
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
		if !intctrlutil.PodIsReady(&v) && intctrlutil.PodIsControlledByLatestRevision(&v, &stsObj) {
			existLatestRevisionFailedPod = true
		}
	}
	return util.GetCompPhaseByConditions(existLatestRevisionFailedPod, leaderIsReady,
		componentReplicas, int32(podCount), stsObj.Status.AvailableReplicas), nil, nil
}

func (r *ConsensusSet) HandleRestart(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	if r.getWorkloadType() != appsv1alpha1.Consensus {
		return nil, nil
	}

	stsObj := util.ConvertToStatefulSet(obj)
	pods, err := util.GetPodListByStatefulSet(ctx, r.Cli, stsObj)
	if err != nil {
		return nil, err
	}

	// prepare to do pods Deletion, that's the only thing we should do,
	// the statefulset reconciler will do the others.
	// to simplify the process, we do pods Deletion after statefulset reconcile done,
	// that is stsObj.Generation == stsObj.Status.ObservedGeneration
	if stsObj.Generation != stsObj.Status.ObservedGeneration {
		return nil, nil
	}

	// then we wait all pods' presence, that is len(pods) == stsObj.Spec.Replicas
	// only then, we have enough info about the previous pods before delete the current one
	if len(pods) != int(*stsObj.Spec.Replicas) {
		return nil, nil
	}

	// we don't check whether pod role label present: prefer stateful set's Update done than role probing ready

	// generate the pods Deletion plan
	podsToDelete := make([]*corev1.Pod, 0)
	plan := generateRestartPodPlan(ctx, r.Cli, stsObj, pods, r.getConsensusSpec(), &podsToDelete)
	// execute plan
	if _, err := plan.WalkOneStep(); err != nil {
		return nil, err
	}

	vertexes := make([]graph.Vertex, 0)
	for _, pod := range podsToDelete {
		vertexes = append(vertexes, &ictrltypes.LifecycleVertex{
			Obj:    pod,
			Action: ictrltypes.ActionDeletePtr(),
			Orphan: true,
		})
	}
	return vertexes, nil
}

func (r *ConsensusSet) HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	if r.getWorkloadType() != appsv1alpha1.Consensus {
		return nil, nil
	}

	stsObj := util.ConvertToStatefulSet(obj)
	pods, err := util.GetPodListByStatefulSet(ctx, r.Cli, stsObj)
	if err != nil {
		return nil, err
	}

	// update cluster.status.component.consensusSetStatus based on all pods currently exist
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
	// then, calculate the new status
	setConsensusSetStatusRoles(newConsensusSetStatus, r.getConsensusSpec(), pods)
	// if status changed, do update
	if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
		if err = util.InitClusterComponentStatusIfNeed(r.Cluster, componentName, r.getWorkloadType()); err != nil {
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

func (r *ConsensusSet) HandleHA(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

// func (r *ConsensusSet) HandleUpdate(ctx context.Context, obj client.Object) error {
//	if r == nil {
//		return nil
//	}
//	return r.Stateful.HandleUpdateWithProcessors(ctx, obj,
//		func(componentDef *appsv1alpha1.ClusterComponentDefinition, pods []corev1.Pod, componentName string) error {
//			// first, get the old status
//			var oldConsensusSetStatus *appsv1alpha1.ConsensusSetStatus
//			if v, ok := r.Cluster.Status.Components[componentName]; ok {
//				oldConsensusSetStatus = v.ConsensusSetStatus
//			}
//			// create the initial status
//			newConsensusSetStatus := &appsv1alpha1.ConsensusSetStatus{
//				Leader: appsv1alpha1.ConsensusMemberStatus{
//					Name:       "",
//					Pod:        constant.ComponentStatusDefaultPodName,
//					AccessMode: appsv1alpha1.None,
//				},
//			}
//			// then, calculate the new status
//			setConsensusSetStatusRoles(newConsensusSetStatus, r.getConsensusSpec(), pods)
//			// if status changed, do update
//			if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
//				patch := client.MergeFrom((*r.Cluster).DeepCopy())
//				if err := util.InitClusterComponentStatusIfNeed(r.Cluster, componentName, r.getWorkloadType()); err != nil {
//					return err
//				}
//				componentStatus := r.Cluster.Status.Components[componentName]
//				componentStatus.ConsensusSetStatus = newConsensusSetStatus
//				r.Cluster.Status.SetComponentStatus(componentName, componentStatus)
//				if err := r.Cli.Status().Patch(ctx, r.Cluster, patch); err != nil {
//					return err
//				}
//				// add consensus role info to pod env
//				if err := updateConsensusRoleInfo(ctx, r.Cli, r.Cluster, r.getConsensusSpec(), r.getDefName(), componentName, pods, nil); err != nil {
//					return err
//				}
//			}
//			return nil
//		}, ComposeRolePriorityMap, generateConsensusSerialPlan, generateConsensusBestEffortParallelPlan, generateConsensusParallelPlan)
// }

func newConsensusSet(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *ConsensusSet {
	return &ConsensusSet{
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
