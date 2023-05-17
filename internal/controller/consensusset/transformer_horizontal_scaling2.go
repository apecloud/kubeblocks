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
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	"github.com/apecloud/kubeblocks/internal/controllerutil"
)

// HorizontalScaling2Transformer Pod level horizontal scaling handling
type HorizontalScaling2Transformer struct{}

type podAction struct {
	podName       string
	hasPreAction  bool
	hasPostAction bool
	actionList    []*batchv1.Job
}

type conditionChecker = func() bool

var actionNameRegex = regexp.MustCompile("(.*)-([0-9]+)-([0-9]+)-([a-zA-Z\\-]+)$")

func (t *HorizontalScaling2Transformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	if model.IsObjectDeleting(transCtx.CSSet) {
		return nil
	}

	// get the underlying sts
	stsVertex, err := getUnderlyingStsVertex(dag)
	if err != nil {
		return err
	}
	// handle Update only, i.e. consensus cluster exists
	if stsVertex.Action == nil || *stsVertex.Action != model.UPDATE {
		return nil
	}

	// handle membership in consensus_set level and pod lifecycle in stateful_set
	// pre-conditions validation: make sure sts level is ok
	sts, _ := stsVertex.OriObj.(*apps.StatefulSet)
	if !isStatefulSetReady(sts) {
		return nil
	}

	// reach the consensus_set expected state
	switch ready, err := isConsensusSetReady(transCtx, transCtx.CSSet); {
	case err != nil:
		return err
	case ready:
		return nil
	}

	// handle membership
	stsVertex.Immutable = true
	updateStsHandler := func() {
		stsVertex.Immutable = false
	}
	pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, sts)
	if err != nil {
		return err
	}
	roleMap := composeRoleMap(*transCtx.CSSet)
	leaderPodName := getLeaderPod(pods, roleMap)

	// TODO(free6om): separate consensus cluster Creation from Update
	if len(leaderPodName) == 0 {
		return nil
	}
	// members with ordinal less than 'spec.replicas' should in the consensus cluster
	// member join
	// create actions
	memberJoining := make([]string, 0)
	actionTypeList := []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
	for i := 0; i < int(transCtx.CSSet.Spec.Replicas); i++ {
		podName := getPodName(transCtx.CSSet.Name, i)
		if isMemberReady(podName, transCtx.CSSet.Status.MembersStatus) {
			continue
		}
		actionCreated, err := doActions(transCtx.CSSet, dag, leaderPodName, i, actionTypeList)
		if err != nil {
			return err
		}
		if actionCreated {
			memberJoining = append(memberJoining, podName)
		}
	}
	// member leave
	// members with ordinal greater than 'spec.replicas - 1' should not in the consensus cluster
	memberLeaving := make([]string, 0)
	actionTypeList = []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
	for i := transCtx.CSSet.Spec.Replicas; i < transCtx.CSSet.Status.Replicas; i++ {
		actionCreated, err := doActions(transCtx.CSSet, dag, leaderPodName, int(i), actionTypeList)
		if err != nil {
			return err
		}
		if actionCreated {
			memberLeaving = append(memberLeaving, getPodName(transCtx.CSSet.Name, int(i)))
		}
	}

	// handle actions
	// barrier: make sure all actions are in cache:
	// 1. all joining members with a final join action
	// 2. all leaving members with a final leave action
	podActionMap, orphanActionList, err := getPodActionMap(transCtx)
	if err != nil {
		return err
	}
	for _, podName := range memberJoining {
		if !hasFinalJoinAction(podName, podActionMap) {
			return nil
		}
	}
	for _, podName := range memberLeaving {
		if !hasFinalLeaveAction(podName, podActionMap) {
			return nil
		}
	}

	// clean up orphan actions
	// if pod not exist, delete all related suspend actions
	for _, action := range orphanActionList {
		doActionCleanup(dag, action)
	}

	// update sts when no pre-actions (i.e. no pre-actions configured or spec.replicas == status.replicas)
	if len(memberLeaving) == 0 {
		updateStsHandler()
	}

	// handle actions in serial order to minimum disrupt on current cluster
	// sort pods in order:
	// 1. action generation in ascend
	// 2. leaving with ordinal descend
	// 3. joining with ordinal ascend
	//
	// e.g. if the 'spec.replicas' changing history is: 3->5->1->7, and the initial object 'generation' is 1.
	// the action list may look like this:
	// action-1-3-join-in-progress | action-1-4-join-suspend | action-2-4-leave-suspend | action-2-3-leave-suspend |
	// action-2-2-leave-suspend | action-2-1-leave-suspend | action-3-1-join-suspend | action-3-2-join-suspend |
	// action-3-3-join-suspend | action-3-4-join-suspend | action-3-5-join-suspend | action-3-6-join-suspend
	//
	// the action list can be shortened:
	// basic idea:
	// if one pod has a leave-join action pair, the pair can be removed.
	// one leave-join pair of pod 4 in the example above: action-1-4-join-suspend | action-2-4-leave-suspend.
	//
	// algorithm:
	// loop the action list and try to put action into a stack
	// if current action and action at the stack top are a leave-join pair, pop
	// else push
	// pop the stack into a list and reverse it
	allActionList := buildAllActionList(podActionMap)
	printActionList(transCtx.Logger, allActionList)
	finalActionList := buildFinalActionList(allActionList)
	printActionList(transCtx.Logger, finalActionList)
	// find first unfinished action
	index := findFirstUnfinishedAction(finalActionList)
	switch {
	case index > 0:
		lastFinishedAction := finalActionList[index-1]
		if lastFinishedAction.Status.Failed > 0 {
			emitActionFailedEvent(transCtx, lastFinishedAction.Labels[jobTypeLabel], lastFinishedAction.Name)
		}
		fallthrough
	case index == 0:
		// start action if suspend
		if *finalActionList[index].Spec.Suspend {
			// validate cluster state: all pods without actions should be ok(role label set and has one leader)
			if err := abnormalAnalysis(pods, podActionMap, roleMap); err != nil {
				emitAbnormalEvent(transCtx, finalActionList[index].Labels[jobTypeLabel], finalActionList[index].Name, err)
				return err
			}
			startAction(dag, finalActionList[index])
		}
	case index < 0:
		// all action finished, do clean up
		for _, ac := range allActionList {
			doActionCleanup(dag, ac)
		}
		updateStsHandler()
	}

	return nil
}

