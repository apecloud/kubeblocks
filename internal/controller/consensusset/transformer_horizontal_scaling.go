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

type actionInfo struct {
	shortActionName string
	ordinal         int
	actionType      string
}

type conditionChecker = func() bool

var actionNameRegex = regexp.MustCompile(`(.*)-([0-9]+)-([0-9]+)-([a-zA-Z\-]+)$`)

func (t *HorizontalScalingTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	if model.IsObjectDeleting(transCtx.CSSet) {
		return nil
	}
	csSet := transCtx.CSSet

	// get the underlying sts
	stsVertex, err := getUnderlyingStsVertex(dag)
	if err != nil {
		return err
	}

	// handle cluster initialization
	// set initReplicas at creation
	if csSet.Status.InitReplicas == 0 {
		csSet.Status.InitReplicas = csSet.Spec.Replicas
		return nil
	}
	// update readyInitReplicas
	if csSet.Status.ReadyInitReplicas < csSet.Status.InitReplicas {
		csSet.Status.ReadyInitReplicas = int32(len(csSet.Status.MembersStatus))
	}
	// return if cluster initialization not done
	if csSet.Status.ReadyInitReplicas != csSet.Status.InitReplicas {
		return nil
	}

	// cluster initialization done, handle dynamic membership reconfiguration

	// consensus cluster is ready
	if isConsensusSetReady(csSet) {
		return cleanAction(transCtx, dag)
	}

	if !shouldHaveActions(csSet) {
		return nil
	}

	// no enough replicas in scale out, tell sts to create them.
	sts, _ := stsVertex.OriObj.(*apps.StatefulSet)
	memberReadyReplicas := int32(len(csSet.Status.MembersStatus))
	if memberReadyReplicas < csSet.Spec.Replicas &&
		sts.Status.ReadyReplicas < csSet.Spec.Replicas {
		return nil
	}

	stsVertex.Immutable = true

	// barrier: the underlying sts is ready and has enough replicas
	if sts.Status.ReadyReplicas < csSet.Spec.Replicas || !isStatefulSetReady(sts) {
		return nil
	}

	// get last action
	actionList, err := getActionList(transCtx)
	if err != nil {
		return err
	}

	// if no action, create the first one
	if len(actionList) == 0 {
		return createNextAction(transCtx, dag, csSet, nil)
	}

	// got action, there should be only one action
	action := actionList[0]
	switch {
	case action.Status.Succeeded > 0:
		// wait action's result:
		// e.g. action with ordinal 3 and type member-join, wait member 3 until it appears in status.membersStatus
		if !isActionDone(csSet, action) {
			return nil
		}
		// mark it as 'handled'
		deleteAction(dag, action)
		return createNextAction(transCtx, dag, csSet, action)
	case action.Status.Failed > 0:
		emitEvent(transCtx, action)
		if !isSwitchoverAction(action) {
			// need manual handling
			return nil
		}
		return createNextAction(transCtx, dag, csSet, action)
	default:
		// action in progress
		return nil
	}
}

