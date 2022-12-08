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

package stateless

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component/util"
)

type Stateless struct {
	Cli     client.Client
	Ctx     context.Context
	Cluster *dbaasv1alpha1.Cluster
}

func (stateless *Stateless) IsRunning(obj client.Object) (bool, error) {
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
	return DeploymentIsReady(deploy), nil
}

func (stateless *Stateless) HandleProbeTimeoutWhenPodsReady() (bool, error) {
	return false, nil
}

func (stateless *Stateless) CalculatePhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error) {
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
	return util.CalculateComponentPhase(isFailed, isAbnormal), nil
}

func NewStateless(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster) *Stateless {
	return &Stateless{
		Ctx:     ctx,
		Cli:     cli,
		Cluster: cluster,
	}
}

// DeploymentIsReady check deployment is ready
func DeploymentIsReady(deploy *appsv1.Deployment) bool {
	var (
		targetReplicas     = *deploy.Spec.Replicas
		componentIsRunning = true
	)
	if deploy.Status.AvailableReplicas != targetReplicas ||
		deploy.Status.Replicas != targetReplicas ||
		deploy.Status.ObservedGeneration != deploy.GetGeneration() {
		componentIsRunning = false
	}
	return componentIsRunning
}
