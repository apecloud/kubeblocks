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

package consensusset

import (
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UpdateStrategyTransformer struct{}

func (t *UpdateStrategyTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	csSet := transCtx.CSSet
	origCSSet := transCtx.OrigCSSet
	if !model.IsObjectStatusUpdating(origCSSet) {
		return nil
	}

	// read the underlying sts
	stsObj := &apps.StatefulSet{}
	if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(csSet), stsObj); err != nil {
		return err
	}
	// read all pods belong to the sts, hence belong to our consensus set
	pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, stsObj)
	if err != nil {
		return err
	}

	// prepare to do pods Deletion, that's the only thing we should do,
	// the stateful_set reconciler will do the others.
	// to simplify the process, we do pods Deletion after stateful_set reconcile done,
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
	// TODO(free6om): maybe should wait consensus ready for high availability:
	// 1. after some pods updated
	// 2. before switchover
	// 3. after switchover done

	// generate the pods Deletion plan
	plan := newUpdatePlan(*csSet, pods)
	podsToBeUpdated, err := plan.execute()
	if err != nil {
		return err
	}

	// do switchover if leader in pods to be updated
	switch shouldWaitNextLoop, err := doSwitchoverIfNeeded(transCtx, dag, pods, podsToBeUpdated); {
	case err != nil:
		return err
	case shouldWaitNextLoop:
		return nil
	}

	for _, pod := range podsToBeUpdated {
		model.PrepareDelete(dag, pod)
	}

	return nil
}

// return true means action created or in progress, should wait it to the termination state
func doSwitchoverIfNeeded(transCtx *CSSetTransformContext, dag *graph.DAG, pods []corev1.Pod, podsToBeUpdated []*corev1.Pod) (bool, error) {
	if len(podsToBeUpdated) == 0 {
		return false, nil
	}

	csSet := transCtx.CSSet
	if !shouldSwitchover(csSet, podsToBeUpdated) {
		return false, nil
	}

	actionList, err := getActionList(transCtx, jobScenarioUpdate)
	if err != nil {
		return true, err
	}
	if len(actionList) == 0 {
		return true, createSwitchoverAction(dag, csSet, pods)
	}

	// switch status if found:
	// 1. succeed means action executed successfully,
	//    but the consensus cluster may have false positive(apecloud-mysql only?),
	//    we can't wait forever, update is more important.
	//    do the next pod update stage
	// 2. failed means action executed failed,
	//    but this doesn't mean the consensus cluster didn't switchover(again, apecloud-mysql only?)
	//    we can't do anything either in this situation, emit failed event and
	//    do the next pod update state
	// 3. in progress means action still running,
	//    return and wait it reaches termination state.
	action := actionList[0]
	switch {
	case action.Status.Succeeded == 0 && action.Status.Failed == 0:
		// action in progress, wait
		return true, nil
	case action.Status.Failed > 0:
		emitActionFailedEvent(transCtx, jobTypeSwitchover, action.Name)
		fallthrough
	case action.Status.Succeeded > 0:
		// clean up the action
		doActionCleanup(dag, action)
	}
	return false, nil
}

func createSwitchoverAction(dag *graph.DAG, csSet *workloads.StatefulReplicaSet, pods []corev1.Pod) error {
	leader := getLeaderPodName(csSet.Status.MembersStatus)
	targetOrdinal := selectSwitchoverTarget(csSet, pods)
	target := getPodName(csSet.Name, targetOrdinal)
	actionType := jobTypeSwitchover
	ordinal, _ := getPodOrdinal(leader)
	actionName := getActionName(csSet.Name, int(csSet.Generation), ordinal, actionType)
	action := buildAction(csSet, actionName, actionType, jobScenarioUpdate, leader, target)

	// don't do cluster abnormal status analysis, prefer faster update process
	return createAction(dag, csSet, action)
}

func selectSwitchoverTarget(csSet *workloads.StatefulReplicaSet, pods []corev1.Pod) int {
	var podUpdated, podUpdatedWithLabel string
	for _, pod := range pods {
		if intctrlutil.GetPodRevision(&pod) != csSet.Status.UpdateRevision {
			continue
		}
		if len(podUpdated) == 0 {
			podUpdated = pod.Name
		}
		if _, ok := pod.Labels[model.RoleLabelKey]; !ok {
			continue
		}
		if len(podUpdatedWithLabel) == 0 {
			podUpdatedWithLabel = pod.Name
			break
		}
	}
	var finalPod string
	switch {
	case len(podUpdatedWithLabel) > 0:
		finalPod = podUpdatedWithLabel
	case len(podUpdated) > 0:
		finalPod = podUpdated
	default:
		finalPod = pods[0].Name
	}
	ordinal, _ := getPodOrdinal(finalPod)
	return ordinal
}

func shouldSwitchover(csSet *workloads.StatefulReplicaSet, podsToBeUpdated []*corev1.Pod) bool {
	leaderName := getLeaderPodName(csSet.Status.MembersStatus)
	for _, pod := range podsToBeUpdated {
		if pod.Name == leaderName {
			return true
		}
	}
	return false
}

var _ graph.Transformer = &UpdateStrategyTransformer{}