// consensus_set level 'ready' state:
// 1. all replicas exist
// 2. all members have role set
func isConsensusSetReady(csSet *workloads.ConsensusSet) bool {
	membersStatus := csSet.Status.MembersStatus
	if len(membersStatus) != int(csSet.Spec.Replicas) {
		return false
	}
	for i := 0; i < int(csSet.Spec.Replicas); i++ {
		podName := getPodName(csSet.Name, i)
		if !isMemberReady(podName, membersStatus) {
			return false
		}
	}
	return true
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

func isMemberReady(podName string, membersStatus []workloads.ConsensusMemberStatus) bool {
	for _, memberStatus := range membersStatus {
		if memberStatus.PodName == podName {
			return true
		}
	}
	return false
}

func cleanAction(transCtx *CSSetTransformContext, dag *graph.DAG) error {
	actionList, err := getActionList(transCtx)
	if err != nil {
		return err
	}
	if len(actionList) == 0 {
		return nil
	}
	action := actionList[0]
	switch {
	case action.Status.Succeeded > 0:
		deleteAction(dag, action)
	case action.Status.Failed > 0:
		emitEvent(transCtx, action)
	}
	return nil
}

func isActionDone(csSet *workloads.ConsensusSet, action *batchv1.Job) bool {
	ordinal, _ := getActionOrdinal(action.Name)
	podName := getPodName(csSet.Name, ordinal)
	membersStatus := csSet.Status.MembersStatus
	switch action.Labels[jobTypeLabel] {
	case jobTypeSwitchover:
		leader := getLeaderPodName(csSet.Status.MembersStatus)
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

func deleteAction(dag *graph.DAG, action *batchv1.Job) {
	doActionCleanup(dag, action)
}

func createNextAction(transCtx *CSSetTransformContext, dag *graph.DAG, csSet *workloads.ConsensusSet, currentAction *batchv1.Job) error {
	actionInfoList := generateActionInfoList(csSet)

	if len(actionInfoList) == 0 {
		return nil
	}

	var nextActionInfo *actionInfo
	switch {
	case currentAction == nil, isSwitchoverAction(currentAction):
		nextActionInfo = actionInfoList[0]
	default:
		nextActionInfo = nil
		ordinal, _ := getActionOrdinal(currentAction.Name)
		shortName := buildShortActionName(csSet.Name, ordinal, currentAction.Labels[jobTypeLabel])
		for i := 0; i < len(actionInfoList); i++ {
			if actionInfoList[i].shortActionName != shortName {
				continue
			}
			if i+1 < len(actionInfoList) {
				nextActionInfo = actionInfoList[i+1]
				break
			}
		}
	}

	if nextActionInfo == nil {
		return nil
	}

	leader := getLeaderPodName(csSet.Status.MembersStatus)
	ordinal := nextActionInfo.ordinal
	if nextActionInfo.actionType == jobTypeSwitchover {
		ordinal = 0
	}
	target := getPodName(csSet.Name, ordinal)
	actionName := buildActionName(csSet.Name, int(csSet.Generation), ordinal, nextActionInfo.actionType)
	nextAction := buildAction(csSet, actionName, nextActionInfo.actionType, leader, target, false)

	if err := abnormalAnalysis(csSet, nextAction); err != nil {
		emitAbnormalEvent(transCtx, nextActionInfo.actionType, nextActionInfo.shortActionName, err)
		return err
	}

	return createAction(dag, csSet, nextAction)
}

func generateActionInfoList(csSet *workloads.ConsensusSet) []*actionInfo {
	var actionInfoList []*actionInfo
	memberReadyReplicas := int32(len(csSet.Status.MembersStatus))

	switch {
	case memberReadyReplicas < csSet.Spec.Replicas:
		// member join
		// members with ordinal less than 'spec.replicas' should in the consensus cluster
		actionTypeList := []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
		for i := memberReadyReplicas; i < csSet.Spec.Replicas; i++ {
			actionInfos := generateActionInfos(csSet, int(i), actionTypeList)
			actionInfoList = append(actionInfoList, actionInfos...)
		}
	case memberReadyReplicas > csSet.Spec.Replicas:
		// member leave
		// members with ordinal greater than 'spec.replicas - 1' should not in the consensus cluster
		actionTypeList := []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
		for i := memberReadyReplicas - 1; i >= csSet.Spec.Replicas; i-- {
			actionInfos := generateActionInfos(csSet, int(i), actionTypeList)
			actionInfoList = append(actionInfoList, actionInfos...)
		}
	}

	return actionInfoList
}

// TODO(free6om): remove all printActionList when all testes pass
func printActionList(logger logr.Logger, actionList []*batchv1.Job) {
	var actionNameList []string
	for _, action := range actionList {
		actionNameList = append(actionNameList, fmt.Sprintf("%s-%v", action.Name, *action.Spec.Suspend))
	}
	logger.Info(fmt.Sprintf("action list: %v\n", actionNameList))
}

func isPreAction(actionType string) bool {
	return actionType == jobTypeSwitchover || actionType == jobTypeMemberLeaveNotifying
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

func getActionList(transCtx *CSSetTransformContext) ([]*batchv1.Job, error) {
	var actionList []*batchv1.Job
	ml := client.MatchingLabels{
		model.AppInstanceLabelKey: transCtx.CSSet.Name,
		model.KBManagedByKey:      kindConsensusSet,
		jobHandledLabel:           jobHandledFalse,
	}
	jobList := &batchv1.JobList{}
	if err := transCtx.Client.List(transCtx.Context, jobList, ml); err != nil {
		return nil, err
	}
	for i := range jobList.Items {
		actionList = append(actionList, &jobList.Items[i])
	}
	printActionList(transCtx.Logger, actionList)
	return actionList, nil
}

func getPodName(parent string, ordinal int) string {
	return fmt.Sprintf("%s-%d", parent, ordinal)
}

func buildActionName(parent string, generation, ordinal int, actionType string) string {
	return fmt.Sprintf("%s-%d-%d-%s", parent, generation, ordinal, actionType)
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
	currentMembers := ordinal
	if isPreAction(action.Labels[jobTypeLabel]) {
		currentMembers = ordinal + 1
	}
	var abnormalPodList, leaderPodList []string
	for i := 0; i < currentMembers; i++ {
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

func generateActionInfos(csSet *workloads.ConsensusSet, ordinal int, actionTypeList []string) []*actionInfo {
	var actionInfos []*actionInfo
	leaderPodName := getLeaderPodName(csSet.Status.MembersStatus)
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
		info := &actionInfo{
			shortActionName: buildShortActionName(csSet.Name, ordinal, actionType),
			ordinal:         ordinal,
			actionType:      actionType,
		}
		actionInfos = append(actionInfos, info)
	}
	return actionInfos
}

// ordinal is the ordinal of pod which this action apply to
func createAction(dag *graph.DAG, csSet *workloads.ConsensusSet, action *batchv1.Job) error {
	if err := controllerutil.SetOwnership(csSet, action, model.GetScheme(), csSetFinalizerName); err != nil {
		return err
	}
	model.PrepareCreate(dag, action)
	return nil
}

func buildAction(csSet *workloads.ConsensusSet, actionName, actionType string, leader, target string, suspend bool) *batchv1.Job {
	env := buildActionEnv(csSet, leader, target)
	template := buildActionPodTemplate(csSet, env, actionType)
	return builder.NewJobBuilder(csSet.Namespace, actionName).
		AddLabels(model.AppInstanceLabelKey, csSet.Name).
		AddLabels(model.KBManagedByKey, kindConsensusSet).
		AddLabels(jobTypeLabel, actionType).
		AddLabels(jobHandledLabel, jobHandledFalse).
		SetPodTemplateSpec(*template).
		SetSuspend(suspend).
		GetObject()
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
		if image := getImage(reconfiguration.SwitchoverAction); len(image) > 0 {
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
