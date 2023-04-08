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
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ConsensusSet struct {
	Cli          client.Client
	Cluster      *appsv1alpha1.Cluster
	Component    *appsv1alpha1.ClusterComponentSpec
	ComponentDef *appsv1alpha1.ClusterComponentDefinition
}

//var _ types.Component = &ConsensusSet{}

func (c *ConsensusSet) GetName() string {
	return c.Component.Name
}

func (c *ConsensusSet) GetNamespace() string {
	return c.Cluster.GetNamespace()
}

func (c *ConsensusSet) GetMatchingLabels() client.MatchingLabels {
	return util.GetComponentMatchLabels(c.GetNamespace(), c.GetName())
}

func (c *ConsensusSet) GetDefinition() *appsv1alpha1.ClusterComponentDefinition {
	return c.ComponentDef
}

func (r *ConsensusSet) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	pods, err := util.GetPodListByStatefulSet(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			return false, nil
		}
	}

	return util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, &r.Component.Replicas), nil
}

func (r *ConsensusSet) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	return util.StatefulSetPodsAreReady(sts, r.Component.Replicas), nil
}

func (r *ConsensusSet) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return intctrlutil.PodIsReadyWithLabel(*pod)
}

//func (r *ConsensusSet) HandleProbeTimeoutWhenPodsReady(ctx context.Context, recorder record.EventRecorder) (bool, error) {
//	var (
//		compStatus    appsv1alpha1.ClusterComponentStatus
//		ok            bool
//		cluster       = r.Cluster
//		componentName = r.GetName()
//	)
//	if len(cluster.Status.Components) == 0 {
//		return true, nil
//	}
//	if compStatus, ok = cluster.Status.Components[componentName]; !ok {
//		return true, nil
//	}
//	if compStatus.PodsReadyTime == nil {
//		return true, nil
//	}
//	if !util.IsProbeTimeout(r.GetDefinition(), compStatus.PodsReadyTime) {
//		return true, nil
//	}
//
//	podList, err := util.GetComponentPodList(ctx, r.Cli, *cluster, componentName)
//	if err != nil {
//		return true, err
//	}
//	var (
//		isAbnormal bool
//		needPatch  bool
//		isFailed   = true
//	)
//	patch := client.MergeFrom(cluster.DeepCopy())
//	for _, pod := range podList.Items {
//		role := pod.Labels[constant.RoleLabelKey]
//		if role == r.ComponentDef.ConsensusSpec.Leader.Name {
//			isFailed = false
//		}
//		if role == "" {
//			isAbnormal = true
//			compStatus.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
//			needPatch = true
//		}
//		// TODO clear up the message of ready pod in component.message.
//	}
//	if !needPatch {
//		return true, nil
//	}
//	if isFailed {
//		compStatus.Phase = appsv1alpha1.FailedClusterCompPhase
//	} else if isAbnormal {
//		compStatus.Phase = appsv1alpha1.AbnormalClusterCompPhase
//	}
//	cluster.Status.SetComponentStatus(componentName, compStatus)
//	if err = r.Cli.Status().Patch(ctx, cluster, patch); err != nil {
//		return false, err
//	}
//	if recorder != nil {
//		recorder.Eventf(cluster, corev1.EventTypeWarning, types.RoleProbeTimeoutReason, "pod role detection timed out in Component: "+r.Component.Name)
//	}
//	// when component status changed, mark OpsRequest to reconcile.
//	//return false, opsutil.MarkRunningOpsRequestAnnotation(ctx, r.Cli, cluster)
//	return false, nil // TODO(refactor)
//}

func (r *ConsensusSet) HandleProbeTimeoutWhenPodsReady(status *appsv1alpha1.ClusterComponentStatus, pods []*corev1.Pod) {
	var (
		isAbnormal bool
		isFailed   = true
	)
	for _, pod := range pods {
		role := pod.Labels[constant.RoleLabelKey]
		if role == r.ComponentDef.ConsensusSpec.Leader.Name {
			isFailed = false
		}
		if role == "" {
			isAbnormal = true
			status.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
		}
		// TODO clear up the message of ready pod in component.message.
	}
	if isFailed {
		status.Phase = appsv1alpha1.FailedClusterCompPhase
	} else if isAbnormal {
		status.Phase = appsv1alpha1.AbnormalClusterCompPhase
	}
	//if recorder != nil {
	//	recorder.Eventf(r.Cluster, corev1.EventTypeWarning, types.RoleProbeTimeoutReason, "pod role detection timed out in Component: "+r.Component.Name)
	//}
}

