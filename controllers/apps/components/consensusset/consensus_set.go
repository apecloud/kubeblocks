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

	"github.com/google/go-cmp/cmp"
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
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(consensusSet.Ctx, consensusSet.Cli, sts)
	if err != nil {
		return false, err
	}
	targetReplicas := util.GetComponentReplicas(consensusSet.Component, consensusSet.ComponentDef)

	return util.StatefulSetIsReady(sts, isRevisionConsistent, &targetReplicas), nil
}

func (consensusSet *ConsensusSet) PodsReady(obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
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

func (consensusSet *ConsensusSet) HandleUpdate(obj client.Object) error {
	var (
		cluster = consensusSet.Cluster
		ctx     = consensusSet.Ctx
		cli     = consensusSet.Cli
	)
	stsObj := util.ConvertToStatefulSet(obj)
	// get compDefName from stsObj.name
	compDefName := cluster.GetComponentDefRefName(stsObj.Labels[intctrlutil.AppComponentLabelKey])

	// get component from ClusterDefinition by compDefName
	component, err := util.GetComponentDefByCluster(ctx, cli, cluster, compDefName)
	if err != nil {
		return err
	}

	if component == nil || component.WorkloadType != appsv1alpha1.Consensus {
		return nil
	}
	pods, err := util.GetPodListByStatefulSet(ctx, cli, stsObj)
	if err != nil {
		return err
	}

	// update cluster.status.component.consensusSetStatus based on all pods currently exist
	componentName := stsObj.Labels[intctrlutil.AppComponentLabelKey]

	// first, get the old status
	var oldConsensusSetStatus *appsv1alpha1.ConsensusSetStatus
	if cluster.Status.Components != nil {
		if v, ok := cluster.Status.Components[componentName]; ok {
			oldConsensusSetStatus = v.ConsensusSetStatus
		}
	}
	// create the initial status
	newConsensusSetStatus := &appsv1alpha1.ConsensusSetStatus{
		Leader: appsv1alpha1.ConsensusMemberStatus{
			Name:       "",
			Pod:        util.ComponentStatusDefaultPodName,
			AccessMode: appsv1alpha1.None,
		},
	}
	// then, calculate the new status
	setConsensusSetStatusRoles(newConsensusSetStatus, *component, pods)
	// if status changed, do update
	if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
		patch := client.MergeFrom(cluster.DeepCopy())
		util.InitClusterComponentStatusIfNeed(cluster, componentName, component)
		componentStatus := cluster.Status.Components[componentName]
		componentStatus.ConsensusSetStatus = newConsensusSetStatus
		cluster.Status.Components[componentName] = componentStatus
		if err = cli.Status().Patch(ctx, cluster, patch); err != nil {
			return err
		}
		// add consensus role info to pod env
		if err := updateConsensusRoleInfo(ctx, cli, cluster, *component, componentName, pods); err != nil {
			return err
		}
	}

	// prepare to do pods Deletion, that's the only thing we should do.
	// the stateful set reconciler will do the others.
	// to simplify the process, wo do pods Delete after stateful set reconcile done,
	// that is stsObj.Generation == stsObj.Status.ObservedGeneration
	if stsObj.Generation != stsObj.Status.ObservedGeneration {
		return nil
	}

	// then we wait all pods' presence, that is len(pods) == stsObj.Spec.Replicas
	// only then, we have enough info about the previous pods before delete the current one
	if len(pods) != int(*stsObj.Spec.Replicas) {
		return nil
	}

	// we don't check whether pod role label present: prefer stateful set's Update done than role probing ready

	// generate the pods Deletion plan
	plan := generateConsensusUpdatePlan(ctx, cli, stsObj, pods, *component)
	// execute plan
	if _, err := plan.WalkOneStep(); err != nil {
		return err
	}
	return nil
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