func buildFinalActionList(allActionList []*batchv1.Job) []*batchv1.Job {
	var finalActionList []*batchv1.Job
	for _, action := range allActionList {
		if len(finalActionList) == 0 {
			finalActionList = append(finalActionList, action)
			continue
		}
		lastIndex := len(finalActionList) - 1
		lastAction := finalActionList[lastIndex]
		if isSuspendPair(lastAction, action) {
			finalActionList = finalActionList[:lastIndex]
		} else {
			finalActionList = append(finalActionList, action)
		}
	}
	return finalActionList
}

func printActionList(logger logr.Logger, actionList []*batchv1.Job) {
	var actionNameList []string
	for _, action := range actionList {
		actionNameList = append(actionNameList, fmt.Sprintf("%s-%v", action.Name, *action.Spec.Suspend))
	}
	logger.Info(fmt.Sprintf("action list: %v\n", actionNameList))
}

func isSuspendPair(lastAction, currentAction *batchv1.Job) bool {
	if lastAction.Spec.Suspend != nil && !*lastAction.Spec.Suspend {
		return false
	}
	lastOrdinal, _ := getActionOrdinal(lastAction.Name)
	currentOrdinal, _ := getActionOrdinal(currentAction.Name)
	return lastOrdinal == currentOrdinal
}

func findFirstUnfinishedAction(allActionList []*batchv1.Job) int {
	index := -1
	for i := range allActionList {
		if allActionList[i].Status.Failed > 0 || allActionList[i].Status.Succeeded > 0 {
			continue
		}
		index = i
		break
	}
	return index
}

