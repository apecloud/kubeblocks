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

package rsm

import (
	"fmt"
	"regexp"
	"strconv"

	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// MemberReconfigurationTransformer handles member reconfiguration
type MemberReconfigurationTransformer struct{}

var _ graph.Transformer = &MemberReconfigurationTransformer{}

type actionInfo struct {
	shortActionName string
	ordinal         int
	actionType      string
}

type conditionChecker = func() bool

var actionNameRegex = regexp.MustCompile(`(.*)-([0-9]+)-([0-9]+)-([a-zA-Z\-]+)$`)

func (t *MemberReconfigurationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rsmTransformContext)
	if model.IsObjectDeleting(transCtx.rsm) {
		return nil
	}
	rsm := transCtx.rsm
	graphCli, _ := transCtx.Client.(model.GraphClient)

	if len(rsm.Spec.Roles) == 0 || rsm.Spec.RoleProbe == nil {
		return nil
	}

	// handle cluster initialization
	// set initReplicas at creation
	if rsm.Status.InitReplicas == 0 {
		rsm.Status.InitReplicas = *rsm.Spec.Replicas
		return nil
	}
	// update readyInitReplicas
	if rsm.Status.ReadyInitReplicas < rsm.Status.InitReplicas {
		rsm.Status.ReadyInitReplicas = int32(len(rsm.Status.MembersStatus))
	}
	// return if cluster initialization not done
	if rsm.Status.ReadyInitReplicas != rsm.Status.InitReplicas {
		return nil
	}

	// cluster initialization done, handle dynamic membership reconfiguration

	// rsm is ready
	if IsRSMReady(rsm) {
		return cleanAction(transCtx, dag)
	}

	if !shouldHaveActions(rsm) {
		return nil
	}

	// get the underlying sts
	sts := &apps.StatefulSet{}
	if err := graphCli.Get(transCtx.Context, client.ObjectKeyFromObject(rsm), sts); err != nil {
		return err
	}

	// no enough replicas in scale out, tell sts to create them.
	memberReadyReplicas := int32(len(rsm.Status.MembersStatus))
	if memberReadyReplicas < *rsm.Spec.Replicas &&
		sts.Status.ReadyReplicas < *rsm.Spec.Replicas {
		return nil
	}

	graphCli.Noop(dag, sts)

	// barrier: the underlying sts is ready and has enough replicas
	if sts.Status.ReadyReplicas < *rsm.Spec.Replicas || !isStatefulSetReady(sts) {
		return nil
	}

	// get last action
	actionList, err := getActionList(transCtx, jobScenarioMembership)
	if err != nil {
		return err
	}

	// if no action, create the first one
	if len(actionList) == 0 {
		return createNextAction(transCtx, dag, rsm, nil)
	}

	// got action, there should be only one action
	action := actionList[0]
	switch {
	case action.Status.Succeeded > 0:
		// wait action's result:
		// e.g. action with ordinal 3 and type member-join, wait member 3 until it appears in status.membersStatus
		if !isActionDone(rsm, action) {
			return nil
		}
		// mark it as 'handled'
		deleteAction(transCtx, dag, action)
		return createNextAction(transCtx, dag, rsm, action)
	case action.Status.Failed > 0:
		emitEvent(transCtx, action)
		if !isSwitchoverAction(action) {
			// need manual handling
			return nil
		}
		return createNextAction(transCtx, dag, rsm, action)
	default:
		// action in progress
		return nil
	}
}

func isStatefulSetReady(sts *apps.StatefulSet) bool {
	if sts == nil {
		return false
	}
	if sts.Status.ObservedGeneration == sts.Generation &&
		sts.Status.Replicas == *sts.Spec.Replicas &&
		sts.Status.ReadyReplicas == sts.Status.Replicas {
		return true
	}
	return false
}

