/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consensus

import (
	"context"
	"sort"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch;delete

type consensusRole string

type consensusMemberExt struct {
	name          string
	consensusRole consensusRole
	accessMode    appsv1alpha1.AccessMode
	podName       string
}

const (
	roleLeader   consensusRole = "Leader"
	roleFollower consensusRole = "Follower"
	roleLearner  consensusRole = "Learner"
)

const (
	leaderPriority            = 1 << 5
	followerReadWritePriority = 1 << 4
	followerReadonlyPriority  = 1 << 3
	followerNonePriority      = 1 << 2
	learnerPriority           = 1 << 1
	emptyPriority             = 1 << 0
	// unknownPriority           = 0
)

// SortPods sorts pods by their role priority
func SortPods(pods []corev1.Pod, rolePriorityMap map[string]int) {
	// make a Serial pod list,
	// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name < follower2.Name
	sort.SliceStable(pods, func(i, j int) bool {
		roleI := pods[i].Labels[constant.RoleLabelKey]
		roleJ := pods[j].Labels[constant.RoleLabelKey]

		if rolePriorityMap[roleI] == rolePriorityMap[roleJ] {
			_, ordinal1 := intctrlutil.GetParentNameAndOrdinal(&pods[i])
			_, ordinal2 := intctrlutil.GetParentNameAndOrdinal(&pods[j])
			return ordinal1 < ordinal2
		}

		return rolePriorityMap[roleI] < rolePriorityMap[roleJ]
	})
}

// generateConsensusUpdatePlan generates Update plan based on UpdateStrategy
func generateConsensusUpdatePlan(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet, pods []corev1.Pod,
	component appsv1alpha1.ClusterComponentDefinition) *util.Plan {
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

	rolePriorityMap := ComposeRolePriorityMap(component)
	SortPods(pods, rolePriorityMap)

	// generate plan by UpdateStrategy
	switch component.ConsensusSpec.UpdateStrategy {
	case appsv1alpha1.SerialStrategy:
		generateConsensusSerialPlan(plan, pods)
	case appsv1alpha1.ParallelStrategy:
		generateConsensusParallelPlan(plan, pods)
	case appsv1alpha1.BestEffortParallelStrategy:
		generateConsensusBestEffortParallelPlan(plan, pods, rolePriorityMap)
	}

	return plan
}

// unknown & empty & learner & 1/2 followers -> 1/2 followers -> leader
func generateConsensusBestEffortParallelPlan(plan *util.Plan, pods []corev1.Pod, rolePriorityMap map[string]int) {
	start := plan.Start
	// append unknown, empty and learner
	index := 0
	for _, pod := range pods {
		role := pod.Labels[constant.RoleLabelKey]
		if rolePriorityMap[role] <= learnerPriority {
			nextStep := &util.Step{}
			nextStep.Obj = pod
			start.NextSteps = append(start.NextSteps, nextStep)
			index++
		}
	}
	if len(start.NextSteps) > 0 {
		start = start.NextSteps[0]
	}
	// append 1/2 followers
	podList := pods[index:]
	followerCount := 0
	for _, pod := range podList {
		if rolePriorityMap[pod.Labels[constant.RoleLabelKey]] < leaderPriority {
			followerCount++
		}
	}
	end := followerCount / 2
	for i := 0; i < end; i++ {
		nextStep := &util.Step{}
		nextStep.Obj = podList[i]
		start.NextSteps = append(start.NextSteps, nextStep)
	}

	if len(start.NextSteps) > 0 {
		start = start.NextSteps[0]
	}
	// append the other 1/2 followers
	podList = podList[end:]
	end = followerCount - end
	for i := 0; i < end; i++ {
		nextStep := &util.Step{}
		nextStep.Obj = podList[i]
		start.NextSteps = append(start.NextSteps, nextStep)
	}

	if len(start.NextSteps) > 0 {
		start = start.NextSteps[0]
	}
	// append leader
	podList = podList[end:]
	for _, pod := range podList {
		nextStep := &util.Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
	}
}

// unknown & empty & leader & followers & learner
func generateConsensusParallelPlan(plan *util.Plan, pods []corev1.Pod) {
	start := plan.Start
	for _, pod := range pods {
		nextStep := &util.Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
	}
}

// unknown -> empty -> learner -> followers(none->readonly->readwrite) -> leader
func generateConsensusSerialPlan(plan *util.Plan, pods []corev1.Pod) {
	start := plan.Start
	for _, pod := range pods {
		nextStep := &util.Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
		start = nextStep
	}
}

// ComposeRolePriorityMap generates a priority map based on roles.
func ComposeRolePriorityMap(component appsv1alpha1.ClusterComponentDefinition) map[string]int {
	if component.ConsensusSpec == nil {
		component.ConsensusSpec = &appsv1alpha1.ConsensusSetSpec{Leader: appsv1alpha1.DefaultLeader}
	}

	rolePriorityMap := make(map[string]int, 0)
	rolePriorityMap[""] = emptyPriority
	rolePriorityMap[component.ConsensusSpec.Leader.Name] = leaderPriority
	if component.ConsensusSpec.Learner != nil {
		rolePriorityMap[component.ConsensusSpec.Learner.Name] = learnerPriority
	}
	for _, follower := range component.ConsensusSpec.Followers {
		switch follower.AccessMode {
		case appsv1alpha1.None:
			rolePriorityMap[follower.Name] = followerNonePriority
		case appsv1alpha1.Readonly:
			rolePriorityMap[follower.Name] = followerReadonlyPriority
		case appsv1alpha1.ReadWrite:
			rolePriorityMap[follower.Name] = followerReadWritePriority
		}
	}

	return rolePriorityMap
}

// UpdateConsensusSetRoleLabel updates pod role label when internal container role changed
func UpdateConsensusSetRoleLabel(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	pod *corev1.Pod, role string) error {
	ctx := reqCtx.Ctx
	if componentDef == nil {
		return nil
	}
	roleMap := composeConsensusRoleMap(componentDef)
	// role not defined in CR, ignore it
	if _, ok := roleMap[role]; !ok {
		return nil
	}

	// update pod role label
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[constant.RoleLabelKey] = role
	pod.Labels[constant.ConsensusSetAccessModeLabelKey] = string(roleMap[role].accessMode)
	return cli.Patch(ctx, pod, patch)
}

func putConsensusMemberExt(roleMap map[string]consensusMemberExt, name string, role consensusRole, accessMode appsv1alpha1.AccessMode) {
	if roleMap == nil {
		return
	}

	if name == "" || role == "" || accessMode == "" {
		return
	}

	memberExt := consensusMemberExt{
		name:          name,
		consensusRole: role,
		accessMode:    accessMode,
	}

	roleMap[name] = memberExt
}

func composeConsensusRoleMap(componentDef *appsv1alpha1.ClusterComponentDefinition) map[string]consensusMemberExt {
	roleMap := make(map[string]consensusMemberExt, 0)
	putConsensusMemberExt(roleMap,
		componentDef.ConsensusSpec.Leader.Name,
		roleLeader,
		componentDef.ConsensusSpec.Leader.AccessMode)

	for _, follower := range componentDef.ConsensusSpec.Followers {
		putConsensusMemberExt(roleMap,
			follower.Name,
			roleFollower,
			follower.AccessMode)
	}

	if componentDef.ConsensusSpec.Learner != nil {
		putConsensusMemberExt(roleMap,
			componentDef.ConsensusSpec.Learner.Name,
			roleLearner,
			componentDef.ConsensusSpec.Learner.AccessMode)
	}

	return roleMap
}

func setConsensusSetStatusLeader(consensusSetStatus *appsv1alpha1.ConsensusSetStatus, memberExt consensusMemberExt) bool {
	if consensusSetStatus.Leader.Pod == memberExt.podName {
		return false
	}
	consensusSetStatus.Leader.Pod = memberExt.podName
	consensusSetStatus.Leader.AccessMode = memberExt.accessMode
	consensusSetStatus.Leader.Name = memberExt.name
	return true
}

func setConsensusSetStatusFollower(consensusSetStatus *appsv1alpha1.ConsensusSetStatus, memberExt consensusMemberExt) bool {
	for _, member := range consensusSetStatus.Followers {
		if member.Pod == memberExt.podName {
			return false
		}
	}
	member := appsv1alpha1.ConsensusMemberStatus{
		Pod:        memberExt.podName,
		AccessMode: memberExt.accessMode,
		Name:       memberExt.name,
	}
	consensusSetStatus.Followers = append(consensusSetStatus.Followers, member)
	sort.SliceStable(consensusSetStatus.Followers, func(i, j int) bool {
		fi := consensusSetStatus.Followers[i]
		fj := consensusSetStatus.Followers[j]
		return strings.Compare(fi.Pod, fj.Pod) < 0
	})
	return true
}

func setConsensusSetStatusLearner(consensusSetStatus *appsv1alpha1.ConsensusSetStatus, memberExt consensusMemberExt) bool {
	if consensusSetStatus.Learner == nil {
		consensusSetStatus.Learner = &appsv1alpha1.ConsensusMemberStatus{}
	}
	if consensusSetStatus.Learner.Pod == memberExt.podName {
		return false
	}
	consensusSetStatus.Learner.Pod = memberExt.podName
	consensusSetStatus.Learner.AccessMode = memberExt.accessMode
	consensusSetStatus.Learner.Name = memberExt.name
	return true
}

func resetConsensusSetStatusRole(consensusSetStatus *appsv1alpha1.ConsensusSetStatus, podName string) {
	// reset leader
	if consensusSetStatus.Leader.Pod == podName {
		consensusSetStatus.Leader.Pod = util.ComponentStatusDefaultPodName
		consensusSetStatus.Leader.AccessMode = appsv1alpha1.None
		consensusSetStatus.Leader.Name = ""
	}

	// reset follower
	for index, member := range consensusSetStatus.Followers {
		if member.Pod == podName {
			consensusSetStatus.Followers = append(consensusSetStatus.Followers[:index], consensusSetStatus.Followers[index+1:]...)
		}
	}

	// reset learner
	if consensusSetStatus.Learner != nil && consensusSetStatus.Learner.Pod == podName {
		consensusSetStatus.Learner = nil
	}
}

func setConsensusSetStatusRoles(
	consensusSetStatus *appsv1alpha1.ConsensusSetStatus,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	pods []corev1.Pod) {
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			continue
		}

		role := pod.Labels[constant.RoleLabelKey]
		_ = setConsensusSetStatusRole(consensusSetStatus, componentDef, role, pod.Name)
	}
}

