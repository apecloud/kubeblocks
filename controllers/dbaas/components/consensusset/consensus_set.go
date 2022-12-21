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

package consensusset

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	"github.com/apecloud/kubeblocks/controllers/dbaas/operations"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ConsensusSet struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *dbaasv1alpha1.Cluster
	ComponentDef *dbaasv1alpha1.ClusterDefinitionComponent
	Component    *dbaasv1alpha1.ClusterComponent
}

func (consensusSet *ConsensusSet) IsRunning(obj client.Object) (bool, error) {
	sts := util.CovertToStatefulSet(obj)
	if statefulStatusRevisionIsEquals, err := handleConsensusSetUpdate(consensusSet.Ctx, consensusSet.Cli, consensusSet.Cluster, sts); err != nil {
		return false, err
	} else {
		return util.StatefulSetIsReady(sts, statefulStatusRevisionIsEquals), nil
	}
}

func (consensusSet *ConsensusSet) PodsReady(obj client.Object) (bool, error) {
	sts := util.CovertToStatefulSet(obj)
	return util.StatefulSetPodsIsReady(sts), nil
}

func (consensusSet *ConsensusSet) HandleProbeTimeoutWhenPodsReady() (bool, error) {
	var (
		statusComponent dbaasv1alpha1.ClusterStatusComponent
		ok              bool
		cluster         = consensusSet.Cluster
		componentName   = consensusSet.Component.Name
	)
	if cluster.Status.Components == nil {
		return true, nil
	}
	if statusComponent, ok = cluster.Status.Components[componentName]; !ok {
		return true, nil
	}
	if statusComponent.PodsReadyTime == nil {
		return true, nil
	}
	if !util.IsProbeTimeout(statusComponent.PodsReadyTime) {
		return true, nil
	}

	podList, err := util.GetComponentPodList(consensusSet.Ctx, consensusSet.Cli, cluster, componentName)
	if err != nil {
		return true, err
	}
	var (
		isAbnormal bool
		needPatch  bool
		isFailed   = true
	)

	for _, pod := range podList.Items {
		role := pod.Labels[intctrlutil.ConsensusSetRoleLabelKey]
		if role == consensusSet.ComponentDef.ConsensusSpec.Leader.Name {
			isFailed = false
		}
		if role == "" {
			isAbnormal = true
			messageMap := statusComponent.GetMessage()
			messageMap.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
			statusComponent.SetMessage(messageMap)
			needPatch = true
		}
	}
	if !needPatch {
		return true, nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	if isFailed {
		statusComponent.Phase = dbaasv1alpha1.FailedPhase
	} else if isAbnormal {
		statusComponent.Phase = dbaasv1alpha1.AbnormalPhase
	}
	cluster.Status.Components[componentName] = statusComponent
	if err = consensusSet.Cli.Status().Patch(consensusSet.Ctx, cluster, patch); err != nil {
		return true, err
	}
	// when component status changed, mark OpsRequest to reconcile.
	return false, operations.MarkRunningOpsRequestAnnotation(consensusSet.Ctx, consensusSet.Cli, cluster)
}

func (consensusSet *ConsensusSet) CalculatePhaseWhenPodsNotReady(componentName string) (dbaasv1alpha1.Phase, error) {
	var (
		isFailed      = true
		isAbnormal    bool
		podList       *corev1.PodList
		allPodIsReady = true
		cluster       = consensusSet.Cluster
		err           error
	)

	if podList, err = util.GetComponentPodList(consensusSet.Ctx, consensusSet.Cli, cluster, componentName); err != nil {
		return "", err
	}

	podCount := len(podList.Items)
	if podCount == 0 {
		return dbaasv1alpha1.FailedPhase, nil
	}
	for _, v := range podList.Items {
		// if the pod is terminating, ignore the warning event
		if v.DeletionTimestamp != nil {
			return "", nil
		}
		labelValue := v.Labels[intctrlutil.ConsensusSetRoleLabelKey]
		if labelValue == consensusSet.ComponentDef.ConsensusSpec.Leader.Name {
			isFailed = false
		}
		// if no role label, the pod is not ready
		if labelValue == "" {
			isAbnormal = true
		}
		if !intctrlutil.PodIsReady(&v) {
			allPodIsReady = false
		}
	}
	// check pod count is equals to the component replicas
	if !consensusSet.podCountEqualsComponentReplicas(consensusSet.ComponentDef, podCount) {
		isAbnormal = true
		allPodIsReady = false
	}
	// if all pod is ready, ignore the warning event
	if allPodIsReady {
		return "", nil
	}
	return util.CalculateComponentPhase(isFailed, isAbnormal), nil
}

// podCountEqualsComponentReplicas check the pod count is equal to the component replicas
func (consensusSet *ConsensusSet) podCountEqualsComponentReplicas(componentDef *dbaasv1alpha1.ClusterDefinitionComponent, podCount int) bool {
	component := consensusSet.Component
	replicas := componentDef.DefaultReplicas
	if component.Replicas != nil {
		replicas = *component.Replicas
	}
	return component != nil && replicas == int32(podCount)
}

func NewConsensusSet(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster,
	component *dbaasv1alpha1.ClusterComponent,
	componentDef *dbaasv1alpha1.ClusterDefinitionComponent) *ConsensusSet {
	return &ConsensusSet{
		Ctx:          ctx,
		Cli:          cli,
		Cluster:      cluster,
		ComponentDef: componentDef,
		Component:    component,
	}
}
