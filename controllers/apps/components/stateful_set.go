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
	"errors"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type stateful struct {
	componentSetBase
}

var _ componentSet = &stateful{}

func (r *stateful) getReplicas() int32 {
	if r.SynthesizedComponent != nil {
		return r.SynthesizedComponent.Replicas
	}
	return r.ComponentSpec.Replicas
}

func (r *stateful) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := convertToStatefulSet(obj)
	isRevisionConsistent, err := isStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	targetReplicas := r.getReplicas()
	return statefulSetOfComponentIsReady(sts, isRevisionConsistent, &targetReplicas), nil
}

func (r *stateful) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := convertToStatefulSet(obj)
	return statefulSetPodsAreReady(sts, r.getReplicas()), nil
}

func (r *stateful) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

func (r *stateful) GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap) {
	return "", nil
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (r *stateful) GetPhaseWhenPodsNotReady(ctx context.Context,
	componentName string,
	originPhaseIsUpRunning bool) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := getCompRelatedObjectList(ctx, r.Cli, *r.Cluster, componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", nil, err
	}
	statusMessages := appsv1alpha1.ComponentMessageMap{}
	// if the failed pod is not controlled by the latest revision
	checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
		sts := workload.(*appsv1.StatefulSet)
		// if component is up running but pod is not ready, this pod should be failed.
		// for example: full disk cause readiness probe failed and serve is not available.
		// but kubelet only sets the container is not ready and pod is also Running.
		if originPhaseIsUpRunning {
			return !intctrlutil.PodIsReady(pod) && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
		}
		isFailed, _, message := IsPodFailedAndTimedOut(pod)
		existLatestRevisionFailedPod := isFailed && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
		if existLatestRevisionFailedPod {
			statusMessages.SetObjectMessage(pod.Kind, pod.Name, message)
		}
		return existLatestRevisionFailedPod
	}
	stsObj := stsList.Items[0]
	return getComponentPhaseWhenPodsNotReady(podList, &stsObj, r.getReplicas(),
		stsObj.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision), statusMessages, nil
}

