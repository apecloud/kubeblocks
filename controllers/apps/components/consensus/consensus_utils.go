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
	"errors"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
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

// generateRestartPodPlan generates update plan to restart pods based on UpdateStrategy
func generateRestartPodPlan(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet, pods []corev1.Pod,
	consensusSpec *appsv1alpha1.ConsensusSetSpec, podsToDelete []*corev1.Pod) *util.Plan {
	restartPod := func(obj interface{}) (bool, error) {
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
		podsToDelete = append(podsToDelete, &pod)

		return true, nil
	}
	return generateConsensusUpdatePlanLow(ctx, cli, stsObj, pods, consensusSpec, restartPod)
}

// generateConsensusUpdatePlanLow generates Update plan based on UpdateStrategy
func generateConsensusUpdatePlanLow(ctx context.Context, cli client.Client, stsObj *appsv1.StatefulSet, pods []corev1.Pod,
	consensusSpec *appsv1alpha1.ConsensusSetSpec, restartPod func(obj any) (bool, error)) *util.Plan {
	plan := &util.Plan{}
	plan.Start = &util.Step{}
	plan.WalkFunc = restartPod

	rolePriorityMap := ComposeRolePriorityMap(consensusSpec)
	util.SortPods(pods, rolePriorityMap, constant.RoleLabelKey)

	// generate plan by UpdateStrategy
	switch consensusSpec.UpdateStrategy {
	case appsv1alpha1.SerialStrategy:
		generateConsensusSerialPlan(plan, pods, rolePriorityMap)
	case appsv1alpha1.ParallelStrategy:
		generateConsensusParallelPlan(plan, pods, rolePriorityMap)
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
func ComposeRolePriorityMap(consensusSpec *appsv1alpha1.ConsensusSetSpec) map[string]int {
	if consensusSpec == nil {
		consensusSpec = appsv1alpha1.NewConsensusSetSpec()
	}
	rolePriorityMap := make(map[string]int, 0)
	rolePriorityMap[""] = emptyPriority
	rolePriorityMap[consensusSpec.Leader.Name] = leaderPriority
	if consensusSpec.Learner != nil {
		rolePriorityMap[consensusSpec.Learner.Name] = learnerPriority
	}
	for _, follower := range consensusSpec.Followers {
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
	if componentDef == nil {
		return nil
	}
	return updateConsensusSetRoleLabel(cli, reqCtx, componentDef.ConsensusSpec, pod, role)
}

func updateConsensusSetRoleLabel(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	consensusSpec *appsv1alpha1.ConsensusSetSpec,
	pod *corev1.Pod, role string) error {
	ctx := reqCtx.Ctx
	roleMap := composeConsensusRoleMap(consensusSpec)
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

func composeConsensusRoleMap(consensusSpec *appsv1alpha1.ConsensusSetSpec) map[string]consensusMemberExt {
	roleMap := make(map[string]consensusMemberExt, 0)
	putConsensusMemberExt(roleMap,
		consensusSpec.Leader.Name,
		roleLeader,
		consensusSpec.Leader.AccessMode)

	for _, follower := range consensusSpec.Followers {
		putConsensusMemberExt(roleMap,
			follower.Name,
			roleFollower,
			follower.AccessMode)
	}

	if consensusSpec.Learner != nil {
		putConsensusMemberExt(roleMap,
			consensusSpec.Learner.Name,
			roleLearner,
			consensusSpec.Learner.AccessMode)
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
		consensusSetStatus.Leader.Pod = constant.ComponentStatusDefaultPodName
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
	consensusSpec *appsv1alpha1.ConsensusSetSpec,
	pods []corev1.Pod) {
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			continue
		}

		role := pod.Labels[constant.RoleLabelKey]
		_ = setConsensusSetStatusRole(consensusSetStatus, consensusSpec, role, pod.Name)
	}
}

func setConsensusSetStatusRole(
	consensusSetStatus *appsv1alpha1.ConsensusSetStatus,
	consensusSpec *appsv1alpha1.ConsensusSetSpec,
	role, podName string) bool {
	// mapping role label to consensus member
	roleMap := composeConsensusRoleMap(consensusSpec)
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
	consensusSpec *appsv1alpha1.ConsensusSetSpec,
	componentName string,
	compDefName string,
	pods []corev1.Pod,
	vertexes *[]graph.Vertex) error {
	leader, followers := composeRoleEnv(consensusSpec, pods)
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.GetName(),
		constant.KBAppComponentLabelKey: componentName,
		constant.AppConfigTypeLabelKey:  "kubeblocks-env",
	}

	configList := &corev1.ConfigMapList{}
	if err := cli.List(ctx, configList, ml); err != nil {
		return err
	}

	for idx := range configList.Items {
		config := configList.Items[idx]
		config.Data["KB_"+strings.ToUpper(compDefName)+"_LEADER"] = leader
		config.Data["KB_"+strings.ToUpper(compDefName)+"_FOLLOWERS"] = followers
		*vertexes = append(*vertexes, &ictrltypes.LifecycleVertex{
			Obj:    &config,
			Action: ictrltypes.ActionUpdatePtr(),
		})
	}

	// patch pods' annotations
	for idx := range pods {
		pod := pods[idx]
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[constant.LeaderAnnotationKey] = leader
		*vertexes = append(*vertexes, &ictrltypes.LifecycleVertex{
			Obj:    &pod,
			Action: ictrltypes.ActionUpdatePtr(),
		})
	}

	return nil
}

func composeRoleEnv(consensusSpec *appsv1alpha1.ConsensusSetSpec, pods []corev1.Pod) (leader, followers string) {
	leader, followers = "", ""
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			continue
		}
		role := pod.Labels[constant.RoleLabelKey]
		// mapping role label to consensus member
		roleMap := composeConsensusRoleMap(consensusSpec)
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