func (r *ConsensusSet) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string) (appsv1alpha1.ClusterComponentPhase, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(ctx, r.Cli, *r.Cluster,
		componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", err
	}
	stsObj := stsList.Items[0]
	podCount := len(podList.Items)
	componentReplicas := r.Component.Replicas
	if podCount == 0 || stsObj.Status.AvailableReplicas == 0 {
		return util.GetPhaseWithNoAvailableReplicas(componentReplicas), nil
	}
	// get the statefulSet of component
	var (
		existLatestRevisionFailedPod bool
		leaderIsReady                bool
		consensusSpec                = r.ComponentDef.ConsensusSpec
	)
	for _, v := range podList.Items {
		// if the pod is terminating, ignore it
		if v.DeletionTimestamp != nil {
			return "", nil
		}
		labelValue := v.Labels[constant.RoleLabelKey]
		if consensusSpec != nil && labelValue == consensusSpec.Leader.Name && intctrlutil.PodIsReady(&v) {
			leaderIsReady = true
			continue
		}
		if !intctrlutil.PodIsReady(&v) && intctrlutil.PodIsControlledByLatestRevision(&v, &stsObj) {
			existLatestRevisionFailedPod = true
		}
	}
	return util.GetCompPhaseByConditions(existLatestRevisionFailedPod, leaderIsReady,
		componentReplicas, int32(podCount), stsObj.Status.AvailableReplicas), nil
}

//func (r *ConsensusSet) HandleUpdate(ctx context.Context, obj client.Object) error {
//	if r == nil {
//		return nil
//	}
//
//	stsObj := util.ConvertToStatefulSet(obj)
//	// get compDefName from stsObj.name
//	compDefName := r.Cluster.GetComponentDefRefName(stsObj.Labels[constant.KBAppComponentLabelKey])
//
//	// get component from ClusterDefinition by compDefName
//	component, err := util.GetComponentDefByCluster(ctx, r.Cli, *r.Cluster, compDefName)
//	if err != nil {
//		return err
//	}
//
//	if component == nil || component.WorkloadType != appsv1alpha1.Consensus {
//		return nil
//	}
//	pods, err := util.GetPodListByStatefulSet(ctx, r.Cli, stsObj)
//	if err != nil {
//		return err
//	}
//
//	// update cluster.status.component.consensusSetStatus based on all pods currently exist
//	componentName := stsObj.Labels[constant.KBAppComponentLabelKey]
//
//	// first, get the old status
//	var oldConsensusSetStatus *appsv1alpha1.ConsensusSetStatus
//	if v, ok := r.Cluster.Status.Components[componentName]; ok {
//		oldConsensusSetStatus = v.ConsensusSetStatus
//	}
//	// create the initial status
//	newConsensusSetStatus := &appsv1alpha1.ConsensusSetStatus{
//		Leader: appsv1alpha1.ConsensusMemberStatus{
//			Name:       "",
//			Pod:        util.ComponentStatusDefaultPodName,
//			AccessMode: appsv1alpha1.None,
//		},
//	}
//	// then, calculate the new status
//	setConsensusSetStatusRoles(newConsensusSetStatus, component, pods)
//	// if status changed, do update
//	if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
//		patch := client.MergeFrom((*r.Cluster).DeepCopy())
//		if err = util.InitClusterComponentStatusIfNeed(r.Cluster, componentName, *component); err != nil {
//			return err
//		}
//		componentStatus := r.Cluster.Status.Components[componentName]
//		componentStatus.ConsensusSetStatus = newConsensusSetStatus
//		r.Cluster.Status.SetComponentStatus(componentName, componentStatus)
//		if err = r.Cli.Status().Patch(ctx, r.Cluster, patch); err != nil {
//			return err
//		}
//		// add consensus role info to pod env
//		if err := updateConsensusRoleInfo(ctx, r.Cli, r.Cluster, component, componentName, pods); err != nil {
//			return err
//		}
//	}
//
//	// prepare to do pods Deletion, that's the only thing we should do,
//	// the statefulset reconciler will do the others.
//	// to simplify the process, we do pods Deletion after statefulset reconcile done,
//	// that is stsObj.Generation == stsObj.Status.ObservedGeneration
//	if stsObj.Generation != stsObj.Status.ObservedGeneration {
//		return nil
//	}
//
//	// then we wait all pods' presence, that is len(pods) == stsObj.Spec.Replicas
//	// only then, we have enough info about the previous pods before delete the current one
//	if len(pods) != int(*stsObj.Spec.Replicas) {
//		return nil
//	}
//
//	// we don't check whether pod role label present: prefer stateful set's Update done than role probing ready
//
//	// generate the pods Deletion plan
//	plan := generateConsensusUpdatePlan(ctx, r.Cli, stsObj, pods, *component)
//	// execute plan
//	if _, err := plan.WalkOneStep(); err != nil {
//		return err
//	}
//	return nil
//}

