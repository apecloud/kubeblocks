/*
Copyright ApeCloud, Inc.

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

package replicationset

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReplicationSet is a component object used by Cluster, ClusterComponentDefinition and ClusterComponentSpec
type ReplicationSet types.ComponentBase

var _ types.Component = &ReplicationSet{}

// IsRunning is the implementation of the type Component interface method,
// which is used to check whether the replicationSet component is running normally.
func (r *ReplicationSet) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	var componentStsList = &appsv1.StatefulSetList{}
	var componentStatusIsRunning = true
	sts := util.ConvertToStatefulSet(obj)
	if err := util.GetObjectListByComponentName(ctx, r.Cli, *r.Cluster,
		componentStsList, sts.Labels[constant.KBAppComponentLabelKey]); err != nil {
		return false, err
	}
	var availableReplicas int32
	for _, stsObj := range componentStsList.Items {
		isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
		if err != nil {
			return false, err
		}
		stsIsReady := util.StatefulSetOfComponentIsReady(&stsObj, isRevisionConsistent, nil)
		availableReplicas += stsObj.Status.AvailableReplicas
		if !stsIsReady {
			return false, nil
		}
	}
	if availableReplicas < r.Component.Replicas {
		componentStatusIsRunning = false
	}
	return componentStatusIsRunning, nil
}

// PodsReady is the implementation of the type Component interface method,
// which is used to check whether all the pods of replicationSet component is ready.
func (r *ReplicationSet) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	var podsReady = true
	var componentStsList = &appsv1.StatefulSetList{}
	sts := util.ConvertToStatefulSet(obj)
	if err := util.GetObjectListByComponentName(ctx, r.Cli, *r.Cluster, componentStsList,
		sts.Labels[constant.KBAppComponentLabelKey]); err != nil {
		return false, err
	}
	var availableReplicas int32
	for _, stsObj := range componentStsList.Items {
		availableReplicas += stsObj.Status.AvailableReplicas
	}
	if availableReplicas < r.Component.Replicas {
		podsReady = false
	}
	return podsReady, nil
}

// PodIsAvailable is the implementation of the type Component interface method,
// Check whether the status of a Pod of the replicationSet is ready, including the role label on the Pod
func (r *ReplicationSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

// HandleProbeTimeoutWhenPodsReady is the implementation of the type Component interface method,
// and replicationSet does not need to do role probe detection, so it returns false directly.
func (r *ReplicationSet) HandleProbeTimeoutWhenPodsReady(ctx context.Context, recorder record.EventRecorder) (bool, error) {
	return false, nil
}

// GetPhaseWhenPodsNotReady is the implementation of the type Component interface method,
// when the pods of replicationSet are not ready, calculate the component phase is Failed or Abnormal.
// if return an empty phase, means the pods of component are ready and skips it.
func (r *ReplicationSet) GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (appsv1alpha1.ClusterComponentPhase, error) {
	componentStsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(ctx, r.Cli, *r.Cluster, componentName, componentStsList)
	if err != nil || len(componentStsList.Items) == 0 {
		return "", err
	}
	podCount, componentReplicas := len(podList.Items), r.Component.Replicas
	if podCount == 0 {
		return util.GetPhaseWithNoAvailableReplicas(componentReplicas), nil
	}
	var (
		stsMap                       = make(map[string]appsv1.StatefulSet)
		availableReplicas            int32
		primaryIsReady               bool
		existLatestRevisionFailedPod bool
		needPatch                    bool
		compStatus                   = r.Cluster.Status.Components[componentName]
	)
	for _, v := range componentStsList.Items {
		stsMap[v.Name] = v
		availableReplicas += v.Status.AvailableReplicas
	}
	for _, v := range podList.Items {
		// if the pod is terminating, ignore the warning event.
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
		controllerRef := metav1.GetControllerOf(&v)
		stsObj := stsMap[controllerRef.Name]
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
		componentReplicas, int32(podCount), availableReplicas), nil
}

// HandleUpdate is the implementation of the type Component interface method, handles replicationSet workload Pod updates.
func (r *ReplicationSet) HandleUpdate(ctx context.Context, obj client.Object) error {
	var componentStsList = &appsv1.StatefulSetList{}
	var podList []*corev1.Pod
	sts := util.ConvertToStatefulSet(obj)
	if err := util.GetObjectListByComponentName(ctx, r.Cli, *r.Cluster, componentStsList,
		sts.Labels[constant.KBAppComponentLabelKey]); err != nil {
		return err
	}
	for _, sts := range componentStsList.Items {
		if sts.Generation != sts.Status.ObservedGeneration {
			continue
		}
		pod, err := getAndCheckReplicationPodByStatefulSet(ctx, r.Cli, &sts)
		if err != nil {
			return err
		}
		// if there is no role label on the Pod, it needs to be updated with statefulSet's role label.
		if v, ok := pod.Labels[constant.RoleLabelKey]; !ok || v == "" {
			if err := updateObjRoleLabel(ctx, r.Cli, *pod, sts.Labels[constant.RoleLabelKey]); err != nil {
				return err
			}
		} else {
			podList = append(podList, pod)
		}
		if err := util.DeleteStsPods(ctx, r.Cli, &sts); err != nil {
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

// NewReplicationComponent creates a new ReplicationSet object.
func NewReplicationComponent(
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef appsv1alpha1.ClusterComponentDefinition) (types.Component, error) {
	if err := util.ComponentRuntimeReqArgsCheck(cli, cluster, component); err != nil {
		return nil, err
	}
	return &ReplicationSet{
		Cli:          cli,
		Cluster:      cluster,
		Component:    component,
		ComponentDef: &componentDef,
	}, nil
}
