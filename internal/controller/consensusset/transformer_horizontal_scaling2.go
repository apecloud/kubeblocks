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
type HorizontalScaling2Transformer struct {}

type podAction struct {
	podName       string
	hasPreAction  bool
	hasPostAction bool
	actionList    []*batchv1.Job
}

var actionNameRegex = regexp.MustCompile("(.*)-([0-9]+)-([0-9]+)-([a-zA-Z]+)$")

func (t *HorizontalScaling2Transformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// handle Update only, i.e. consensus cluster exists
	transCtx, _ := ctx.(*CSSetTransformContext)
	if model.IsObjectDeleting(transCtx.CSSet) {
		return nil
	}
	// get the underlying sts
	stsVertex, err := getUnderlyingStsVertex(dag)
	if err != nil {
		return err
	}
	if stsVertex.Action == nil || *stsVertex.Action != model.UPDATE {
		return nil
	}
	sts, _ := stsVertex.Obj.(*apps.StatefulSet)
	pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, sts)
	if err != nil {
		return err
	}

	// handle membership in consensus_set level and pod lifecycle in stateful_set
	// pre-conditions validation: make sure sts level is ok
	if sts.Status.ObservedGeneration != sts.Generation ||
		sts.Status.Replicas != *sts.Spec.Replicas ||
		sts.Status.ReadyReplicas != sts.Status.Replicas ||
		len(pods) != int(sts.Status.Replicas) {
		return nil
	}

	actionLists, err := getAllActionList(transCtx)
	if err != nil {
		return err
	}
	noPendingActions := func() bool {
		for _, actionList := range actionLists {
			if len(actionList.Items) > 0 {
				return false
			}
		}
		return true
	}
	// replicas in status is same as in spec, and no pending membership action exists, no need to handle scaling
	if transCtx.CSSet.Spec.Replicas == transCtx.CSSet.Status.Replicas &&
		noPendingActions() {
		return nil
	}

	// handle horizontal scaling
	stsVertex.Immutable = true
	updateStsHandler := func() {
		stsVertex.Immutable = false
	}
	// handle spec update
	// compute diff set: pods to be created and pods to be deleted
	roleMap := composeRoleMap(*transCtx.CSSet)
	leaderPodName := getLeaderPod(pods, roleMap)
	createActions := func(ordinal int, actionTypeList []string) (bool, error) {
		podName := getPodName(transCtx.CSSet.Name, ordinal)
		hasActionCreated := false
		for _, actionType := range actionTypeList {
			checker := func() bool {
				return podName == leaderPodName
			}
			if actionType == jobTypeSwitchover {
				checker = nil
			}
			if !shouldDoAction(transCtx.CSSet, actionType, checker) {
				continue
			}
			env := buildActionEnv(transCtx.CSSet, leaderPodName, podName)
			if err := doAction(transCtx.CSSet, dag, env, actionType, ordinal, true); err != nil {
				return false, err
			}
			hasActionCreated = true
		}
		return hasActionCreated, nil
	}
	// handle member join
	jobTypeList := []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
	for i := transCtx.CSSet.Status.Replicas; i < transCtx.CSSet.Spec.Replicas; i++ {
		if _, err := createActions(int(i), jobTypeList); err != nil {
			return err
		}
	}
	// tell sts to create new pods and wait them ready
	if transCtx.CSSet.Status.Replicas < transCtx.CSSet.Spec.Replicas {
		updateStsHandler()
		return nil
	}

	// handle member leave
	jobTypeList = []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
	memberLeaveList := make([]string, 0)
	for i := transCtx.CSSet.Spec.Replicas; i < transCtx.CSSet.Status.Replicas; i++ {
		hasActionCreated, err := createActions(int(i), jobTypeList)
		if err != nil {
			return err
		}
		if hasActionCreated {
			memberLeaveList = append(memberLeaveList, getPodName(transCtx.CSSet.Name, int(i)))
		}
	}

	// handle membership of each pod with unhandled actions
	// compose pod-action map
	podActionMap, orphanActionList, err := getPodActionMap(transCtx)
	if err != nil {
		return err
	}

	// if pod to be deleted with no pre actions(but should), return and wait actions synced into cache
	for _, podName := range memberLeaveList {
		if pAction, ok := podActionMap[podName]; !ok || !pAction.hasPreAction {
			return nil
		}
	}
	// if pod not exist, delete all related suspend actions
	for _, action := range orphanActionList {
		doActionCleanup(dag, action)
	}
	// if both pre and post suspend actions exist, delete actions
	for podName, pAction := range podActionMap {
		if !pAction.hasPreAction || !pAction.hasPostAction {
			continue
		}
		if !isAllSuspend(pAction.actionList) {
			continue
		}
		for _, job := range pAction.actionList {
			doActionCleanup(dag, job)
		}
		delete(podActionMap, podName)
	}
	// if action list empty, delete pods
	if len(podActionMap) == 0 {
		updateStsHandler()
		return nil
	}

	// handle membership actions
	for _, pAction := range podActionMap {
		job := pAction.actionList[0]
		switch {
		case *job.Spec.Suspend:
			// TODO(free6om): validate cluster state: all pods without actions should be ok(role label set and has one leader)
			if err := abnormalAnalysis(transCtx, pods, roleMap); err != nil {
				return err
			}
			startAction(dag, pAction.actionList[0])
		case job.Status.Succeeded == 0 && job.Status.Failed == 0:
			// in progress, do nothing and wait job done event
		case job.Status.Failed > 0:
			emitControlJobFailedEvent(transCtx, job.Labels[jobTypeLabel])
			fallthrough
		case job.Status.Succeeded > 0:
			doActionCleanup(dag, job)
		}
	}
	return nil
}

