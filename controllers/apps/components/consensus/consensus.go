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
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ConsensusComponent struct {
	stateful.StatefulComponent
}

var _ types.Component = &ConsensusComponent{}

func (r *ConsensusComponent) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
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

	return util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, &r.Component.Replicas), nil
}

func (r *ConsensusComponent) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	return r.StatefulComponent.PodsReady(ctx, obj)
}

func (r *ConsensusComponent) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

func (r *ConsensusComponent) HandleProbeTimeoutWhenPodsReady(ctx context.Context, recorder record.EventRecorder) (bool, error) {
	var (
		compStatus    appsv1alpha1.ClusterComponentStatus
		ok            bool
		cluster       = r.Cluster
		componentName = r.Component.Name
	)
	if len(cluster.Status.Components) == 0 {
		return true, nil
	}
	if compStatus, ok = cluster.Status.Components[componentName]; !ok {
		return true, nil
	}
	if compStatus.PodsReadyTime == nil {
		return true, nil
	}
	if !util.IsProbeTimeout(r.ComponentDef, compStatus.PodsReadyTime) {
		return true, nil
	}

	podList, err := util.GetComponentPodList(ctx, r.Cli, *cluster, componentName)
	if err != nil {
		return true, err
	}
	var (
		isAbnormal bool
		needPatch  bool
		isFailed   = true
	)
	patch := client.MergeFrom(cluster.DeepCopy())
	for _, pod := range podList.Items {
		role := pod.Labels[constant.RoleLabelKey]
		if role == r.ComponentDef.ConsensusSpec.Leader.Name {
			isFailed = false
		}
		if role == "" {
			isAbnormal = true
			compStatus.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
			needPatch = true
		}
		// TODO clear up the message of ready pod in component.message.
	}
	if !needPatch {
		return true, nil
	}
	if isFailed {
		compStatus.Phase = appsv1alpha1.FailedClusterCompPhase
	} else if isAbnormal {
		compStatus.Phase = appsv1alpha1.AbnormalClusterCompPhase
	}
	cluster.Status.SetComponentStatus(componentName, compStatus)
	if err = r.Cli.Status().Patch(ctx, cluster, patch); err != nil {
		return false, err
	}
	if recorder != nil {
		recorder.Eventf(cluster, corev1.EventTypeWarning, types.RoleProbeTimeoutReason, "pod role detection timed out in Component: "+r.Component.Name)
	}
	// when component status changed, mark OpsRequest to reconcile.
	return false, opsutil.MarkRunningOpsRequestAnnotation(ctx, r.Cli, cluster)
}

func (r *ConsensusComponent) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string) (appsv1alpha1.ClusterComponentPhase, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(ctx, r.Cli, *r.Cluster,
		componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", err
	}
	stsObj := stsList.Items[0]
	podCount := len(podList.Items)
	componentReplicas := r.Component.Replicas
	if podCount == 0 || stsObj.Status.AvailableReplicas == 0 {
		return util.GetPhaseWithNoAvailableReplicas(componentReplicas), nil
	}
	// get the statefulSet of component
	var (
		existLatestRevisionFailedPod bool
		leaderIsReady                bool
		consensusSpec                = r.ComponentDef.ConsensusSpec
	)
	for _, v := range podList.Items {
		// if the pod is terminating, ignore it
		if v.DeletionTimestamp != nil {
			return "", nil
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
		componentReplicas, int32(podCount), stsObj.Status.AvailableReplicas), nil
}

func (r *ConsensusComponent) HandleUpdate(ctx context.Context, obj client.Object) error {
	if r == nil {
		return nil
	}
	return r.StatefulComponent.HandleUpdateWithProcessors(ctx, obj,
		func(componentDef *appsv1alpha1.ClusterComponentDefinition, pods []corev1.Pod, componentName string) error {
			// first, get the old status
			var oldConsensusSetStatus *appsv1alpha1.ConsensusSetStatus
			if v, ok := r.Cluster.Status.Components[componentName]; ok {
				oldConsensusSetStatus = v.ConsensusSetStatus
			}
			// create the initial status
			newConsensusSetStatus := &appsv1alpha1.ConsensusSetStatus{
				Leader: appsv1alpha1.ConsensusMemberStatus{
					Name:       "",
					Pod:        util.ComponentStatusDefaultPodName,
					AccessMode: appsv1alpha1.None,
				},
			}
			// then, calculate the new status
			setConsensusSetStatusRoles(newConsensusSetStatus, componentDef, pods)
			// if status changed, do update
			if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
				patch := client.MergeFrom((*r.Cluster).DeepCopy())
				if err := util.InitClusterComponentStatusIfNeed(r.Cluster, componentName, *componentDef); err != nil {
					return err
				}
				componentStatus := r.Cluster.Status.Components[componentName]
				componentStatus.ConsensusSetStatus = newConsensusSetStatus
				r.Cluster.Status.SetComponentStatus(componentName, componentStatus)
				if err := r.Cli.Status().Patch(ctx, r.Cluster, patch); err != nil {
					return err
				}
				// add consensus role info to pod env
				if err := updateConsensusRoleInfo(ctx, r.Cli, r.Cluster, componentDef, componentName, pods); err != nil {
					return err
				}
			}
			return nil
		}, ComposeRolePriorityMap, generateConsensusSerialPlan, generateConsensusBestEffortParallelPlan, generateConsensusParallelPlan)
}

func NewConsensusComponent(
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef appsv1alpha1.ClusterComponentDefinition) (types.Component, error) {
	if err := util.ComponentRuntimeReqArgsCheck(cli, cluster, component); err != nil {
		return nil, err
	}
	return &ConsensusComponent{
		StatefulComponent: stateful.StatefulComponent{
			Cli:          cli,
			Cluster:      cluster,
			Component:    component,
			ComponentDef: &componentDef,
		},
	}, nil
}
