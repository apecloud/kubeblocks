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

package consensus

import (
	"context"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
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
func generateConsensusParallelPlan(plan *util.Plan, pods []corev1.Pod, rolePriorityMap map[string]int) {
	start := plan.Start
	for _, pod := range pods {
		nextStep := &util.Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
	}
}

// unknown -> empty -> learner -> followers(none->readonly->readwrite) -> leader
func generateConsensusSerialPlan(plan *util.Plan, pods []corev1.Pod, rolePriorityMap map[string]int) {
	start := plan.Start
	for _, pod := range pods {
		nextStep := &util.Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
		start = nextStep
	}
}

// ComposeRolePriorityMap generates a priority map based on roles.
func ComposeRolePriorityMap(componentDef *appsv1alpha1.ClusterComponentDefinition) map[string]int {
	if componentDef.ConsensusSpec == nil {
		componentDef.ConsensusSpec = appsv1alpha1.NewConsensusSetSpec()
	}
	rolePriorityMap := make(map[string]int, 0)
	rolePriorityMap[""] = emptyPriority
	rolePriorityMap[componentDef.ConsensusSpec.Leader.Name] = leaderPriority
	if componentDef.ConsensusSpec.Learner != nil {
		rolePriorityMap[componentDef.ConsensusSpec.Learner.Name] = learnerPriority
	}
	for _, follower := range componentDef.ConsensusSpec.Followers {
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