func shouldDoAction(csSet *workloads.ConsensusSet, actionType string, checker preConditionChecker) bool {
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
	jobTypeList := []string{jobTypeSwitchover, jobTypeMemberJoinNotifying, jobTypeMemberLeaveNotifying, jobTypeLogSync, jobTypePromote}
	jobLists := make([]*batchv1.JobList, 0)
	ml := client.MatchingLabels{
		model.AppInstanceLabelKey: transCtx.CSSet.Name,
		model.KBManagedByKey:      kindConsensusSet,
		jobHandledLabel:           jobHandledFalse,
	}
	for _, jobType := range jobTypeList {
		ml[jobTypeLabel] = jobType
		jobList := &batchv1.JobList{}
		if err := transCtx.Client.List(transCtx.Context, jobList, ml); err != nil {
			return nil, err
		}
		jobLists = append(jobLists, jobList)
	}
	return jobLists, nil
}

func getPodName(parent string, ordinal int) string {
	return fmt.Sprintf("%s-%d", parent, ordinal)
}

func isAllSuspend(jobList []*batchv1.Job) bool {
	for _, job := range jobList {
		suspend := job.Spec.Suspend
		if suspend == nil || !*suspend {
			return false
		}
	}
	return true
}

func getPodActionMap(transCtx *CSSetTransformContext) (map[string]*podAction, []*batchv1.Job, error) {
	// sort actions by generation and jobType
	actionLists, err := getAllActionList(transCtx)
	if err != nil {
		return nil, nil, err
	}
	podActionMap := make(map[string]*podAction, 0)
	orphanActionList := make([]*batchv1.Job, 0)
	for _, list := range actionLists {
		for _, action := range list.Items {
			ordinal, err := getActionOrdinal(action.Name)
			if err != nil {
				return nil, nil, err
			}
			if ordinal >= int(transCtx.CSSet.Status.Replicas) {
				orphanActionList = append(orphanActionList, &action)
				continue
			}
			podName := getPodName(transCtx.CSSet.Name, ordinal)
			pAction, ok := podActionMap[podName]
			if !ok {
				pAction = &podAction{}
			}
			actionList := pAction.actionList
			if actionList == nil {
				actionList = make([]*batchv1.Job, 0)
			}
			actionList = append(actionList, &action)
			pAction.actionList = actionList
			podActionMap[podName] = pAction
		}
	}
	for _, pAction := range podActionMap {
		sort.Slice(pAction.actionList, func(i, j int) bool {
			return pAction.actionList[i].Name < pAction.actionList[j].Name
		})
	}

	return podActionMap, orphanActionList, nil
}

func getActionOrdinal(actionName string) (int, error) {
	subMatches := actionNameRegex.FindStringSubmatch(actionName)
	if len(subMatches) < 5 {
		return 0, fmt.Errorf("error jobName: %s", actionName)
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
func abnormalAnalysis(transCtx *CSSetTransformContext, pods []corev1.Pod, roleMap map[string]workloads.ConsensusRole) error {
	if len(pods) != int(transCtx.CSSet.Status.Replicas) {
		// TODO(free6om): should handle this error in a more user-friendly way
		// set condition, emit event if error happens consecutive x times.
		return fmt.Errorf("cluster unhealthy: # of pods %d not equals to replicas %d", len(pods), transCtx.OrigCSSet.Status.Replicas)
	}
	// if no pods, no need to check the following conditions
	if len(pods) == 0 {
		return nil
	}
	allRoleLabelSet := true
	leaderCount := 0
	for _, pod := range pods {
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

// ordinal is the ordinal of pod which this action apply to
func doAction(csSet *workloads.ConsensusSet, dag *graph.DAG, env []corev1.EnvVar, actionType string, ordinal int, suspend bool) error {
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

var _ graph.Transformer = &HorizontalScaling2Transformer{}
