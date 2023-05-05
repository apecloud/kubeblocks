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
	"errors"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TODO: dedup, copy from controllers/apps/component/consensusset/consensus_set_utils.go
type consensusRole string

type consensusMemberExt struct {
	name          string
	consensusRole consensusRole
	accessMode    workloads.AccessMode
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
func generateConsensusUpdatePlan(stsObj *appsv1.StatefulSet, pods []corev1.Pod, set workloads.ConsensusSet, dag *graph.DAG) *Plan {
	plan := &Plan{}
	plan.Start = &Step{}
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
		root, err := model.FindRootVertex(dag)
		if err != nil {
			return false, err
		}
		vertex := &model.ObjectVertex{Obj: &pod, Action: model.ActionPtr(model.DELETE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)

		return true, nil
	}

	rolePriorityMap := ComposeRolePriorityMap(set)
	SortPods(pods, rolePriorityMap)

	// generate plan by UpdateStrategy
	switch set.Spec.UpdateStrategy {
	case workloads.SerialUpdateStrategy:
		generateConsensusSerialPlan(plan, pods)
	case workloads.ParallelUpdateStrategy:
		generateConsensusParallelPlan(plan, pods)
	case workloads.BestEffortParallelUpdateStrategy:
		generateConsensusBestEffortParallelPlan(plan, pods, rolePriorityMap)
	}

	return plan
}

// unknown & empty & learner & 1/2 followers -> 1/2 followers -> leader
func generateConsensusBestEffortParallelPlan(plan *Plan, pods []corev1.Pod, rolePriorityMap map[string]int) {
	start := plan.Start
	// append unknown, empty and learner
	index := 0
	for _, pod := range pods {
		role := pod.Labels[constant.RoleLabelKey]
		if rolePriorityMap[role] <= learnerPriority {
			nextStep := &Step{}
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
		nextStep := &Step{}
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
		nextStep := &Step{}
		nextStep.Obj = podList[i]
		start.NextSteps = append(start.NextSteps, nextStep)
	}

	if len(start.NextSteps) > 0 {
		start = start.NextSteps[0]
	}
	// append leader
	podList = podList[end:]
	for _, pod := range podList {
		nextStep := &Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
	}
}

// unknown & empty & leader & followers & learner
func generateConsensusParallelPlan(plan *Plan, pods []corev1.Pod) {
	start := plan.Start
	for _, pod := range pods {
		nextStep := &Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
	}
}

// unknown -> empty -> learner -> followers(none->readonly->readwrite) -> leader
func generateConsensusSerialPlan(plan *Plan, pods []corev1.Pod) {
	start := plan.Start
	for _, pod := range pods {
		nextStep := &Step{}
		nextStep.Obj = pod
		start.NextSteps = append(start.NextSteps, nextStep)
		start = nextStep
	}
}

// ComposeRolePriorityMap generates a priority map based on roles.
func ComposeRolePriorityMap(set workloads.ConsensusSet) map[string]int {
	rolePriorityMap := make(map[string]int, 0)
	rolePriorityMap[""] = emptyPriority
	rolePriorityMap[set.Spec.Leader.Name] = leaderPriority
	if set.Spec.Learner != nil {
		rolePriorityMap[set.Spec.Learner.Name] = learnerPriority
	}
	for _, follower := range set.Spec.Followers {
		switch follower.AccessMode {
		case workloads.NoneMode:
			rolePriorityMap[follower.Name] = followerNonePriority
		case workloads.ReadonlyMode:
			rolePriorityMap[follower.Name] = followerReadonlyPriority
		case workloads.ReadWriteMode:
			rolePriorityMap[follower.Name] = followerReadWritePriority
		}
	}

	return rolePriorityMap
}

// updatePodRoleLabel updates pod role label when internal container role changed
func updatePodRoleLabel(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	set workloads.ConsensusSet,
	pod *corev1.Pod, role string) error {
	ctx := reqCtx.Ctx
	roleMap := composeConsensusRoleMap(set)
	// role not defined in CR, ignore it
	if _, ok := roleMap[role]; !ok {
		return nil
	}

	// update pod role label
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[model.RoleLabelKey] = role
	pod.Labels[model.ConsensusSetAccessModeLabelKey] = string(roleMap[role].accessMode)
	return cli.Patch(ctx, pod, patch)
}

func putConsensusMemberExt(roleMap map[string]consensusMemberExt, name string, role consensusRole, accessMode workloads.AccessMode) {
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

func composeConsensusRoleMap(set workloads.ConsensusSet) map[string]consensusMemberExt {
	roleMap := make(map[string]consensusMemberExt, 0)
	putConsensusMemberExt(roleMap,
		set.Spec.Leader.Name,
		roleLeader,
		set.Spec.Leader.AccessMode)

	for _, follower := range set.Spec.Followers {
		putConsensusMemberExt(roleMap,
			follower.Name,
			roleFollower,
			follower.AccessMode)
	}

	if set.Spec.Learner != nil {
		putConsensusMemberExt(roleMap,
			set.Spec.Learner.Name,
			roleLearner,
			set.Spec.Learner.AccessMode)
	}

	return roleMap
}

func setConsensusSetStatusLeader(set *workloads.ConsensusSet, memberExt consensusMemberExt) bool {
	if set.Status.Leader.PodName == memberExt.podName {
		return false
	}
	set.Status.Leader.PodName = memberExt.podName
	set.Status.Leader.AccessMode = memberExt.accessMode
	set.Status.Leader.RoleName = memberExt.name
	return true
}

func setConsensusSetStatusFollower(set *workloads.ConsensusSet, memberExt consensusMemberExt) bool {
	for _, member := range set.Status.Followers {
		if member.PodName == memberExt.podName {
			return false
		}
	}
	member := workloads.ConsensusMemberStatus{
		PodName:    memberExt.podName,
		AccessMode: memberExt.accessMode,
		RoleName:   memberExt.name,
	}
	set.Status.Followers = append(set.Status.Followers, member)
	sort.SliceStable(set.Status.Followers, func(i, j int) bool {
		fi := set.Status.Followers[i]
		fj := set.Status.Followers[j]
		return strings.Compare(fi.PodName, fj.PodName) < 0
	})
	return true
}

func setConsensusSetStatusLearner(set *workloads.ConsensusSet, memberExt consensusMemberExt) bool {
	if set.Status.Learner == nil {
		set.Status.Learner = &workloads.ConsensusMemberStatus{}
	}
	if set.Status.Learner.PodName == memberExt.podName {
		return false
	}
	set.Status.Learner.PodName = memberExt.podName
	set.Status.Learner.AccessMode = memberExt.accessMode
	set.Status.Learner.RoleName = memberExt.name
	return true
}

func resetConsensusSetStatusRole(set *workloads.ConsensusSet, podName string) {
	// reset leader
	if set.Status.Leader.PodName == podName {
		set.Status.Leader.PodName = DefaultPodName
		set.Status.Leader.AccessMode = workloads.NoneMode
		set.Status.Leader.RoleName = ""
	}

	// reset follower
	for index, member := range set.Status.Followers {
		if member.PodName == podName {
			set.Status.Followers = append(set.Status.Followers[:index], set.Status.Followers[index+1:]...)
		}
	}

	// reset learner
	if set.Status.Learner != nil && set.Status.Learner.PodName == podName {
		set.Status.Learner = nil
	}
}

func setConsensusSetStatusRoles(set *workloads.ConsensusSet, pods []corev1.Pod) {
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			continue
		}

		role := pod.Labels[constant.RoleLabelKey]
		_ = setConsensusSetStatusRole(set, role, pod.Name)
	}
}

func setConsensusSetStatusRole(set *workloads.ConsensusSet, role, podName string) bool {
	// mapping role label to consensus member
	roleMap := composeConsensusRoleMap(*set)
	memberExt, ok := roleMap[role]
	if !ok {
		return false
	}
	memberExt.podName = podName
	resetConsensusSetStatusRole(set, memberExt.podName)
	// update cluster.status
	needUpdate := false
	switch memberExt.consensusRole {
	case roleLeader:
		needUpdate = setConsensusSetStatusLeader(set, memberExt)
	case roleFollower:
		needUpdate = setConsensusSetStatusFollower(set, memberExt)
	case roleLearner:
		needUpdate = setConsensusSetStatusLearner(set, memberExt)
	}
	return needUpdate
}