func buildAllActionList(podActionMap map[string]*podAction) []*batchv1.Job {
	allActionList := make([]*batchv1.Job, 0, len(podActionMap))
	for _, pAction := range podActionMap {
		allActionList = append(allActionList, pAction.actionList...)
	}
	actionTypePriorityMap := map[string]int{
		jobTypeSwitchover:           1,
		jobTypeMemberLeaveNotifying: 2,
		jobTypeMemberJoinNotifying:  3,
		jobTypeLogSync:              4,
		jobTypePromote:              5,
	}
	sort.Slice(allActionList, func(i, j int) bool {
		nameI, nameJ := allActionList[i].Name, allActionList[j].Name
		// name should be legal, no error should be returned
		generationI, ordinalI, actionTypeI, _ := getActionGenerationOrdinalAndType(nameI)
		generationJ, ordinalJ, actionTypeJ, _ := getActionGenerationOrdinalAndType(nameJ)
		switch {
		case generationI == generationJ && ordinalI == ordinalJ:
			return actionTypePriorityMap[actionTypeI] < actionTypePriorityMap[actionTypeJ]
		case generationI == generationJ && (actionTypeI == jobTypeSwitchover || actionTypeI == jobTypeMemberLeaveNotifying):
			return ordinalJ < ordinalI
		case generationI == generationJ:
			return ordinalI < ordinalJ
		default:
			return generationI < generationJ
		}
	})
	return allActionList
}

func hasFinalLeaveAction(podName string, podActionMap map[string]*podAction) bool {
	pAction, ok := podActionMap[podName]
	if !ok {
		return false
	}
	if !pAction.hasPreAction {
		return false
	}
	finalAction := pAction.actionList[len(pAction.actionList)-1]
	actionType := finalAction.Labels[jobTypeLabel]
	return actionType == jobTypeSwitchover || actionType == jobTypeMemberLeaveNotifying
}

func hasFinalJoinAction(podName string, podActionMap map[string]*podAction) bool {
	pAction, ok := podActionMap[podName]
	if !ok {
		return false
	}
	if !pAction.hasPostAction {
		return false
	}
	finalAction := pAction.actionList[len(pAction.actionList)-1]
	actionType := finalAction.Labels[jobTypeLabel]
	return actionType == jobTypeMemberJoinNotifying ||
		actionType == jobTypeLogSync ||
		actionType == jobTypePromote
}

func isStatefulSetReady(sts *apps.StatefulSet) bool {
	if sts.Status.ObservedGeneration == sts.Generation &&
		sts.Status.Replicas == *sts.Spec.Replicas &&
		sts.Status.ReadyReplicas == sts.Status.Replicas {
		return true
	}
	return false
}

// consensus_set level 'ready' state:
// 1. all replicas exist
// 2. all members have role set
func isConsensusSetReady(transCtx *CSSetTransformContext, csSet *workloads.ConsensusSet) (bool, error) {
	if csSet.Status.Replicas != csSet.Spec.Replicas {
		return false, nil
	}
	membersStatus := csSet.Status.MembersStatus
	if len(membersStatus) != int(csSet.Spec.Replicas) {
		return false, nil
	}
	for i := 0; i < int(csSet.Spec.Replicas); i++ {
		podName := getPodName(csSet.Name, i)
		if !isMemberReady(podName, membersStatus) {
			return false, nil
		}
	}
	// no pending actions
	actionLists, err := getAllActionList(transCtx)
	if err != nil {
		return false, err
	}
	for _, actionList := range actionLists {
		if len(actionList.Items) > 0 {
			return false, nil
		}
	}
	return true, nil
}

func isMemberReady(podName string, membersStatus []workloads.ConsensusMemberStatus) bool {
	for _, memberStatus := range membersStatus {
		if memberStatus.PodName == podName {
			return true
		}
	}
	return false
}

