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
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type getRole func(int) string
type getOrdinal func(int) int

const (
	leaderPriority            = 1 << 5
	followerReadWritePriority = 1 << 4
	followerReadonlyPriority  = 1 << 3
	followerNonePriority      = 1 << 2
	learnerPriority           = 1 << 1
	emptyPriority             = 1 << 0
	// unknownPriority           = 0
)

var podNameRegex = regexp.MustCompile(`(.*)-([0-9]+)$`)

// sortPods sorts pods by their role priority
// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name < follower2.Name
// reverse it if reverse==true
func sortPods(pods []corev1.Pod, rolePriorityMap map[string]int, reverse bool) {
	getRoleFunc := func(i int) string {
		return getRoleName(pods[i])
	}
	getOrdinalFunc := func(i int) int {
		_, ordinal := intctrlutil.GetParentNameAndOrdinal(&pods[i])
		return ordinal
	}
	sortMembers(pods, rolePriorityMap, getRoleFunc, getOrdinalFunc, reverse)
}

func sortMembersStatus(membersStatus []workloads.MemberStatus, rolePriorityMap map[string]int) {
	getRoleFunc := func(i int) string {
		return membersStatus[i].Name
	}
	getOrdinalFunc := func(i int) int {
		ordinal, _ := getPodOrdinal(membersStatus[i].PodName)
		return ordinal
	}
	sortMembers(membersStatus, rolePriorityMap, getRoleFunc, getOrdinalFunc, true)
}

func sortMembers[T any](membersStatus []T,
	rolePriorityMap map[string]int,
	getRoleFunc getRole, getOrdinalFunc getOrdinal,
	reverse bool) {
	sort.SliceStable(membersStatus, func(i, j int) bool {
		roleI := getRoleFunc(i)
		roleJ := getRoleFunc(j)
		if reverse {
			roleI, roleJ = roleJ, roleI
		}

		if rolePriorityMap[roleI] == rolePriorityMap[roleJ] {
			ordinal1 := getOrdinalFunc(i)
			ordinal2 := getOrdinalFunc(j)
			if reverse {
				ordinal1, ordinal2 = ordinal2, ordinal1
			}
			return ordinal1 < ordinal2
		}

		return rolePriorityMap[roleI] < rolePriorityMap[roleJ]
	})
}

// composeRolePriorityMap generates a priority map based on roles.
func composeRolePriorityMap(set workloads.StatefulReplicaSet) map[string]int {
	rolePriorityMap := make(map[string]int, 0)
	rolePriorityMap[""] = emptyPriority
	for _, role := range set.Spec.Roles {
		roleName := strings.ToLower(role.Name)
		switch {
		case role.IsLeader:
			rolePriorityMap[roleName] = leaderPriority
		case role.CanVote:
			switch role.AccessMode {
			case workloads.NoneMode:
				rolePriorityMap[roleName] = followerNonePriority
			case workloads.ReadonlyMode:
				rolePriorityMap[roleName] = followerReadonlyPriority
			case workloads.ReadWriteMode:
				rolePriorityMap[roleName] = followerReadWritePriority
			}
		default:
			rolePriorityMap[roleName] = learnerPriority
		}
	}

	return rolePriorityMap
}

// updatePodRoleLabel updates pod role label when internal container role changed
func updatePodRoleLabel(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	set workloads.StatefulReplicaSet,
	pod *corev1.Pod, roleName string) error {
	ctx := reqCtx.Ctx
	roleMap := composeRoleMap(set)
	// role not defined in CR, ignore it
	roleName = strings.ToLower(roleName)

	// update pod role label
	patch := client.MergeFrom(pod.DeepCopy())
	role, ok := roleMap[roleName]
	switch ok {
	case true:
		pod.Labels[model.RoleLabelKey] = role.Name
		pod.Labels[model.ConsensusSetAccessModeLabelKey] = string(role.AccessMode)
	case false:
		delete(pod.Labels, model.RoleLabelKey)
		delete(pod.Labels, model.ConsensusSetAccessModeLabelKey)
	}
	return cli.Patch(ctx, pod, patch)
}

func composeRoleMap(set workloads.StatefulReplicaSet) map[string]workloads.ReplicaRole {
	roleMap := make(map[string]workloads.ReplicaRole, 0)
	for _, role := range set.Spec.Roles {
		roleMap[strings.ToLower(role.Name)] = role
	}
	return roleMap
}

