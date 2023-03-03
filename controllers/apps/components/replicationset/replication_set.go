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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ReplicationSet is a component object used by Cluster, ClusterComponentDefinition and ClusterComponentSpec
type ReplicationSet struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *appsv1alpha1.Cluster
	ComponentDef *appsv1alpha1.ClusterComponentDefinition
	Component    *appsv1alpha1.ClusterComponentSpec
}

var _ types.Component = &ReplicationSet{}

// IsRunning is the implementation of the type Component interface method,
// which is used to check whether the replicationSet component is running normally.
func (rs *ReplicationSet) IsRunning(obj client.Object) (bool, error) {
	var componentStsList = &appsv1.StatefulSetList{}
	var componentStatusIsRunning = true
	sts := util.ConvertToStatefulSet(obj)
	if err := util.GetObjectListByComponentName(rs.Ctx, rs.Cli, rs.Cluster, componentStsList, sts.Labels[constant.KBAppComponentLabelKey]); err != nil {
		return false, err
	}
	var availableReplicas int32
	for _, stsObj := range componentStsList.Items {
		isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(rs.Ctx, rs.Cli, sts)
		if err != nil {
			return false, err
		}
		stsIsReady := util.StatefulSetOfComponentIsReady(&stsObj, isRevisionConsistent, nil)
		availableReplicas += stsObj.Status.AvailableReplicas
		if !stsIsReady {
			return false, nil
		}
	}
	if availableReplicas != rs.Component.Replicas {
		componentStatusIsRunning = false
	}
	return componentStatusIsRunning, nil
}

// PodsReady is the implementation of the type Component interface method,
// which is used to check whether all the pods of replicationSet component is ready.
func (rs *ReplicationSet) PodsReady(obj client.Object) (bool, error) {
	var podsReady = true
	var componentStsList = &appsv1.StatefulSetList{}
	sts := util.ConvertToStatefulSet(obj)
	if err := util.GetObjectListByComponentName(rs.Ctx, rs.Cli, rs.Cluster, componentStsList, sts.Labels[constant.KBAppComponentLabelKey]); err != nil {
		return false, err
	}
	var availableReplicas int32
	for _, stsObj := range componentStsList.Items {
		availableReplicas += stsObj.Status.AvailableReplicas
		if !util.StatefulSetPodsAreReady(&stsObj, *sts.Spec.Replicas) {
			podsReady = false
		}

	}
	if availableReplicas != rs.Component.Replicas {
		podsReady = false
	}
	return podsReady, nil
}

// PodIsAvailable is the implementation of the type Component interface method,
// Check whether the status of a Pod of the replicationSet is ready, including the role label on the Pod
func (rs *ReplicationSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

// HandleProbeTimeoutWhenPodsReady is the implementation of the type Component interface method,
// and replicationSet does not need to do role probe detection, so it returns false directly.
func (rs *ReplicationSet) HandleProbeTimeoutWhenPodsReady(recorder record.EventRecorder) (bool, error) {
	return false, nil
}

// GetPhaseWhenPodsNotReady is the implementation of the type Component interface method,
// when the pods of replicationSet are not ready, calculate the component phase is Failed or Abnormal.
// if return an empty phase, means the pods of component are ready and skips it.
func (rs *ReplicationSet) GetPhaseWhenPodsNotReady(componentName string) (appsv1alpha1.Phase, error) {
	var (
		isFailed         = true
		isAbnormal       bool
		podList          *corev1.PodList
		componentStsList = &appsv1.StatefulSetList{}
		allPodIsReady    = true
		cluster          = rs.Cluster
		compStatus       appsv1alpha1.ClusterComponentStatus
		needPatch        bool
		err              error
		ok               bool
	)
	if err = util.GetObjectListByComponentName(rs.Ctx, rs.Cli, rs.Cluster, componentStsList, componentName); err != nil {
		return "", err
	}
	if podList, err = util.GetComponentPodList(rs.Ctx, rs.Cli, rs.Cluster, componentName); err != nil {
		return "", err
	}
	podCount := len(podList.Items)
	if podCount == 0 || podCount != len(componentStsList.Items) {
		return appsv1alpha1.FailedPhase, nil
	}
	if cluster.Status.Components == nil {
		return "", fmt.Errorf("%s cluster.Status.ComponentDefs is nil", cluster.Name)
	}
	if compStatus, ok = cluster.Status.Components[componentName]; !ok {
		return "", fmt.Errorf("%s cluster.Status.ComponentDefs[%s] is nil", cluster.Name, componentName)
	} else if compStatus.Message == nil {
		compStatus.Message = appsv1alpha1.ComponentMessageMap{}
	}
	for _, v := range podList.Items {
		// if the pod is terminating, ignore the warning event.
		if v.DeletionTimestamp != nil {
			return "", nil
		}
		labelValue := v.Labels[constant.RoleLabelKey]
		if labelValue == "" {
			isAbnormal = true
			compStatus.Message.SetObjectMessage(v.Kind, v.Name, "empty label for pod, please check.")
			needPatch = true
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
			compStatus.Message.SetObjectMessage(v.Kind, v.Name, "statefulSet's AvailableReplicas is not expected, please check.")
			needPatch = true
		}
	}

	// patch abnormal reason to cluster.status.ComponentDefs.
	if needPatch {
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Status.Components[componentName] = compStatus
		if err = rs.Cli.Status().Patch(rs.Ctx, cluster, patch); err != nil {
			return "", err
		}
	}
	// if all pod is ready, ignore the warning event.
	if allPodIsReady {
		return "", nil
	}
	return util.GetComponentPhase(isFailed, isAbnormal), nil
}

// HandleUpdate is the implementation of the type Component interface method, handles replicationSet workload Pod updates.
func (rs *ReplicationSet) HandleUpdate(obj client.Object) error {
	var componentStsList = &appsv1.StatefulSetList{}
	sts := util.ConvertToStatefulSet(obj)
	if err := util.GetObjectListByComponentName(rs.Ctx, rs.Cli, rs.Cluster, componentStsList, sts.Labels[constant.KBAppComponentLabelKey]); err != nil {
		return err
	}
	for _, sts := range componentStsList.Items {
		if sts.Generation != sts.Status.ObservedGeneration {
			continue
		}
		pod, err := GetAndCheckReplicationPodByStatefulSet(rs.Ctx, rs.Cli, &sts)
		if err != nil {
			return err
		}
		// if there is no role label on the Pod, it needs to be updated with statefulSet's role label.
		if _, ok := pod.Labels[constant.RoleLabelKey]; !ok {
			if err := updateObjRoleLabel(rs.Ctx, rs.Cli, *pod, sts.Labels[constant.RoleLabelKey]); err != nil {
				return err
			}
		}
		if err := util.DeleteStsPods(rs.Ctx, rs.Cli, &sts); err != nil {
			return err
		}
	}
	return nil
}

// NewReplicationSet creates a new ReplicationSet object.
func NewReplicationSet(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition) *ReplicationSet {
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
