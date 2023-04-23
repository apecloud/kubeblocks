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

package consensusset

import (
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
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