func setMembersStatus(set *workloads.StatefulReplicaSet, pods []corev1.Pod) {
	// compose new status
	newMembersStatus := make([]workloads.MemberStatus, 0)
	roleMap := composeRoleMap(*set)
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			continue
		}
		roleName := getRoleName(pod)
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		memberStatus := workloads.MemberStatus{
			PodName:     pod.Name,
			ReplicaRole: role,
		}
		newMembersStatus = append(newMembersStatus, memberStatus)
	}

	// members(pods) being scheduled should be kept
	oldMemberMap := make(map[string]*workloads.MemberStatus, len(set.Status.MembersStatus))
	for i, status := range set.Status.MembersStatus {
		oldMemberMap[status.PodName] = &set.Status.MembersStatus[i]
	}
	newMemberMap := make(map[string]*workloads.MemberStatus, len(newMembersStatus))
	for i, status := range newMembersStatus {
		newMemberMap[status.PodName] = &newMembersStatus[i]
	}
	oldMemberSet := sets.KeySet(oldMemberMap)
	newMemberSet := sets.KeySet(newMemberMap)
	memberToKeepSet := oldMemberSet.Difference(newMemberSet)
	for podName := range memberToKeepSet {
		ordinal, _ := getPodOrdinal(podName)
		// members have left because of scale-in
		if ordinal >= int(set.Spec.Replicas) {
			continue
		}
		newMembersStatus = append(newMembersStatus, *oldMemberMap[podName])
	}

	rolePriorityMap := composeRolePriorityMap(*set)
	sortMembersStatus(newMembersStatus, rolePriorityMap)
	set.Status.MembersStatus = newMembersStatus
}

func getRoleName(pod corev1.Pod) string {
	return strings.ToLower(pod.Labels[constant.RoleLabelKey])
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&appsv1.StatefulSetList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&policyv1.PodDisruptionBudgetList{},
	}
}

func deletionKinds() []client.ObjectList {
	kinds := ownedKinds()
	kinds = append(kinds, &corev1.PersistentVolumeClaimList{}, &batchv1.JobList{})
	return kinds
}

func getPodsOfStatefulSet(ctx context.Context, cli roclient.ReadonlyClient, stsObj *appsv1.StatefulSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: stsObj.Namespace},
		client.MatchingLabels{
			model.KBManagedByKey:      stsObj.Labels[model.KBManagedByKey],
			model.AppInstanceLabelKey: stsObj.Labels[model.AppInstanceLabelKey],
		}); err != nil {
		return nil, err
	}
	var pods []corev1.Pod
	for _, pod := range podList.Items {
		if util.IsMemberOf(stsObj, &pod) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func getHeadlessSvcName(set workloads.StatefulReplicaSet) string {
	return strings.Join([]string{set.Name, "headless"}, "-")
}

func findSvcPort(csSet workloads.StatefulReplicaSet) int {
	port := csSet.Spec.Service.Ports[0]
	for _, c := range csSet.Spec.Template.Spec.Containers {
		for _, p := range c.Ports {
			if port.TargetPort.Type == intstr.String && p.Name == port.TargetPort.StrVal ||
				port.TargetPort.Type == intstr.Int && p.ContainerPort == port.TargetPort.IntVal {
				return int(p.ContainerPort)
			}
		}
	}
	return 0
}

func getActionList(transCtx *CSSetTransformContext, actionScenario string) ([]*batchv1.Job, error) {
	var actionList []*batchv1.Job
	ml := client.MatchingLabels{
		model.AppInstanceLabelKey: transCtx.CSSet.Name,
		model.KBManagedByKey:      kindConsensusSet,
		jobScenarioLabel:          actionScenario,
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

func getActionName(parent string, generation, ordinal int, actionType string) string {
	return fmt.Sprintf("%s-%d-%d-%s", parent, generation, ordinal, actionType)
}

func getLeaderPodName(membersStatus []workloads.MemberStatus) string {
	for _, memberStatus := range membersStatus {
		if memberStatus.IsLeader {
			return memberStatus.PodName
		}
	}
	return ""
}

func getPodOrdinal(podName string) (int, error) {
	subMatches := podNameRegex.FindStringSubmatch(podName)
	if len(subMatches) < 3 {
		return 0, fmt.Errorf("wrong pod name: %s", podName)
	}
	return strconv.Atoi(subMatches[2])
}

// ordinal is the ordinal of pod which this action apply to
func createAction(dag *graph.DAG, csSet *workloads.StatefulReplicaSet, action *batchv1.Job) error {
	if err := intctrlutil.SetOwnership(csSet, action, model.GetScheme(), csSetFinalizerName); err != nil {
		return err
	}
	model.PrepareCreate(dag, action)
	return nil
}

func buildAction(csSet *workloads.StatefulReplicaSet, actionName, actionType, actionScenario string, leader, target string) *batchv1.Job {
	env := buildActionEnv(csSet, leader, target)
	template := buildActionPodTemplate(csSet, env, actionType)
	return builder.NewJobBuilder(csSet.Namespace, actionName).
		AddLabels(model.AppInstanceLabelKey, csSet.Name).
		AddLabels(model.KBManagedByKey, kindConsensusSet).
		AddLabels(jobScenarioLabel, actionScenario).
		AddLabels(jobTypeLabel, actionType).
		AddLabels(jobHandledLabel, jobHandledFalse).
		SetSuspend(false).
		SetPodTemplateSpec(*template).
		GetObject()
}

func buildActionPodTemplate(csSet *workloads.StatefulReplicaSet, env []corev1.EnvVar, actionType string) *corev1.PodTemplateSpec {
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

func buildActionEnv(csSet *workloads.StatefulReplicaSet, leader, target string) []corev1.EnvVar {
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
