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
	"math"
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

// HorizontalScalingTransformer Pod level horizontal scaling handling
type HorizontalScalingTransformer struct{}

type conditionChecker = func() bool

var actionNameRegex = regexp.MustCompile(`(.*)-([0-9]+)-([0-9]+)-([a-zA-Z\-]+)$`)

func (t *HorizontalScalingTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	if model.IsObjectDeleting(transCtx.CSSet) {
		return nil
	}

	// get the underlying sts
	stsVertex, err := getUnderlyingStsVertex(dag)
	if err != nil {
		return err
	}

	// barrier 1: the underlying sts is ready
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

	stsVertex.Immutable = true
	updateStsHandler := func() {
		stsVertex.Immutable = false
	}

	// barrier 2: latest action list in the api-server
	if model.IsObjectUpdating(transCtx.OrigCSSet) {
		if transCtx.CSSet.Status.Replicas == transCtx.CSSet.Spec.Replicas {
			return nil
		}
		return generateActions(transCtx, dag)
	}

	// barrier 3: if scale out, make sure new pods are ready
	if len(transCtx.CSSet.Status.MembersStatus) < int(transCtx.CSSet.Spec.Replicas) &&
		sts.Status.ReadyReplicas != transCtx.CSSet.Spec.Replicas {
		updateStsHandler()
		return nil
	}

	// compose action list
	// sort actions in order:
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
	allActionList, err := buildAllActionList(transCtx)
	if err != nil {
		return err
	}
	printActionList(transCtx.Logger, allActionList)
	finalActionList := buildFinalActionList(allActionList)
	printActionList(transCtx.Logger, finalActionList)

	// barrier 4: make sure latest action list is in cache
	// why should have the latest action list?
	// one case:
	// replicas: 5->3, local action list: member-5-leave
	// if action(member-5-leave) is finished,
	// the list is all handled and sts should be updated, which will kill (member)pod-4 without do action(member-4-leave)
	if !isLatestActionList(finalActionList, transCtx.CSSet) {
		return nil
	}

	// finally, have the latest action list, handle it
	// handle actions in serial order to minimum disrupt on current cluster
	// find first unfinished action
	index := findFirstUnfinishedAction(finalActionList)
	switch {
	case index > 0:
		// last finished action
		emitEvent(transCtx, finalActionList[index-1])
		fallthrough
	case index == 0:
		// start action if suspend
		action := finalActionList[index]
		if *action.Spec.Suspend {
			// barrier 5: members with ordinal less than action target ordinal should be ready
			// validate cluster state: all pods without actions should be ok(role label set and has one leader)
			if err := abnormalAnalysis(transCtx.CSSet, action); err != nil {
				emitAbnormalEvent(transCtx, action.Labels[jobTypeLabel], action.Name, err)
				return err
			}
			startAction(dag, action)
		}
	case index < 0:
		// last finished action
		emitEvent(transCtx, finalActionList[len(finalActionList)-1])
		// all action finished, do clean up
		for _, ac := range allActionList {
			doActionCleanup(dag, ac)
		}
		updateStsHandler()
	}

	if len(transCtx.CSSet.Status.MembersStatus) < int(transCtx.CSSet.Spec.Replicas) {
		updateStsHandler()
	}

	return nil
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

func generateActions(transCtx *CSSetTransformContext, dag *graph.DAG) error {
	leaderPodName := getLeaderPodName(transCtx.CSSet.Status.MembersStatus)
	// member join
	// members with ordinal less than 'spec.replicas' should in the consensus cluster
	actionTypeList := []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
	for i := 0; i < int(transCtx.CSSet.Spec.Replicas); i++ {
		podName := getPodName(transCtx.CSSet.Name, i)
		if isMemberReady(podName, transCtx.CSSet.Status.MembersStatus) {
			continue
		}
		if err := doActions(transCtx.CSSet, dag, leaderPodName, i, actionTypeList); err != nil {
			return err
		}
	}
	// member leave
	// members with ordinal greater than 'spec.replicas - 1' should not in the consensus cluster
	actionTypeList = []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
	for i := transCtx.CSSet.Spec.Replicas; i < transCtx.CSSet.Status.Replicas; i++ {
		if err := doActions(transCtx.CSSet, dag, leaderPodName, int(i), actionTypeList); err != nil {
			return err
		}
	}
	return nil
}

// sorted by generation and action priority
func buildAllActionList(transCtx *CSSetTransformContext) ([]*batchv1.Job, error) {
	// get all actions in cache
	actionLists, err := getAllActionList(transCtx)
	if err != nil {
		return nil, err
	}

	// put all actions into a list
	var allActionList []*batchv1.Job
	for _, list := range actionLists {
		for i := range list.Items {
			allActionList = append(allActionList, &list.Items[i])
		}
	}

	// sort the action list
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
		case generationI == generationJ && isPreAction(actionTypeI):
			return ordinalJ < ordinalI
		case generationI == generationJ:
			return ordinalI < ordinalJ
		default:
			return generationI < generationJ
		}
	})
	return allActionList, nil
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
		switch {
		case isSuspendPair(lastAction, action):
			finalActionList = finalActionList[:lastIndex]
		case isAdjacentPair(lastAction, action):
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

func isAdjacentPair(lastAction, currentAction *batchv1.Job) bool {
	if lastAction.Spec.Suspend != nil && !*lastAction.Spec.Suspend {
		return true
	}
	lastOrdinal, _ := getActionOrdinal(lastAction.Name)
	currentOrdinal, _ := getActionOrdinal(currentAction.Name)
	lastPre := isPreAction(lastAction.Labels[jobTypeLabel])
	currentPre := isPreAction(currentAction.Labels[jobTypeLabel])
	// scale out: member-3-post-join adjacent with member-4-post-join
	// scale in:  member-4-pre-leave adjacent with member-3-pre-leave
	return (math.Abs(float64(lastOrdinal-currentOrdinal)) == 1) && (lastPre==currentPre)
}

func isPreAction(actionType string) bool {
	return actionType == jobTypeSwitchover || actionType == jobTypeMemberLeaveNotifying
}

func isLatestActionList(actionList []*batchv1.Job, csSet *workloads.ConsensusSet) bool {
	// no actions as membershipReconfiguration not configured
	// bad case: membership configuration is updated during action list generation.
	// this should be rare, ignore it currently.
	// for the users: don't do `spec.membershipReconfiguration` update during horizontal scaling.
	if !shouldHaveActions(csSet) {
		return true
	}

	// no actions come yet, but should
	if len(actionList) == 0 {
		return false
	}

	// get the last action, its target should be the last member which has ordinal equals to `spec.replicas - 1`
	action := actionList[len(actionList)-1]
	ordinal, _ := getActionOrdinal(action.Name)
	switch {
	case len(csSet.Status.MembersStatus) < int(csSet.Spec.Replicas):
		return ordinal == int(csSet.Spec.Replicas-1)
	default:
		return ordinal == int(csSet.Spec.Replicas)
	}
}

func shouldHaveActions(csSet *workloads.ConsensusSet) bool {
	currentReplicas := len(csSet.Status.MembersStatus)
	expectedReplicas := int(csSet.Spec.Replicas)

	var actionTypeList []string
	switch {
	case currentReplicas > expectedReplicas:
		actionTypeList = []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
	case currentReplicas < expectedReplicas:
		actionTypeList = []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
	}
	for _, actionType := range actionTypeList {
		if shouldCreateAction(csSet, actionType, nil) {
			return true
		}
	}
	return false
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

func getLeaderPodName(membersStatus []workloads.ConsensusMemberStatus) string {
	for _, memberStatus := range membersStatus {
		if memberStatus.IsLeader {
			return memberStatus.PodName
		}
	}
	return ""
}

// all members with ordinal less than action target pod should be in a good consensus state:
// 1. they should be in membersStatus
// 2. they should have a leader
func abnormalAnalysis(csSet *workloads.ConsensusSet, action *batchv1.Job) error {
	membersStatus := csSet.Status.MembersStatus
	statusMap := make(map[string]workloads.ConsensusMemberStatus, len(membersStatus))
	for _, status := range membersStatus {
		statusMap[status.PodName] = status
	}
	ordinal, _ := getActionOrdinal(action.Name)
	var abnormalPodList, leaderPodList []string
	for i := 0; i < ordinal; i++ {
		podName := getPodName(csSet.Name, i)
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

func doActions(csSet *workloads.ConsensusSet, dag *graph.DAG, leaderPodName string, ordinal int, actionTypeList []string) error {
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
			return err
		}
	}
	return nil
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

func emitEvent(transCtx *CSSetTransformContext, action *batchv1.Job) {
	switch {
	case action.Status.Succeeded > 0:
		emitActionSucceedEvent(transCtx, action.Labels[jobTypeLabel], action.Name)
	case action.Status.Failed > 0:
		emitActionFailedEvent(transCtx, action.Labels[jobTypeLabel], action.Name)
	}
}

func emitActionSucceedEvent(transCtx *CSSetTransformContext, actionType, actionName string) {
	message := fmt.Sprintf("%s succeed, job name: %s", actionType, actionName)
	emitActionEvent(transCtx, corev1.EventTypeNormal, actionType, message)
}

func emitActionFailedEvent(transCtx *CSSetTransformContext, actionType, actionName string) {
	message := fmt.Sprintf("%s failed, job name: %s", actionType, actionName)
	emitActionEvent(transCtx, corev1.EventTypeWarning, actionType, message)
}

func emitAbnormalEvent(transCtx *CSSetTransformContext, actionType, actionName string, err error) {
	message := fmt.Sprintf("%s, job name: %s", err.Error(), actionName)
	emitActionEvent(transCtx, corev1.EventTypeWarning, actionType, message)
}

func emitActionEvent(transCtx *CSSetTransformContext, eventType, reason, message string) {
	transCtx.EventRecorder.Event(transCtx.CSSet, eventType, strings.ToUpper(reason), message)
}

var _ graph.Transformer = &HorizontalScalingTransformer{}
