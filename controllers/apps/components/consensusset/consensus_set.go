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

package consensusset

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ConsensusSet struct {
	Cli          client.Client
	Ctx          context.Context
	Cluster      *appsv1alpha1.Cluster
	ComponentDef *appsv1alpha1.ClusterComponentDefinition
	Component    *appsv1alpha1.ClusterComponentSpec
}

var _ types.Component = &ConsensusSet{}

func (consensusSet *ConsensusSet) IsRunning(obj client.Object) (bool, error) {
	// TODO The function name (IsRunning) sounds like it should be side-effect free,
	// TODO however, a lot of changes are done here, including setting cluster status,
	// TODO it may even delete some pod. Should be revised.
	if obj == nil {
		return false, nil
	}
	sts := util.CovertToStatefulSet(obj)
	if statefulStatusRevisionIsEquals, err := handleConsensusSetUpdate(consensusSet.Ctx, consensusSet.Cli, consensusSet.Cluster, sts); err != nil {
		return false, err
	} else {
		targetReplicas := util.GetComponentReplicas(consensusSet.Component, consensusSet.ComponentDef)
		return util.StatefulSetIsReady(sts, statefulStatusRevisionIsEquals, &targetReplicas), nil
	}
}

func (consensusSet *ConsensusSet) PodsReady(obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.CovertToStatefulSet(obj)
	return util.StatefulSetPodsIsReady(sts), nil
}

func (consensusSet *ConsensusSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return util.PodIsReady(*pod)
}

func (consensusSet *ConsensusSet) HandleProbeTimeoutWhenPodsReady(recorder record.EventRecorder) (bool, error) {
	var (
		compStatus    appsv1alpha1.ClusterComponentStatus
		ok            bool
		cluster       = consensusSet.Cluster
		componentName = consensusSet.Component.Name
	)
	if cluster.Status.Components == nil {
		return true, nil
	}
	if compStatus, ok = cluster.Status.Components[componentName]; !ok {
		return true, nil
	}
	if compStatus.PodsReadyTime == nil {
		return true, nil
	}
	if !util.IsProbeTimeout(consensusSet.ComponentDef, compStatus.PodsReadyTime) {
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
	patch := client.MergeFrom(cluster.DeepCopy())
	for _, pod := range podList.Items {
		role := pod.Labels[intctrlutil.RoleLabelKey]
		if role == consensusSet.ComponentDef.ConsensusSpec.Leader.Name {
			isFailed = false
		}
		if role == "" {
			isAbnormal = true
			if compStatus.Message == nil {
				compStatus.Message = appsv1alpha1.ComponentMessageMap{}
			}
			compStatus.Message.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
			needPatch = true
		}
		// TODO clear up the message of ready pod in component.message.
	}
	if !needPatch {
		return true, nil
	}
	if isFailed {
		compStatus.Phase = appsv1alpha1.FailedPhase
	} else if isAbnormal {
		compStatus.Phase = appsv1alpha1.AbnormalPhase
	}
	cluster.Status.Components[componentName] = compStatus
	if err = consensusSet.Cli.Status().Patch(consensusSet.Ctx, cluster, patch); err != nil {
		return false, err
	}
	if recorder != nil {
		recorder.Eventf(cluster, corev1.EventTypeWarning, types.RoleProbeTimeoutReason, "pod role detection timed out in Component: "+consensusSet.Component.Name)
	}
	// when component status changed, mark OpsRequest to reconcile.
	return false, opsutil.MarkRunningOpsRequestAnnotation(consensusSet.Ctx, consensusSet.Cli, cluster)
}

func (consensusSet *ConsensusSet) GetPhaseWhenPodsNotReady(componentName string) (appsv1alpha1.Phase, error) {
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
		return appsv1alpha1.FailedPhase, nil
	}
	for _, v := range podList.Items {
		// if the pod is terminating, ignore the warning event
		if v.DeletionTimestamp != nil {
			return "", nil
		}
		labelValue := v.Labels[intctrlutil.RoleLabelKey]
		if labelValue == consensusSet.ComponentDef.ConsensusSpec.Leader.Name && intctrlutil.PodIsReady(&v) {
			isFailed = false
			continue
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
	componentReplicas := util.GetComponentReplicas(consensusSet.Component, consensusSet.ComponentDef)
	if componentReplicas != int32(podCount) {
		isAbnormal = true
		allPodIsReady = false
	}
	// if all pod is ready, ignore the warning event
	if allPodIsReady {
		return "", nil
	}
	return util.GetComponentPhase(isFailed, isAbnormal), nil
}

func NewConsensusSet(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef *appsv1alpha1.ClusterComponentDefinition) types.Component {
	if component == nil || componentDef == nil {
		return nil
	}
	return &ConsensusSet{
		Ctx:          ctx,
		Cli:          cli,
		Cluster:      cluster,
		ComponentDef: componentDef,
		Component:    component,
	}
}
