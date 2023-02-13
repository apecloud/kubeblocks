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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/types"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
)

// NewRSAvailableReason is added in a deployment when its newest replica set is made available
// ie. the number of new pods that have passed readiness checks and run for at least minReadySeconds
// is at least the minimum available pods that need to run for the deployment.
const NewRSAvailableReason = "NewReplicaSetAvailable"

type Stateless struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *dbaasv1alpha1.Cluster
	ComponentDef *dbaasv1alpha1.ClusterDefinitionComponent
	Component    *dbaasv1alpha1.ClusterComponent
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
	targetReplicas := util.GetComponentReplicas(stateless.Component, stateless.ComponentDef)
	return DeploymentIsReady(deploy, &targetReplicas), nil
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

func (stateless *Stateless) GetPhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error) {
	var (
		isFailed          = true
		isAbnormal        bool
		deployList        = &appsv1.DeploymentList{}
		podsIsTerminating bool
		err               error
	)
	if podsIsTerminating, err = util.CheckRelatedPodIsTerminating(stateless.Ctx,
		stateless.Cli, stateless.Cluster, componentName); err != nil || podsIsTerminating {
		return "", err
	}
	if err = util.GetObjectListByComponentName(stateless.Ctx,
		stateless.Cli, stateless.Cluster, deployList, componentName); err != nil {
		return "", err
	}
	for _, v := range deployList.Items {
		if v.Status.AvailableReplicas < 1 {
			continue
		}
		isFailed = false
		if v.Status.AvailableReplicas < *v.Spec.Replicas {
			isAbnormal = true
		}
	}
	return util.GetComponentPhase(isFailed, isAbnormal), nil
}

func NewStateless(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	component *dbaasv1alpha1.ClusterComponent,
	componentDef *dbaasv1alpha1.ClusterDefinitionComponent) types.Component {
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
