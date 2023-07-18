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
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
func composeRolePriorityMap(rsm workloads.ReplicatedStateMachine) map[string]int {
	rolePriorityMap := make(map[string]int, 0)
	rolePriorityMap[""] = emptyPriority
	for _, role := range rsm.Spec.Roles {
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
	rsm workloads.ReplicatedStateMachine,
	pod *corev1.Pod, roleName string) error {
	ctx := reqCtx.Ctx
	roleMap := composeRoleMap(rsm)
	// role not defined in CR, ignore it
	roleName = strings.ToLower(roleName)

	// update pod role label
	patch := client.MergeFrom(pod.DeepCopy())
	role, ok := roleMap[roleName]
	switch ok {
	case true:
		pod.Labels[roleLabelKey] = role.Name
		pod.Labels[rsmAccessModeLabelKey] = string(role.AccessMode)
	case false:
		delete(pod.Labels, roleLabelKey)
		delete(pod.Labels, rsmAccessModeLabelKey)
	}
	return cli.Patch(ctx, pod, patch)
}

func composeRoleMap(rsm workloads.ReplicatedStateMachine) map[string]workloads.ReplicaRole {
	roleMap := make(map[string]workloads.ReplicaRole, 0)
	for _, role := range rsm.Spec.Roles {
		roleMap[strings.ToLower(role.Name)] = role
	}
	return roleMap
}

func setMembersStatus(rsm *workloads.ReplicatedStateMachine, pods []corev1.Pod) {
	// compose new status
	newMembersStatus := make([]workloads.MemberStatus, 0)
	roleMap := composeRoleMap(*rsm)
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
	oldMemberMap := make(map[string]*workloads.MemberStatus, len(rsm.Status.MembersStatus))
	for i, status := range rsm.Status.MembersStatus {
		oldMemberMap[status.PodName] = &rsm.Status.MembersStatus[i]
	}
	newMemberMap := make(map[string]*workloads.MemberStatus, len(newMembersStatus))
	for i, status := range newMembersStatus {
		newMemberMap[status.PodName] = &newMembersStatus[i]
	}
	oldMemberSet := sets.KeySet(oldMemberMap)
	newMemberSet := sets.KeySet(newMemberMap)
	memberToKeepSet := oldMemberSet.Difference(newMemberSet)
	// TODO(free6om): handle stale role in memberToKeepSet
	for podName := range memberToKeepSet {
		ordinal, _ := getPodOrdinal(podName)
		// members have left because of scale-in
		if ordinal >= int(*rsm.Spec.Replicas) {
			continue
		}
		newMembersStatus = append(newMembersStatus, *oldMemberMap[podName])
	}

	rolePriorityMap := composeRolePriorityMap(*rsm)
	sortMembersStatus(newMembersStatus, rolePriorityMap)
	rsm.Status.MembersStatus = newMembersStatus
}

func getRoleName(pod corev1.Pod) string {
	return strings.ToLower(pod.Labels[constant.RoleLabelKey])
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&appsv1.StatefulSetList{},
		&corev1.ServiceList{},
		&corev1.ConfigMapList{},
	}
}

func deletionKinds() []client.ObjectList {
	kinds := ownedKinds()
	kinds = append(kinds, &corev1.PersistentVolumeClaimList{}, &batchv1.JobList{})
	return kinds
}

