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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ReplicationSet struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *dbaasv1alpha1.Cluster
	ComponentDef *dbaasv1alpha1.ClusterDefinitionComponent
	Component    *dbaasv1alpha1.ClusterComponent
}

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

func (rs *ReplicationSet) HandleProbeTimeoutWhenPodsReady() (bool, error) {
	return true, nil
}

func (rs *ReplicationSet) CalculatePhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error) {
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
	return util.CalculateComponentPhase(isFailed, isAbnormal), nil
}

func NewReplicationSet(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	component *dbaasv1alpha1.ClusterComponent,
	componentDef *dbaasv1alpha1.ClusterDefinitionComponent) *ReplicationSet {
	return &ReplicationSet{
		Ctx:          ctx,
		Cli:          cli,
		Cluster:      cluster,
		ComponentDef: componentDef,
		Component:    component,
	}
}