func shouldCreateAction(csSet *workloads.ConsensusSet, actionType string, checker conditionChecker) bool {
	if checker != nil && !checker() {
		return false
	}
	reconfiguration := csSet.Spec.MembershipReconfiguration
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

func getAllActionList(transCtx *CSSetTransformContext) ([]*batchv1.JobList, error) {
	actionTypeList := []string{jobTypeSwitchover, jobTypeMemberJoinNotifying, jobTypeMemberLeaveNotifying, jobTypeLogSync, jobTypePromote}
	actionLists := make([]*batchv1.JobList, 0)
	ml := client.MatchingLabels{
		model.AppInstanceLabelKey: transCtx.CSSet.Name,
		model.KBManagedByKey:      kindConsensusSet,
		jobHandledLabel:           jobHandledFalse,
	}
	for _, actionType := range actionTypeList {
		ml[jobTypeLabel] = actionType
		actionList := &batchv1.JobList{}
		if err := transCtx.Client.List(transCtx.Context, actionList, ml); err != nil {
			return nil, err
		}
		actionLists = append(actionLists, actionList)
	}
	return actionLists, nil
}

func getPodName(parent string, ordinal int) string {
	return fmt.Sprintf("%s-%d", parent, ordinal)
}

// with actionList sorted by generation and action priority
func getPodActionMap(transCtx *CSSetTransformContext) (map[string]*podAction, []*batchv1.Job, error) {
	// sort actions by generation and actionType
	actionLists, err := getAllActionList(transCtx)
	if err != nil {
		return nil, nil, err
	}
	podActionMap := make(map[string]*podAction, len(actionLists))
	var orphanActionList []*batchv1.Job
	for _, list := range actionLists {
		for i, action := range list.Items {
			ordinal, err := getActionOrdinal(action.Name)
			if err != nil {
				return nil, nil, err
			}
			if ordinal >= int(transCtx.CSSet.Status.Replicas) && ordinal >= int(transCtx.CSSet.Spec.Replicas) {
				orphanActionList = append(orphanActionList, &list.Items[i])
				continue
			}
			podName := getPodName(transCtx.CSSet.Name, ordinal)
			pAction, ok := podActionMap[podName]
			if !ok {
				pAction = &podAction{}
			}
			pAction.podName = podName
			switch action.Labels[jobTypeLabel] {
			case jobTypeSwitchover, jobTypeMemberLeaveNotifying:
				pAction.hasPreAction = true
			case jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote:
				pAction.hasPostAction = true
			}
			actionList := pAction.actionList
			if actionList == nil {
				actionList = make([]*batchv1.Job, 0)
			}
			actionList = append(actionList, &list.Items[i])
			pAction.actionList = actionList
			podActionMap[podName] = pAction
		}
	}

	return podActionMap, orphanActionList, nil
}

func getActionOrdinal(actionName string) (int, error) {
	subMatches := actionNameRegex.FindStringSubmatch(actionName)
	if len(subMatches) < 5 {
		return 0, fmt.Errorf("error actionName: %s", actionName)
	}
	return strconv.Atoi(subMatches[3])
}

func getActionGenerationOrdinalAndType(actionName string) (string, string, string, error) {
	subMatches := actionNameRegex.FindStringSubmatch(actionName)
	if len(subMatches) < 5 {
		return "", "", "", fmt.Errorf("error actionName: %s", actionName)
	}
	return fmt.Sprintf("%s-%s", subMatches[1], subMatches[2]), subMatches[3], subMatches[4], nil
}

func getUnderlyingStsVertex(dag *graph.DAG) (*model.ObjectVertex, error) {
	vertices := model.FindAll[*apps.StatefulSet](dag)
	if len(vertices) != 1 {
		return nil, fmt.Errorf("unexpected sts found, expected 1, but found: %d", len(vertices))
	}
	stsVertex, _ := vertices[0].(*model.ObjectVertex)
	return stsVertex, nil
}

func getLeaderPod(pods []corev1.Pod, roleMap map[string]workloads.ConsensusRole) string {
	for _, pod := range pods {
		roleName := getRoleName(pod)
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		if role.IsLeader {
			return pod.Name
		}
	}
	return ""
}

// abnormalAnalysis normal conditions: all pods with role label set and one is leader
func abnormalAnalysis(pods []corev1.Pod, podActionMap map[string]*podAction, roleMap map[string]workloads.ConsensusRole) error {
	// find all members in current cluster
	memberPods := make([]corev1.Pod, 0)
	for _, pod := range pods {
		if _, ok := podActionMap[pod.Name]; ok {
			continue
		}
		memberPods = append(memberPods, pod)
	}
	// if no pods, no need to check the following conditions
	if len(memberPods) == 0 {
		return nil
	}
	allRoleLabelSet := true
	leaderCount := 0
	for _, pod := range memberPods {
		roleName := getRoleName(pod)
		if len(roleName) == 0 {
			allRoleLabelSet = false
			break
		}
		if role, ok := roleMap[roleName]; ok {
			if role.IsLeader {
				leaderCount++
			}
		}
	}
	if !allRoleLabelSet {
		return fmt.Errorf("cluster unhealthy: pod missing role label")
	}
	if leaderCount != 1 {
		return fmt.Errorf("cluster unhealthy: # of leader %d not equals 1", leaderCount)
	}
	return nil
}

func doActions(csSet *workloads.ConsensusSet, dag *graph.DAG, leaderPodName string, ordinal int, actionTypeList []string) (bool, error) {
	actionCreated := false
	podName := getPodName(csSet.Name, ordinal)
	for _, actionType := range actionTypeList {
		checker := func() bool {
			return podName == leaderPodName
		}
		if actionType != jobTypeSwitchover {
			checker = nil
		}
		if !shouldCreateAction(csSet, actionType, checker) {
			continue
		}
		env := buildActionEnv(csSet, leaderPodName, podName)
		if err := createAction(csSet, dag, env, actionType, ordinal, true); err != nil {
			return false, err
		}
		actionCreated = true
	}
	return actionCreated, nil
}

// ordinal is the ordinal of pod which this action apply to
func createAction(csSet *workloads.ConsensusSet, dag *graph.DAG, env []corev1.EnvVar, actionType string, ordinal int, suspend bool) error {
	actionName := fmt.Sprintf("%s-%d-%d-%s", csSet.Name, csSet.Generation, ordinal, actionType)
	template := buildActionPodTemplate(csSet, env, actionType)
	action := builder.NewJobBuilder(csSet.Namespace, actionName).
		AddLabels(model.AppInstanceLabelKey, csSet.Name).
		AddLabels(model.KBManagedByKey, kindConsensusSet).
		AddLabels(jobTypeLabel, actionType).
		AddLabels(jobHandledLabel, jobHandledFalse).
		SetPodTemplateSpec(*template).
		SetSuspend(suspend).
		GetObject()
	if err := controllerutil.SetOwnership(csSet, action, model.GetScheme(), csSetFinalizerName); err != nil {
		return err
	}
	model.PrepareCreate(dag, action)
	return nil
}

func buildActionPodTemplate(csSet *workloads.ConsensusSet, env []corev1.EnvVar, actionType string) *corev1.PodTemplateSpec {
	credential := csSet.Spec.Credential
	credentialEnv := make([]corev1.EnvVar, 0)
	if credential != nil {
		credentialEnv = append(credentialEnv,
			corev1.EnvVar{
				Name:      usernameCredentialVarName,
				Value:     credential.Username.Value,
				ValueFrom: credential.Username.ValueFrom,
			},
			corev1.EnvVar{
				Name:      passwordCredentialVarName,
				Value:     credential.Password.Value,
				ValueFrom: credential.Password.ValueFrom,
			})
	}
	env = append(env, credentialEnv...)
	reconfiguration := csSet.Spec.MembershipReconfiguration
	image := findActionImage(reconfiguration, actionType)
	command := getActionCommand(reconfiguration, actionType)
	container := corev1.Container{
		Name:            actionType,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         command,
		Env:             env,
	}
	template := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers:    []corev1.Container{container},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
	return template
}

func buildActionEnv(csSet *workloads.ConsensusSet, leader, target string) []corev1.EnvVar {
	svcName := getHeadlessSvcName(*csSet)
	leaderHost := fmt.Sprintf("%s.%s", leader, svcName)
	targetHost := fmt.Sprintf("%s.%s", target, svcName)
	svcPort := findSvcPort(*csSet)
	return []corev1.EnvVar{
		{
			Name:  leaderHostVarName,
			Value: leaderHost,
		},
		{
			Name:  servicePortVarName,
			Value: strconv.Itoa(svcPort),
		},
		{
			Name:  targetHostVarName,
			Value: targetHost,
		},
	}
}

func findActionImage(reconfiguration *workloads.MembershipReconfiguration, actionType string) string {
	if reconfiguration == nil {
		return ""
	}

	getImage := func(action *workloads.Action) string {
		if action != nil && len(action.Image) > 0 {
			return action.Image
		}
		return ""
	}
	switch actionType {
	case jobTypePromote:
		if image := getImage(reconfiguration.PromoteAction); len(image) > 0 {
			return image
		}
		fallthrough
	case jobTypeLogSync:
		if image := getImage(reconfiguration.LogSyncAction); len(image) > 0 {
			return image
		}
		fallthrough
	case jobTypeMemberLeaveNotifying:
		if image := getImage(reconfiguration.MemberLeaveAction); len(image) > 0 {
			return image
		}
		fallthrough
	case jobTypeMemberJoinNotifying:
		if image := getImage(reconfiguration.MemberJoinAction); len(image) > 0 {
			return image
		}
		fallthrough
	case jobTypeSwitchover:
		if image := getImage(reconfiguration.PromoteAction); len(image) > 0 {
			return image
		}
		return defaultActionImage
	}

	return ""
}

func getActionCommand(reconfiguration *workloads.MembershipReconfiguration, actionType string) []string {
	if reconfiguration == nil {
		return nil
	}
	getCommand := func(action *workloads.Action) []string {
		if action == nil {
			return nil
		}
		return action.Command
	}
	switch actionType {
	case jobTypeSwitchover:
		return getCommand(reconfiguration.SwitchoverAction)
	case jobTypeMemberJoinNotifying:
		return getCommand(reconfiguration.MemberJoinAction)
	case jobTypeMemberLeaveNotifying:
		return getCommand(reconfiguration.MemberLeaveAction)
	case jobTypeLogSync:
		return getCommand(reconfiguration.LogSyncAction)
	case jobTypePromote:
		return getCommand(reconfiguration.PromoteAction)
	}
	return nil
}

func startAction(dag *graph.DAG, action *batchv1.Job) {
	actionOld := action.DeepCopy()
	actionNew := actionOld.DeepCopy()
	suspend := false
	actionNew.Spec.Suspend = &suspend
	model.PrepareUpdate(dag, actionOld, actionNew)
}

func doActionCleanup(dag *graph.DAG, action *batchv1.Job) {
	actionOld := action.DeepCopy()
	actionNew := actionOld.DeepCopy()
	actionNew.Labels[jobHandledLabel] = jobHandledTrue
	model.PrepareUpdate(dag, actionOld, actionNew)
}

func emitActionFailedEvent(transCtx *CSSetTransformContext, actionType, actionName string) {
	message := fmt.Sprintf("%s action failed, job name: %s", actionType, actionName)
	emitActionEvent(transCtx, corev1.EventTypeWarning, actionType, message)
}

func emitAbnormalEvent(transCtx *CSSetTransformContext, actionType, actionName string, err error) {
	message := fmt.Sprintf("%s, job name: %s", err.Error(), actionName)
	emitActionEvent(transCtx, corev1.EventTypeWarning, actionType, message)
}

func emitActionEvent(transCtx *CSSetTransformContext, eventType, reason, message string) {
	transCtx.EventRecorder.Event(transCtx.CSSet, eventType, strings.ToUpper(reason), message)
}

var _ graph.Transformer = &HorizontalScaling2Transformer{}
