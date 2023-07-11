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
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// NewRSAvailableReason is added in a deployment when its newest replica set is made available
// ie. the number of new pods that have passed readiness checks and run for at least minReadySeconds
// is at least the minimum available pods that need to run for the deployment.
const NewRSAvailableReason = "NewReplicaSetAvailable"

type stateless struct {
	componentSetBase
}

var _ componentSet = &stateless{}

func (stateless *stateless) getReplicas() int32 {
	if stateless.SynthesizedComponent != nil {
		return stateless.SynthesizedComponent.Replicas
	}
	return stateless.ComponentSpec.Replicas
}

func (stateless *stateless) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if stateless == nil {
		return false, nil
	}
	return stateless.PodsReady(ctx, obj)
}

func (stateless *stateless) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
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

func (stateless *stateless) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if stateless == nil || pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

func (stateless *stateless) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	return "", nil
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (stateless *stateless) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string,
	originPhaseIsUpRunning bool) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	deployList := &appsv1.DeploymentList{}
	podList, err := getCompRelatedObjectList(ctx, stateless.Cli, *stateless.Cluster, componentName, deployList)
	if err != nil || len(deployList.Items) == 0 {
		return "", nil, err
	}
	statusMessages := appsv1alpha1.ComponentMessageMap{}
	// if the failed pod is not controlled by the new ReplicaSetKind
	checkExistFailedPodOfNewRS := func(pod *corev1.Pod, workload metav1.Object) bool {
		d := workload.(*appsv1.Deployment)
		// if component is up running but pod is not ready, this pod should be failed.
		// for example: full disk cause readiness probe failed and serve is not available.
		// but kubelet only sets the container is not ready and pod is also Running.
		if originPhaseIsUpRunning {
			return !intctrlutil.PodIsReady(pod) && belongToNewReplicaSet(d, pod)
		}
		isFailed, _, message := IsPodFailedAndTimedOut(pod)
		existLatestRevisionFailedPod := isFailed && belongToNewReplicaSet(d, pod)
		if existLatestRevisionFailedPod {
			statusMessages.SetObjectMessage(pod.Kind, pod.Name, message)
		}
		return existLatestRevisionFailedPod
	}
	deploy := &deployList.Items[0]
	return getComponentPhaseWhenPodsNotReady(podList, deploy, stateless.getReplicas(),
		deploy.Status.AvailableReplicas, checkExistFailedPodOfNewRS), statusMessages, nil
}

func (stateless *stateless) HandleRestart(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (stateless *stateless) HandleRoleChange(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func newStateless(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *stateless {
	return &stateless{
		componentSetBase: componentSetBase{
			Cli:                  cli,
			Cluster:              cluster,
			SynthesizedComponent: nil,
			ComponentSpec:        spec,
			ComponentDef:         &def,
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
