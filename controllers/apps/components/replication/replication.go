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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReplicationComponent is a component object used by Cluster, ClusterComponentDefinition and ClusterComponentSpec
type ReplicationComponent struct {
	stateful.StatefulComponent
}

var _ types.Component = &ReplicationComponent{}

// IsRunning is the implementation of the type Component interface method,
// which is used to check whether the replicationSet component is running normally.
func (r *ReplicationComponent) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
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
	if sts.Status.AvailableReplicas < r.Component.Replicas {
		componentStatusIsRunning = false
	}
	return componentStatusIsRunning, nil
}

// PodsReady is the implementation of the type Component interface method,
// which is used to check whether all the pods of replicationSet component is ready.
func (r *ReplicationComponent) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	return r.StatefulComponent.PodsReady(ctx, obj)
}

// PodIsAvailable is the implementation of the type Component interface method,
// Check whether the status of a Pod of the replicationSet is ready, including the role label on the Pod
func (r *ReplicationComponent) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

// HandleProbeTimeoutWhenPodsReady is the implementation of the type Component interface method,
// and replicationSet does not need to do role probe detection, so it returns false directly.
func (r *ReplicationComponent) HandleProbeTimeoutWhenPodsReady(ctx context.Context, recorder record.EventRecorder) (bool, error) {
	return false, nil
}

// GetPhaseWhenPodsNotReady is the implementation of the type Component interface method,
// when the pods of replicationSet are not ready, calculate the component phase is Failed or Abnormal.
// if return an empty phase, means the pods of component are ready and skips it.
func (r *ReplicationComponent) GetPhaseWhenPodsNotReady(ctx context.Context,
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
		primaryIsReady               bool
		needPatch                    bool
		compStatus                   = r.Cluster.Status.Components[componentName]
	)
	for _, v := range podList.Items {
		// if the pod is terminating, ignore it
		if v.DeletionTimestamp != nil {
			return "", nil
		}
		labelValue := v.Labels[constant.RoleLabelKey]
		if labelValue == string(Primary) && intctrlutil.PodIsReady(&v) {
			primaryIsReady = true
			continue
		}
		if labelValue == "" {
			compStatus.SetObjectMessage(v.Kind, v.Name, "empty label for pod, please check.")
			needPatch = true
		}
		if !intctrlutil.PodIsReady(&v) && intctrlutil.PodIsControlledByLatestRevision(&v, &stsObj) {
			existLatestRevisionFailedPod = true
		}
	}
	// REVIEW: this isn't a get function, where r.Cluster.Status.Components is being updated.
	// patch abnormal reason to cluster.status.ComponentDefs.
	if needPatch {
		patch := client.MergeFrom(r.Cluster.DeepCopy())
		r.Cluster.Status.SetComponentStatus(componentName, compStatus)
		if err = r.Cli.Status().Patch(ctx, r.Cluster, patch); err != nil {
			return "", err
		}
	}
	return util.GetCompPhaseByConditions(existLatestRevisionFailedPod, primaryIsReady,
		componentReplicas, int32(podCount), stsObj.Status.AvailableReplicas), nil
}

// HandleUpdate is the implementation of the type Component interface method, handles replicationSet workload Pod updates.
func (r *ReplicationComponent) HandleUpdate(ctx context.Context, obj client.Object) error {
	sts := util.ConvertToStatefulSet(obj)
	if sts.Generation != sts.Status.ObservedGeneration {
		return nil
	}
	podList, err := util.GetPodListByStatefulSet(ctx, r.Cli, sts)
	if err != nil {
		return err
	}
	if len(podList) == 0 {
		return nil
	}
	for _, pod := range podList {
		// if there is no role label on the Pod, it needs to be updated with statefulSet's role label.
		if v, ok := pod.Labels[constant.RoleLabelKey]; !ok || v == "" {
			_, o := util.ParseParentNameAndOrdinal(pod.Name)
			role := string(Secondary)
			if o == r.Component.GetPrimaryIndex() {
				role = string(Primary)
			}
			if err := updateObjRoleLabel(ctx, r.Cli, pod, role); err != nil {
				return err
			}
		}
		if err := util.DeleteStsPods(ctx, r.Cli, sts); err != nil {
			return err
		}
	}
	// sync cluster.spec.componentSpecs.[x].primaryIndex when failover occurs and switchPolicy is Noop.
	if err := syncPrimaryIndex(ctx, r.Cli, r.Cluster, sts.Labels[constant.KBAppComponentLabelKey]); err != nil {
		return err
	}
	// sync cluster.status.components.replicationSet.status
	clusterDeepCopy := r.Cluster.DeepCopy()
	if err := syncReplicationSetClusterStatus(r.Cli, ctx, r.Cluster, podList); err != nil {
		return err
	}
	if reflect.DeepEqual(clusterDeepCopy.Status.Components, r.Cluster.Status.Components) {
		return nil
	}
	return r.Cli.Status().Patch(ctx, r.Cluster, client.MergeFrom(clusterDeepCopy))
}

func DefaultRole(i int32) string {
	role := string(Secondary)
	if i == 0 {
		role = string(Primary)
	}
	return role
}

// NewReplicationComponent creates a new ReplicationSet object.
func NewReplicationComponent(
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef appsv1alpha1.ClusterComponentDefinition) (types.Component, error) {
	if err := util.ComponentRuntimeReqArgsCheck(cli, cluster, component); err != nil {
		return nil, err
	}
	return &ReplicationComponent{
		StatefulComponent: stateful.StatefulComponent{
			Cli:          cli,
			Cluster:      cluster,
			Component:    component,
			ComponentDef: &componentDef,
		},
	}, nil
}