func cleanAction(transCtx *rsmTransformContext, dag *graph.DAG) error {
	actionList, err := getActionList(transCtx, jobScenarioMembership)
	if err != nil {
		return err
	}
	if len(actionList) == 0 {
		return nil
	}
	action := actionList[0]
	switch {
	case action.Status.Succeeded > 0:
		deleteAction(transCtx, dag, action)
	case action.Status.Failed > 0:
		emitEvent(transCtx, action)
	}
	return nil
}

func isActionDone(rsm *workloads.ReplicatedStateMachine, action *batchv1.Job) bool {
	ordinal, _ := getActionOrdinal(action.Name)
	podName := getPodName(rsm.Name, ordinal)
	membersStatus := rsm.Status.MembersStatus
	switch action.Labels[jobTypeLabel] {
	case jobTypeSwitchover:
		leader := getLeaderPodName(rsm.Status.MembersStatus)
		return podName != leader
	case jobTypeMemberLeaveNotifying:
		return !isMemberReady(podName, membersStatus)
	case jobTypeMemberJoinNotifying:
		return isMemberReady(podName, membersStatus)
	case jobTypeLogSync, jobTypePromote:
		// no info, ignore them
	}
	return true
}

func isSwitchoverAction(action *batchv1.Job) bool {
	return action.Labels[jobTypeLabel] == jobTypeSwitchover
}

func deleteAction(transCtx *rsmTransformContext, dag *graph.DAG, action *batchv1.Job) {
	cli, _ := transCtx.Client.(model.GraphClient)
	doActionCleanup(dag, cli, action)
}

func createNextAction(transCtx *rsmTransformContext, dag *graph.DAG, rsm *workloads.ReplicatedStateMachine, currentAction *batchv1.Job) error {
	actionInfoList := generateActionInfoList(rsm)

	if len(actionInfoList) == 0 {
		return nil
	}

	nextActionInfo := actionInfoList[0]
	leader := getLeaderPodName(rsm.Status.MembersStatus)
	ordinal := nextActionInfo.ordinal
	if nextActionInfo.actionType == jobTypeSwitchover {
		ordinal = 0
	}
	target := getPodName(rsm.Name, ordinal)
	actionName := getActionName(rsm.Name, int(rsm.Generation), nextActionInfo.ordinal, nextActionInfo.actionType)
	nextAction := buildAction(rsm, actionName, nextActionInfo.actionType, jobScenarioMembership, leader, target)

	if err := abnormalAnalysis(rsm, nextAction); err != nil {
		emitAbnormalEvent(transCtx, nextActionInfo.actionType, actionName, err)
		return err
	}

	cli, _ := transCtx.Client.(model.GraphClient)
	return createAction(dag, cli, rsm, nextAction)
}

func generateActionInfoList(rsm *workloads.ReplicatedStateMachine) []*actionInfo {
	var actionInfoList []*actionInfo
	memberReadyReplicas := int32(len(rsm.Status.MembersStatus))

	switch {
	case memberReadyReplicas < *rsm.Spec.Replicas:
		// member join
		// members with ordinal less than 'spec.replicas' should in the active cluster
		actionTypeList := []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
		for i := memberReadyReplicas; i < *rsm.Spec.Replicas; i++ {
			actionInfos := generateActionInfos(rsm, int(i), actionTypeList)
			actionInfoList = append(actionInfoList, actionInfos...)
		}
	case memberReadyReplicas > *rsm.Spec.Replicas:
		// member leave
		// members with ordinal greater than 'spec.replicas - 1' should not in the active cluster
		actionTypeList := []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
		for i := memberReadyReplicas - 1; i >= *rsm.Spec.Replicas; i-- {
			actionInfos := generateActionInfos(rsm, int(i), actionTypeList)
			actionInfoList = append(actionInfoList, actionInfos...)
		}
	}

	return actionInfoList
}

func isPreAction(actionType string) bool {
	return actionType == jobTypeSwitchover || actionType == jobTypeMemberLeaveNotifying
}