func setConsensusSetStatusRole(
	consensusSetStatus *appsv1alpha1.ConsensusSetStatus,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	role, podName string) bool {
	// mapping role label to consensus member
	roleMap := composeConsensusRoleMap(componentDef)
	memberExt, ok := roleMap[role]
	if !ok {
		return false
	}
	memberExt.podName = podName
	resetConsensusSetStatusRole(consensusSetStatus, memberExt.podName)
	// update cluster.status
	needUpdate := false
	switch memberExt.consensusRole {
	case roleLeader:
		needUpdate = setConsensusSetStatusLeader(consensusSetStatus, memberExt)
	case roleFollower:
		needUpdate = setConsensusSetStatusFollower(consensusSetStatus, memberExt)
	case roleLearner:
		needUpdate = setConsensusSetStatusLearner(consensusSetStatus, memberExt)
	}
	return needUpdate
}

func updateConsensusRoleInfo(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentDef *appsv1alpha1.ClusterComponentDefinition,
	componentName string,
	pods []corev1.Pod) error {
	leader, followers := composeRoleEnv(componentDef, pods)

	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.GetName(),
		constant.KBAppComponentLabelKey: componentName,
		constant.AppConfigTypeLabelKey:  "kubeblocks-env",
	}

	configList := &corev1.ConfigMapList{}
	if err := cli.List(ctx, configList, ml); err != nil {
		return err
	}

	if len(configList.Items) > 0 {
		for _, config := range configList.Items {
			patch := client.MergeFrom(config.DeepCopy())
			config.Data["KB_"+strings.ToUpper(componentDef.Name)+"_LEADER"] = leader
			config.Data["KB_"+strings.ToUpper(componentDef.Name)+"_FOLLOWERS"] = followers
			if err := cli.Patch(ctx, &config, patch); err != nil {
				return err
			}
		}
	}
	// patch pods' annotations
	for _, pod := range pods {
		patch := client.MergeFrom(pod.DeepCopy())
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[constant.LeaderAnnotationKey] = leader
		if err := cli.Patch(ctx, &pod, patch); err != nil {
			return err
		}
	}

	return nil
}

func composeRoleEnv(componentDef *appsv1alpha1.ClusterComponentDefinition, pods []corev1.Pod) (leader, followers string) {
	leader, followers = "", ""
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			continue
		}
		role := pod.Labels[constant.RoleLabelKey]
		// mapping role label to consensus member
		roleMap := composeConsensusRoleMap(componentDef)
		memberExt, ok := roleMap[role]
		if !ok {
			continue
		}
		switch memberExt.consensusRole {
		case roleLeader:
			leader = pod.Name
		case roleFollower:
			if len(followers) > 0 {
				followers += ","
			}
			followers += pod.Name
		case roleLearner:
			// TODO: CT
		}
	}
	return
}
