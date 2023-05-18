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

package stateful

import (
	"context"
	"errors"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type StatefulComponent types.ComponentBase

var _ types.Component = &StatefulComponent{}

func (r *StatefulComponent) IsRunning(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	isRevisionConsistent, err := util.IsStsAndPodsRevisionConsistent(ctx, r.Cli, sts)
	if err != nil {
		return false, err
	}
	return util.StatefulSetOfComponentIsReady(sts, isRevisionConsistent, &r.Component.Replicas), nil
}

func (r *StatefulComponent) PodsReady(ctx context.Context, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	sts := util.ConvertToStatefulSet(obj)
	return util.StatefulSetPodsAreReady(sts, r.Component.Replicas), nil
}

func (r *StatefulComponent) PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	if pod == nil {
		return false
	}
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Time{Time: time.Now()})
}

// HandleProbeTimeoutWhenPodsReady the Stateful component has no role detection, empty implementation here.
func (r *StatefulComponent) HandleProbeTimeoutWhenPodsReady(ctx context.Context, recorder record.EventRecorder) (bool, error) {
	return false, nil
}

// GetPhaseWhenPodsNotReady gets the component phase when the pods of component are not ready.
func (r *StatefulComponent) GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (appsv1alpha1.ClusterComponentPhase, error) {
	stsList := &appsv1.StatefulSetList{}
	podList, err := util.GetCompRelatedObjectList(ctx, r.Cli, *r.Cluster, componentName, stsList)
	if err != nil || len(stsList.Items) == 0 {
		return "", err
	}
	// if the failed pod is not controlled by the latest revision
	checkExistFailedPodOfLatestRevision := func(pod *corev1.Pod, workload metav1.Object) bool {
		sts := workload.(*appsv1.StatefulSet)
		return !intctrlutil.PodIsReady(pod) && intctrlutil.PodIsControlledByLatestRevision(pod, sts)
	}
	stsObj := stsList.Items[0]
	return util.GetComponentPhaseWhenPodsNotReady(podList, &stsObj, r.Component.Replicas,
		stsObj.Status.AvailableReplicas, checkExistFailedPodOfLatestRevision), nil
}

// HandleUpdateWithProcessors extended HandleUpdate() with custom processors
// REVIEW/TODO: (nashtsai)
//  1. too many args
func (r *StatefulComponent) HandleUpdateWithProcessors(ctx context.Context, obj client.Object,
	compStatusProcessor func(compDef *appsv1alpha1.ClusterComponentDefinition, pods []corev1.Pod, componentName string) error,
	priorityMapper func(component *appsv1alpha1.ClusterComponentDefinition) map[string]int,
	serialStrategyHandler, bestEffortParallelStrategyHandler, parallelStrategyHandler func(plan *util.Plan, pods []corev1.Pod, rolePriorityMap map[string]int)) error {
	if r == nil {
		return nil
	}

	stsObj := util.ConvertToStatefulSet(obj)
	// get compDefName from stsObj.name
	compDefName := r.Cluster.Spec.GetComponentDefRefName(stsObj.Labels[constant.KBAppComponentLabelKey])

	// get componentDef from ClusterDefinition by compDefName
	componentDef, err := util.GetComponentDefByCluster(ctx, r.Cli, *r.Cluster, compDefName)
	if err != nil {
		return err
	}

	if componentDef == nil || componentDef.IsStatelessWorkload() {
		return nil
	}
	pods, err := util.GetPodListByStatefulSet(ctx, r.Cli, stsObj)
	if err != nil {
		return err
	}

	// update cluster.status.component.consensusSetStatus based on all pods currently exist
	if compStatusProcessor != nil {
		componentName := stsObj.Labels[constant.KBAppComponentLabelKey]
		if err = compStatusProcessor(componentDef, pods, componentName); err != nil {
			return err
		}
	}

	// prepare to do pods Deletion, that's the only thing we should do,
	// the statefulset reconciler will do the others.
	// to simplify the process, we do pods Deletion after statefulset reconcile done,
	// that is stsObj.Generation == stsObj.Status.ObservedGeneration
	if stsObj.Generation != stsObj.Status.ObservedGeneration {
		return nil
	}

	// then we wait all pods' presence, that is len(pods) == stsObj.Spec.Replicas
	// only then, we have enough info about the previous pods before delete the current one
	if len(pods) != int(*stsObj.Spec.Replicas) {
		return nil
	}

	// generate the pods Deletion plan
	plan := generateUpdatePlan(ctx, r.Cli, stsObj, pods, componentDef, priorityMapper,
		serialStrategyHandler, bestEffortParallelStrategyHandler, parallelStrategyHandler)
	// execute plan
	if _, err := plan.WalkOneStep(); err != nil {
		return err
	}
	return nil
}

// generateConsensusUpdatePlan generates Update plan based on UpdateStrategy
func generateUpdatePlan(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet, pods []corev1.Pod,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	priorityMapper func(component *appsv1alpha1.ClusterComponentDefinition) map[string]int,
	serialStrategyHandler, bestEffortParallelStrategyHandler, parallelStrategyHandler func(plan *util.Plan, pods []corev1.Pod, rolePriorityMap map[string]int)) *util.Plan {
	stsWorkload := componentDef.GetStatefulSetWorkload()
	_, s := stsWorkload.FinalStsUpdateStrategy()
	switch s.Type {
	case appsv1.RollingUpdateStatefulSetStrategyType, "":
		return nil
	}

	plan := &util.Plan{}
	plan.Start = &util.Step{}
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
		if err := cli.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}

		return true, nil
	}

	var rolePriorityMap map[string]int
	if priorityMapper != nil {
		rolePriorityMap = priorityMapper(componentDef)
		util.SortPods(pods, rolePriorityMap, constant.RoleLabelKey)
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

func (r *StatefulComponent) HandleUpdate(ctx context.Context, obj client.Object) error {
	if r == nil {
		return nil
	}
	return r.HandleUpdateWithProcessors(ctx, obj, nil, nil, nil, nil, nil)
}

func NewStatefulComponent(
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef appsv1alpha1.ClusterComponentDefinition,
) (types.Component, error) {
	if err := util.ComponentRuntimeReqArgsCheck(cli, cluster, component); err != nil {
		return nil, err
	}
	return &StatefulComponent{
		Cli:          cli,
		Cluster:      cluster,
		Component:    component,
		ComponentDef: &componentDef,
	}, nil
}