func shouldHaveActions(rsm *workloads.ReplicatedStateMachine) bool {
	currentReplicas := len(rsm.Status.MembersStatus)
	expectedReplicas := int(*rsm.Spec.Replicas)

	var actionTypeList []string
	switch {
	case currentReplicas > expectedReplicas:
		actionTypeList = []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
	case currentReplicas < expectedReplicas:
		actionTypeList = []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
	}
	for _, actionType := range actionTypeList {
		if shouldCreateAction(rsm, actionType, nil) {
			return true
		}
	}
	return false
}

func shouldCreateAction(rsm *workloads.ReplicatedStateMachine, actionType string, checker conditionChecker) bool {
	if checker != nil && !checker() {
		return false
	}
	reconfiguration := rsm.Spec.MembershipReconfiguration
	if reconfiguration == nil {
		return false
	}
	switch actionType {
	case jobTypeSwitchover:
		return reconfiguration.SwitchoverAction != nil
	case jobTypeMemberJoinNotifying:
		return reconfiguration.MemberJoinAction != nil
	case jobTypeMemberLeaveNotifying:
		return reconfiguration.MemberLeaveAction != nil
	case jobTypeLogSync:
		return reconfiguration.LogSyncAction != nil
	case jobTypePromote:
		return reconfiguration.PromoteAction != nil
	}
	return false
}

func buildShortActionName(parent string, ordinal int, actionType string) string {
	return fmt.Sprintf("%s-%d-%s", parent, ordinal, actionType)
}

func getActionOrdinal(actionName string) (int, error) {
	subMatches := actionNameRegex.FindStringSubmatch(actionName)
	if len(subMatches) < 5 {
		return 0, fmt.Errorf("error actionName: %s", actionName)
	}
	return strconv.Atoi(subMatches[3])
}

// all members with ordinal less than action target pod should be in a good replication state:
// 1. they should be in membersStatus
// 2. they should have a leader
func abnormalAnalysis(rsm *workloads.ReplicatedStateMachine, action *batchv1.Job) error {
	membersStatus := rsm.Status.MembersStatus
	statusMap := make(map[string]workloads.MemberStatus, len(membersStatus))
	for _, status := range membersStatus {
		statusMap[status.PodName] = status
	}
	ordinal, _ := getActionOrdinal(action.Name)
	currentMembers := ordinal
	if isPreAction(action.Labels[jobTypeLabel]) {
		currentMembers = ordinal + 1
	}
	var abnormalPodList, leaderPodList []string
	for i := 0; i < currentMembers; i++ {
		podName := getPodName(rsm.Name, i)
		status, ok := statusMap[podName]
		if !ok {
			abnormalPodList = append(abnormalPodList, podName)
		}
		if status.IsLeader {
			leaderPodList = append(leaderPodList, podName)
		}
	}

	var message string
	if len(abnormalPodList) > 0 {
		message = fmt.Sprintf("abnormal pods: %v", abnormalPodList)
	}
	switch len(leaderPodList) {
	case 0:
		message = fmt.Sprintf("%s, no leader exists", message)
	case 1:
	default:
		message = fmt.Sprintf("%s, too many leaders: %v", message, leaderPodList)
	}
	if len(message) > 0 {
		return fmt.Errorf("cluster unhealthy: %s", message)
	}

	return nil
}

func generateActionInfos(rsm *workloads.ReplicatedStateMachine, ordinal int, actionTypeList []string) []*actionInfo {
	var actionInfos []*actionInfo
	leaderPodName := getLeaderPodName(rsm.Status.MembersStatus)
	podName := getPodName(rsm.Name, ordinal)
	for _, actionType := range actionTypeList {
		checker := func() bool {
			return podName == leaderPodName
		}
		if actionType != jobTypeSwitchover {
			checker = nil
		}
		if !shouldCreateAction(rsm, actionType, checker) {
			continue
		}
		info := &actionInfo{
			shortActionName: buildShortActionName(rsm.Name, ordinal, actionType),
			ordinal:         ordinal,
			actionType:      actionType,
		}
		actionInfos = append(actionInfos, info)
	}
	return actionInfos
}
