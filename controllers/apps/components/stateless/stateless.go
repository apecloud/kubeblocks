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

package stateless

import (
	"context"
	"math"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// NewRSAvailableReason is added in a deployment when its newest replica set is made available
// ie. the number of new pods that have passed readiness checks and run for at least minReadySeconds
// is at least the minimum available pods that need to run for the deployment.
const NewRSAvailableReason = "NewReplicaSetAvailable"

type Stateless struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *appsv1alpha1.Cluster
	ComponentDef *appsv1alpha1.ClusterComponentDefinition
	Component    *appsv1alpha1.ClusterComponentSpec
}

var _ types.Component = &Stateless{}

func (stateless *Stateless) IsRunning(obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	return stateless.PodsReady(obj)
}

func (stateless *Stateless) PodsReady(obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	deploy, ok := obj.(*appsv1.Deployment)
	if !ok {
		return false, nil
	}
	return DeploymentIsReady(deploy, &stateless.Component.Replicas), nil
}

func (stateless *Stateless) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

// HandleProbeTimeoutWhenPodsReady the stateless component has no role detection, empty implementation here.
func (stateless *Stateless) HandleProbeTimeoutWhenPodsReady(recorder record.EventRecorder) (bool, error) {
	return false, nil
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (stateless *Stateless) GetPhaseWhenPodsNotReady(componentName string) (appsv1alpha1.Phase, error) {
	deployList := &appsv1.DeploymentList{}
	podList, err := util.GetCompRelatedObjectList(stateless.Ctx, stateless.Cli, stateless.Cluster, componentName, deployList)
	if err != nil || len(deployList.Items) == 0 {
		return "", err
	}
	// if the failed pod is not controlled by the new ReplicaSet
	checkExistFailedPodOfNewRS := func(pod *corev1.Pod, workload metav1.Object) bool {
		d := workload.(*appsv1.Deployment)
		return !intctrlutil.PodIsReady(pod) && belongToNewReplicaSet(d, pod)
	}
	deploy := &deployList.Items[0]
	return util.GetComponentPhaseWhenPodsNotReady(podList, deploy, stateless.Component.Replicas,
		deploy.Status.AvailableReplicas, checkExistFailedPodOfNewRS), nil
}

func (stateless *Stateless) HandleUpdate(obj client.Object) error {
	return nil
}

func NewStateless(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition) types.Component {
	if component == nil || componentDef == nil {
		return nil
	}
	return &Stateless{
		Ctx:          ctx,
		Cli:          cli,
		Cluster:      cluster,
		Component:    component,
		ComponentDef: componentDef,
	}
}

// DeploymentIsReady check deployment is ready
func DeploymentIsReady(deploy *appsv1.Deployment, targetReplicas *int32) bool {
	var (
		componentIsRunning = true
		newRSAvailable     = true
	)
	if targetReplicas == nil {
		targetReplicas = deploy.Spec.Replicas
	}

	if HasProgressDeadline(deploy) {
		// if the deployment.Spec.ProgressDeadlineSeconds exists, we should check if the new replicaSet is available.
		// when deployment.Spec.ProgressDeadlineSeconds does not exist, the deployment controller will remove the DeploymentProgressing condition.
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

// HasProgressDeadline checks if the Deployment d is expected to surface the reason
// "ProgressDeadlineExceeded" when the Deployment progress takes longer than expected time.
func HasProgressDeadline(d *appsv1.Deployment) bool {
	return d.Spec.ProgressDeadlineSeconds != nil && *d.Spec.ProgressDeadlineSeconds != math.MaxInt32
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
		if v.Kind == constant.ReplicaSet && strings.Contains(condition.Message, v.Name) {
			return d.Status.ObservedGeneration == d.Generation
		}
	}
	return false
}
