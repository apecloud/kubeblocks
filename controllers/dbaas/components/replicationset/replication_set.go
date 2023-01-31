/*
Copyright ApeCloud Inc.

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

	"k8s.io/client-go/tools/record"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/types"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReplicationSet is a component object used by Cluster, ClusterDefinitionComponent and ClusterComponent
type ReplicationSet struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *dbaasv1alpha1.Cluster
	ComponentDef *dbaasv1alpha1.ClusterDefinitionComponent
	Component    *dbaasv1alpha1.ClusterComponent
}

var _ types.Component = &ReplicationSet{}

// IsRunning is the implementation of the type Component interface method,
// which is used to check whether the replicationSet component is running normally.
func (rs *ReplicationSet) IsRunning(obj client.Object) (bool, error) {
	var componentStsList = &appsv1.StatefulSetList{}
	var componentStatusIsRunning = true
	sts := util.CovertToStatefulSet(obj)
	if err := util.GetObjectListByComponentName(rs.Ctx, rs.Cli, rs.Cluster, componentStsList, sts.Labels[intctrlutil.AppComponentLabelKey]); err != nil {
		return false, err
	}
	for _, stsObj := range componentStsList.Items {
		statefulStatusRevisionIsEquals := sts.Status.UpdateRevision == sts.Status.CurrentRevision
		stsIsReady := util.StatefulSetIsReady(&stsObj, statefulStatusRevisionIsEquals)
		if !stsIsReady {
			componentStatusIsRunning = false
		}
	}
	return componentStatusIsRunning, nil
}

// PodsReady is the implementation of the type Component interface method,
// which is used to check whether all the pods of replicationSet component is ready.
func (rs *ReplicationSet) PodsReady(obj client.Object) (bool, error) {
	var podsReady = true
	var componentStsList = &appsv1.StatefulSetList{}
	sts := util.CovertToStatefulSet(obj)
	if err := util.GetObjectListByComponentName(rs.Ctx, rs.Cli, rs.Cluster, componentStsList, sts.Labels[intctrlutil.AppComponentLabelKey]); err != nil {
		return false, err
	}
	for _, stsObj := range componentStsList.Items {
		if !util.StatefulSetPodsIsReady(&stsObj) {
			podsReady = false
		}
	}
	return podsReady, nil
}

// PodIsAvailable is the implementation of the type Component interface method,
// Check whether the status of a Pod of the replicationSet is ready, including the role label on the Pod
func (rs *ReplicationSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return util.PodIsReady(*pod)
}

// HandleProbeTimeoutWhenPodsReady is the implementation of the type Component interface method,
// and replicationSet does not need to do role probe detection, so it returns true directly.
func (rs *ReplicationSet) HandleProbeTimeoutWhenPodsReady(recorder record.EventRecorder) (bool, error) {
	return true, nil
}

// GetPhaseWhenPodsNotReady is the implementation of the type Component interface method,
// when the pods of replicationSet are not ready, calculate the component phase is Failed or Abnormal.
// if return an empty phase, means the pods of component are ready and skips it.
func (rs *ReplicationSet) GetPhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error) {
	var (
		isFailed         = true
		isAbnormal       bool
		podList          *corev1.PodList
		componentStsList = &appsv1.StatefulSetList{}
		allPodIsReady    = true
		err              error
	)
	if err = util.GetObjectListByComponentName(rs.Ctx, rs.Cli, rs.Cluster, componentStsList, componentName); err != nil {
		return "", err
	}
	if podList, err = util.GetComponentPodList(rs.Ctx, rs.Cli, rs.Cluster, componentName); err != nil {
		return "", err
	}
	podCount := len(podList.Items)
	if podCount == 0 || podCount != len(componentStsList.Items) {
		return dbaasv1alpha1.FailedPhase, nil
	}

	for _, v := range podList.Items {
		// if the pod is terminating, ignore the warning event.
		if v.DeletionTimestamp != nil {
			return "", nil
		}
		labelValue := v.Labels[intctrlutil.RoleLabelKey]
		if labelValue == "" {
			isAbnormal = true
		}
		if !intctrlutil.PodIsReady(&v) {
			allPodIsReady = false
		}
	}

	for _, v := range componentStsList.Items {
		if v.Status.AvailableReplicas < 1 {
			continue
		}
		isFailed = false
		if v.Status.AvailableReplicas < *v.Spec.Replicas {
			isAbnormal = true
		}
	}
	// if all pod is ready, ignore the warning event.
	if allPodIsReady {
		return "", nil
	}
	return util.GetComponentPhase(isFailed, isAbnormal), nil
}

// NewReplicationSet creates a new ReplicationSet object.
func NewReplicationSet(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	component *dbaasv1alpha1.ClusterComponent,
	componentDef *dbaasv1alpha1.ClusterDefinitionComponent) *ReplicationSet {
	if component == nil || componentDef == nil {
		return nil
	}
	return &ReplicationSet{
		Ctx:          ctx,
		Cli:          cli,
		Cluster:      cluster,
		ComponentDef: componentDef,
		Component:    component,
	}
}