func (r *ConsensusSet) HandleRestart(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	componentDef := r.ComponentDef
	if componentDef == nil || componentDef.WorkloadType != appsv1alpha1.Consensus {
		return nil, nil
	}

	stsObj := util.ConvertToStatefulSet(obj)
	pods, err := util.GetPodListByStatefulSet(ctx, r.Cli, stsObj)
	if err != nil {
		return nil, err
	}

	// prepare to do pods Deletion, that's the only thing we should do,
	// the statefulset reconciler will do the others.
	// to simplify the process, we do pods Deletion after statefulset reconcile done,
	// that is stsObj.Generation == stsObj.Status.ObservedGeneration
	if stsObj.Generation != stsObj.Status.ObservedGeneration {
		return nil, nil
	}

	// then we wait all pods' presence, that is len(pods) == stsObj.Spec.Replicas
	// only then, we have enough info about the previous pods before delete the current one
	if len(pods) != int(*stsObj.Spec.Replicas) {
		return nil, nil
	}

	// we don't check whether pod role label present: prefer stateful set's Update done than role probing ready

	// generate the pods Deletion plan
	podsToDelete := make([]*corev1.Pod, 0)
	plan := generateRestartPodPlan(ctx, r.Cli, stsObj, pods, *componentDef, podsToDelete)
	// execute plan
	if _, err := plan.WalkOneStep(); err != nil {
		return nil, err
	}

	vertexes := make([]graph.Vertex, 0)
	for _, pod := range podsToDelete {
		vertexes = append(vertexes, &ictrltypes.LifecycleVertex{
			Obj:    pod,
			Action: ictrltypes.ActionDeletePtr(),
			Orphan: true,
		})
	}
	return vertexes, nil
}

func (r *ConsensusSet) HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error) {
	componentDef := r.ComponentDef
	if componentDef == nil || componentDef.WorkloadType != appsv1alpha1.Consensus {
		return nil, nil
	}

	stsObj := util.ConvertToStatefulSet(obj)
	pods, err := util.GetPodListByStatefulSet(ctx, r.Cli, stsObj)
	if err != nil {
		return nil, err
	}

	// update cluster.status.component.consensusSetStatus based on all pods currently exist
	componentName := r.GetName()
	vertexes := make([]graph.Vertex, 0)

	// first, get the old status
	var oldConsensusSetStatus *appsv1alpha1.ConsensusSetStatus
	if v, ok := r.Cluster.Status.Components[componentName]; ok {
		oldConsensusSetStatus = v.ConsensusSetStatus
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
	setConsensusSetStatusRoles(newConsensusSetStatus, componentDef, pods)
	// if status changed, do update
	if !cmp.Equal(newConsensusSetStatus, oldConsensusSetStatus) {
		if err = util.InitClusterComponentStatusIfNeed(r.Cluster, componentName, componentDef.WorkloadType); err != nil {
			return nil, err
		}
		componentStatus := r.Cluster.Status.Components[componentName]
		componentStatus.ConsensusSetStatus = newConsensusSetStatus
		r.Cluster.Status.SetComponentStatus(componentName, componentStatus)

		// TODO: does the update order between cluster and env configmap matter?

		// add consensus role info to pod env
		if err := updateConsensusRoleInfo2(ctx, r.Cli, r.Cluster, componentDef, componentName, pods, vertexes); err != nil {
			return nil, err
		}
	}
	return vertexes, nil
}

func NewConsensusSet(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef appsv1alpha1.ClusterComponentDefinition) (*ConsensusSet, error) {
	if err := util.ComponentRuntimeReqArgsCheck(cli, cluster, component); err != nil {
		return nil, err
	}
	consensus := &ConsensusSet{
		Cli:          cli,
		Cluster:      cluster,
		Component:    component,
		ComponentDef: &componentDef,
	}
	return consensus, nil
}