func (r *stateful) HandleRestart(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

func (r *stateful) HandleRoleChange(context.Context, client.Object) ([]graph.Vertex, error) {
	return nil, nil
}

// HandleUpdateWithStrategy handles the update of component with strategy.
// REVIEW/TODO: (nashtsai)
//  1. too many args
func (r *stateful) HandleUpdateWithStrategy(ctx context.Context, obj client.Object,
	compStatusProcessor func(compDef *appsv1alpha1.ClusterComponentDefinition, pods []corev1.Pod, componentName string) error,
	priorityMapper func(component *appsv1alpha1.ClusterComponentDefinition) map[string]int,
	serialStrategyHandler, bestEffortParallelStrategyHandler, parallelStrategyHandler func(plan *Plan, pods []corev1.Pod, rolePriorityMap map[string]int)) ([]graph.Vertex, error) {
	if r == nil {
		return nil, nil
	}

	stsObj := convertToStatefulSet(obj)
	// get compDefName from stsObj.name
	compDefName := r.Cluster.Spec.GetComponentDefRefName(stsObj.Labels[constant.KBAppComponentLabelKey])

	// get componentDef from ClusterDefinition by compDefName
	componentDef, err := appsv1alpha1.GetComponentDefByCluster(ctx, r.Cli, *r.Cluster, compDefName)
	if err != nil {
		return nil, err
	}

	if componentDef == nil || componentDef.IsStatelessWorkload() {
		return nil, nil
	}
	pods, err := GetPodListByStatefulSet(ctx, r.Cli, stsObj)
	if err != nil {
		return nil, err
	}

	// update cluster.status.component.consensusSetStatus when all pods currently exist
	if compStatusProcessor != nil {
		componentName := stsObj.Labels[constant.KBAppComponentLabelKey]
		if err = compStatusProcessor(componentDef, pods, componentName); err != nil {
			return nil, err
		}
	}

	// prepare to do pods Deletion, that's the only thing we should do,
	// the statefulset reconciler will do the rest.
	// to simplify the process, we do pods Deletion after statefulset reconciliation done,
	// that is stsObj.Generation == stsObj.Status.ObservedGeneration
	if stsObj.Generation != stsObj.Status.ObservedGeneration {
		return nil, nil
	}

	// then we wait for all pods' presence, that is len(pods) == stsObj.Spec.Replicas
	// at that point, we have enough info about the previous pods before delete the current one
	if len(pods) != int(*stsObj.Spec.Replicas) {
		return nil, nil
	}

	// we don't check whether pod role label is present: prefer stateful set's Update done than role probing ready

	// generate the pods Deletion plan
	podsToDelete := make([]*corev1.Pod, 0)
	plan := generateUpdatePlan(stsObj, pods, componentDef, priorityMapper,
		serialStrategyHandler, bestEffortParallelStrategyHandler, parallelStrategyHandler, &podsToDelete)
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

func newStateful(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	spec *appsv1alpha1.ClusterComponentSpec,
	def appsv1alpha1.ClusterComponentDefinition) *stateful {
	return &stateful{
		componentSetBase: componentSetBase{
			Cli:                  cli,
			Cluster:              cluster,
			SynthesizedComponent: nil,
			ComponentSpec:        spec,
			ComponentDef:         &def,
		},
	}
}

// generateConsensusUpdatePlan generates Update plan based on UpdateStrategy
func generateUpdatePlan(stsObj *appsv1.StatefulSet, pods []corev1.Pod,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	priorityMapper func(component *appsv1alpha1.ClusterComponentDefinition) map[string]int,
	serialStrategyHandler, bestEffortParallelStrategyHandler, parallelStrategyHandler func(plan *Plan, pods []corev1.Pod, rolePriorityMap map[string]int),
	podsToDelete *[]*corev1.Pod) *Plan {
	stsWorkload := componentDef.GetStatefulSetWorkload()
	_, s := stsWorkload.FinalStsUpdateStrategy()
	switch s.Type {
	case appsv1.RollingUpdateStatefulSetStrategyType, "":
		return nil
	}

	plan := &Plan{}
	plan.Start = &Step{}
	plan.WalkFunc = func(obj interface{}) (bool, error) {
		pod, ok := obj.(corev1.Pod)
		if !ok {
			return false, errors.New("wrong type: obj not Pod")
		}

		// if DeletionTimestamp is not nil, it is terminating.
		if pod.DeletionTimestamp != nil {
			return true, nil
		}

		// if pod is the latest version, we do nothing
		if intctrlutil.GetPodRevision(&pod) == stsObj.Status.UpdateRevision {
			// wait until ready
			return !intctrlutil.PodIsReadyWithLabel(pod), nil
		}

		// delete the pod to trigger associate StatefulSet to re-create it
		*podsToDelete = append(*podsToDelete, &pod)

		return true, nil
	}

	var rolePriorityMap map[string]int
	if priorityMapper != nil {
		rolePriorityMap = priorityMapper(componentDef)
		SortPods(pods, rolePriorityMap, constant.RoleLabelKey)
	}

	// generate plan by UpdateStrategy
	switch stsWorkload.GetUpdateStrategy() {
	case appsv1alpha1.ParallelStrategy:
		if parallelStrategyHandler != nil {
			parallelStrategyHandler(plan, pods, rolePriorityMap)
		}
	case appsv1alpha1.BestEffortParallelStrategy:
		if bestEffortParallelStrategyHandler != nil {
			bestEffortParallelStrategyHandler(plan, pods, rolePriorityMap)
		}
	case appsv1alpha1.SerialStrategy:
		fallthrough
	default:
		if serialStrategyHandler != nil {
			serialStrategyHandler(plan, pods, rolePriorityMap)
		}
	}
	return plan
}
