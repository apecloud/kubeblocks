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

package stateless

import (
	"context"
	"math"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// NewRSAvailableReason is added in a deployment when its newest replica set is made available
// ie. the number of new pods that have passed readiness checks and run for at least minReadySeconds
// is at least the minimum available pods that need to run for the deployment.
const NewRSAvailableReason = "NewReplicaSetAvailable"

type Stateless struct {
	types.ComponentSetBase
}

var _ types.ComponentSet = &Stateless{}

func (stateless *Stateless) getReplicas() int32 {
	if stateless.Component != nil {
		return stateless.Component.GetReplicas()
	}
	return stateless.ComponentSpec.Replicas
}

func (stateless *Stateless) SetComponent(comp types.Component) {
	stateless.Component = comp
}

func (stateless *Stateless) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if stateless == nil {
		return false, nil
	}
	return stateless.PodsReady(ctx, obj)
}

func (stateless *Stateless) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if stateless == nil {
		return false, nil
	}
	deploy, ok := obj.(*appsv1.Deployment)
	if !ok {
		return false, nil
	}
	targetReplicas := stateless.getReplicas()
	return deploymentIsReady(deploy, &targetReplicas), nil
}

func (stateless *Stateless) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if stateless == nil || pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

func (stateless *Stateless) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	return "", nil
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (stateless *Stateless) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	deployList := &appsv1.DeploymentList{}
	podList, err := util.GetCompRelatedObjectList(ctx, stateless.Cli, *stateless.Cluster, componentName, deployList)
	if err != nil || len(deployList.Items) == 0 {
		return "", nil, err
	}
	// if the failed pod is not controlled by the new ReplicaSetKind
	checkExistFailedPodOfNewRS := func(pod *corev1.Pod, workload metav1.Object) bool {
		d := workload.(*appsv1.Deployment)
		return !intctrlutil.PodIsReady(pod) && belongToNewReplicaSet(d, pod)
	}
	deploy := &deployList.Items[0]
	return util.GetComponentPhaseWhenPodsNotReady(podList, deploy, stateless.getReplicas(),
		deploy.Status.AvailableReplicas, checkExistFailedPodOfNewRS), nil, nil
}

func (stateless *Stateless) HandleRestart(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (stateless *Stateless) HandleRoleChange(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (stateless *Stateless) HandleHA(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func newStateless(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *Stateless {
	return &Stateless{
		ComponentSetBase: types.ComponentSetBase{
			Cli:           cli,
			Cluster:       cluster,
			ComponentSpec: spec,
			ComponentDef:  &def,
			Component:     nil,
		},
	}
}

// deploymentIsReady checks deployment is ready
func deploymentIsReady(deploy *appsv1.Deployment, targetReplicas *int32) bool {
	var (
		componentIsRunning = true
		newRSAvailable     = true
	)
	if targetReplicas == nil {
		targetReplicas = deploy.Spec.Replicas
	}

	if hasProgressDeadline(deploy) {
		// if the deployment.Spec.ProgressDeadlineSeconds exists, we should check if the new replicaSet is available.
		// when deployment.Spec.ProgressDeadlineSeconds does not exist, the deployment controller will remove the
		// DeploymentProgressing condition.
		condition := deploymentutil.GetDeploymentCondition(deploy.Status, appsv1.DeploymentProgressing)
		if condition == nil || condition.Reason != NewRSAvailableReason || condition.Status != corev1.ConditionTrue {
			newRSAvailable = false
		}
	}
	// check if the deployment of component is updated completely and ready.
	if deploy.Status.AvailableReplicas != *targetReplicas ||
		deploy.Status.Replicas != *targetReplicas ||
		deploy.Status.ObservedGeneration != deploy.Generation ||
		deploy.Status.UpdatedReplicas != *targetReplicas ||
		!newRSAvailable {
		componentIsRunning = false
	}
	return componentIsRunning
}

// hasProgressDeadline checks if the Deployment d is expected to suffice the reason
// "ProgressDeadlineExceeded" when the Deployment progress takes longer than expected time.
func hasProgressDeadline(d *appsv1.Deployment) bool {
	return d.Spec.ProgressDeadlineSeconds != nil &&
		*d.Spec.ProgressDeadlineSeconds > 0 &&
		*d.Spec.ProgressDeadlineSeconds != math.MaxInt32
}

// belongToNewReplicaSet checks if the pod belongs to the new replicaSet of deployment
func belongToNewReplicaSet(d *appsv1.Deployment, pod *corev1.Pod) bool {
	if pod == nil || d == nil {
		return false
	}
	condition := deploymentutil.GetDeploymentCondition(d.Status, appsv1.DeploymentProgressing)
	if condition == nil {
		return false
	}
	for _, v := range pod.OwnerReferences {
		if v.Kind == constant.ReplicaSetKind && strings.Contains(condition.Message, v.Name) {
			return d.Status.ObservedGeneration == d.Generation
		}
	}
	return false
}