func getPodsOfStatefulSet(ctx context.Context, cli roclient.ReadonlyClient, stsObj *appsv1.StatefulSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	selector, err := metav1.LabelSelectorAsMap(stsObj.Spec.Selector)
	if err != nil {
		return nil, err
	}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: stsObj.Namespace},
		client.MatchingLabels(selector)); err != nil {
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

func getHeadlessSvcName(rsm workloads.ReplicatedStateMachine) string {
	return strings.Join([]string{rsm.Name, "headless"}, "-")
}

func findSvcPort(rsm workloads.ReplicatedStateMachine) int {
	if rsm.Spec.Service == nil {
		return 0
	}
	port := rsm.Spec.Service.Ports[0]
	for _, c := range rsm.Spec.Template.Spec.Containers {
		for _, p := range c.Ports {
			if port.TargetPort.Type == intstr.String && p.Name == port.TargetPort.StrVal ||
				port.TargetPort.Type == intstr.Int && p.ContainerPort == port.TargetPort.IntVal {
				return int(p.ContainerPort)
			}
		}
	}
	return 0
}

func getActionList(transCtx *rsmTransformContext, actionScenario string) ([]*batchv1.Job, error) {
	labels := getLabels(transCtx.rsm)
	labels[jobScenarioLabel] = actionScenario
	labels[jobHandledLabel] = jobHandledFalse
	ml := client.MatchingLabels(labels)

	var actionList []*batchv1.Job
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

// TODO(free6om): remove all printActionList when all testes pass
func printActionList(logger logr.Logger, actionList []*batchv1.Job) {
	var actionNameList []string
	for _, action := range actionList {
		actionNameList = append(actionNameList, fmt.Sprintf("%s-%v", action.Name, *action.Spec.Suspend))
	}
	logger.Info(fmt.Sprintf("action list: %v\n", actionNameList))
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
func createAction(dag *graph.DAG, cli model.GraphClient, rsm *workloads.ReplicatedStateMachine, action *batchv1.Job) error {
	if err := setOwnership(rsm, action, model.GetScheme(), getFinalizer(action)); err != nil {
		return err
	}
	cli.Create(dag, action)
	return nil
}

func buildAction(rsm *workloads.ReplicatedStateMachine, actionName, actionType, actionScenario string, leader, target string) *batchv1.Job {
	env := buildActionEnv(rsm, leader, target)
	template := buildActionPodTemplate(rsm, env, actionType)
	labels := getLabels(rsm)
	return builder.NewJobBuilder(rsm.Namespace, actionName).
		AddLabelsInMap(labels).
		AddLabels(jobScenarioLabel, actionScenario).
		AddLabels(jobTypeLabel, actionType).
		AddLabels(jobHandledLabel, jobHandledFalse).
		SetSuspend(false).
		SetPodTemplateSpec(*template).
		GetObject()
}

func buildActionPodTemplate(rsm *workloads.ReplicatedStateMachine, env []corev1.EnvVar, actionType string) *corev1.PodTemplateSpec {
	credential := rsm.Spec.Credential
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
	reconfiguration := rsm.Spec.MembershipReconfiguration
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

func buildActionEnv(rsm *workloads.ReplicatedStateMachine, leader, target string) []corev1.EnvVar {
	svcName := getHeadlessSvcName(*rsm)
	leaderHost := fmt.Sprintf("%s.%s", leader, svcName)
	targetHost := fmt.Sprintf("%s.%s", target, svcName)
	svcPort := findSvcPort(*rsm)
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

func doActionCleanup(dag *graph.DAG, graphCli model.GraphClient, action *batchv1.Job) {
	actionOld := action.DeepCopy()
	actionNew := actionOld.DeepCopy()
	actionNew.Labels[jobHandledLabel] = jobHandledTrue
	graphCli.Update(dag, actionOld, actionNew)
}

func emitEvent(transCtx *rsmTransformContext, action *batchv1.Job) {
	switch {
	case action.Status.Succeeded > 0:
		emitActionSucceedEvent(transCtx, action.Labels[jobTypeLabel], action.Name)
	case action.Status.Failed > 0:
		emitActionFailedEvent(transCtx, action.Labels[jobTypeLabel], action.Name)
	}
}

func emitActionSucceedEvent(transCtx *rsmTransformContext, actionType, actionName string) {
	message := fmt.Sprintf("%s succeed, job name: %s", actionType, actionName)
	emitActionEvent(transCtx, corev1.EventTypeNormal, actionType, message)
}

func emitActionFailedEvent(transCtx *rsmTransformContext, actionType, actionName string) {
	message := fmt.Sprintf("%s failed, job name: %s", actionType, actionName)
	emitActionEvent(transCtx, corev1.EventTypeWarning, actionType, message)
}

func emitAbnormalEvent(transCtx *rsmTransformContext, actionType, actionName string, err error) {
	message := fmt.Sprintf("%s, job name: %s", err.Error(), actionName)
	emitActionEvent(transCtx, corev1.EventTypeWarning, actionType, message)
}

func emitActionEvent(transCtx *rsmTransformContext, eventType, reason, message string) {
	transCtx.EventRecorder.Event(transCtx.rsm, eventType, strings.ToUpper(reason), message)
}

func getFinalizer(obj client.Object) string {
	if _, ok := obj.(*workloads.ReplicatedStateMachine); ok {
		return rsmFinalizerName
	}
	if viper.GetBool(FeatureGateRSMCompatibilityMode) {
		return constant.DBClusterFinalizerName
	}
	return rsmFinalizerName
}

func getLabels(rsm *workloads.ReplicatedStateMachine) map[string]string {
	if viper.GetBool(FeatureGateRSMCompatibilityMode) {
		labels := make(map[string]string, 0)
		keys := []string{
			constant.AppManagedByLabelKey,
			constant.AppNameLabelKey,
			constant.AppComponentLabelKey,
			constant.AppInstanceLabelKey,
			constant.KBAppComponentLabelKey,
		}
		for _, key := range keys {
			if value, ok := rsm.Labels[key]; ok {
				labels[key] = value
			}
		}
		return labels
	}
	return map[string]string{
		constant.AppInstanceLabelKey: rsm.Name,
		constant.KBManagedByKey:      kindReplicatedStateMachine,
	}
}

func getSvcSelector(leader *workloads.ReplicaRole) (string, string) {
	if leader == nil {
		return "", ""
	}
	if viper.GetBool(FeatureGateRSMCompatibilityMode) {
		return constant.RoleLabelKey, leader.Name
	}
	return rsmAccessModeLabelKey, string(leader.AccessMode)
}

func setOwnership(owner, obj client.Object, scheme *runtime.Scheme, finalizer string) error {
	// if viper.GetBool(FeatureGateRSMCompatibilityMode) {
	//	return CopyOwnership(owner, obj, scheme, finalizer)
	// }
	return intctrlutil.SetOwnership(owner, obj, scheme, finalizer)
}

// CopyOwnership copies owner ref fields of 'owner' to 'obj'
// and calls controllerutil.AddFinalizer if not exists.
func CopyOwnership(owner, obj client.Object, scheme *runtime.Scheme, finalizer string) error {
	// Returns true if a and b point to the same object.
	referSameObject := func(a, b metav1.OwnerReference) bool {
		aGV, err := schema.ParseGroupVersion(a.APIVersion)
		if err != nil {
			return false
		}
		bGV, err := schema.ParseGroupVersion(b.APIVersion)
		if err != nil {
			return false
		}
		return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
	}
	// indexOwnerRef returns the index of the owner reference in the slice if found, or -1.
	indexOwnerRef := func(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) int {
		for index, r := range ownerReferences {
			if referSameObject(r, ref) {
				return index
			}
		}
		return -1
	}
	upsertOwnerRef := func(ref metav1.OwnerReference, object metav1.Object) {
		owners := object.GetOwnerReferences()
		if idx := indexOwnerRef(owners, ref); idx == -1 {
			owners = append(owners, ref)
		} else {
			owners[idx] = ref
		}
		object.SetOwnerReferences(owners)
	}

	ownerRefs := owner.GetOwnerReferences()
	for _, ref := range ownerRefs {
		if ref.Controller == nil || !*ref.Controller {
			continue
		}
		// Return early with an error if the object is already controlled.
		if existing := metav1.GetControllerOf(obj); existing != nil && !referSameObject(*existing, ref) {
			return &controllerutil.AlreadyOwnedError{
				Object: obj,
				Owner:  *existing,
			}
		}

		// Update owner references and return.
		upsertOwnerRef(ref, obj)
	}

	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		// pvc objects do not need to add finalizer
		_, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			if !controllerutil.AddFinalizer(obj, finalizer) {
				return intctrlutil.ErrFailedToAddFinalizer
			}
		}
	}
	return nil
}
