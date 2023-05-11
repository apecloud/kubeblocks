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
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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

// sortPods sorts pods by their role priority
// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name < follower2.Name
// reverse it if reverse==true
func sortPods(pods []corev1.Pod, rolePriorityMap map[string]int, reverse bool) {
	sort.SliceStable(pods, func(i, j int) bool {
		roleI := getRoleName(pods[i])
		roleJ := getRoleName(pods[j])
		if reverse {
			roleI, roleJ = roleJ, roleI
		}

		if rolePriorityMap[roleI] == rolePriorityMap[roleJ] {
			_, ordinal1 := intctrlutil.GetParentNameAndOrdinal(&pods[i])
			_, ordinal2 := intctrlutil.GetParentNameAndOrdinal(&pods[j])
			return ordinal1 < ordinal2
		}

		return rolePriorityMap[roleI] < rolePriorityMap[roleJ]
	})
}

// composeRolePriorityMap generates a priority map based on roles.
func composeRolePriorityMap(set workloads.ConsensusSet) map[string]int {
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
	set workloads.ConsensusSet,
	pod *corev1.Pod, roleName string) error {
	ctx := reqCtx.Ctx
	roleMap := composeRoleMap(set)
	// role not defined in CR, ignore it
	roleName = strings.ToLower(roleName)
	role, ok := roleMap[roleName]
	if !ok {
		return nil
	}

	// update pod role label
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[model.RoleLabelKey] = role.Name
	pod.Labels[model.ConsensusSetAccessModeLabelKey] = string(role.AccessMode)
	return cli.Patch(ctx, pod, patch)
}

func composeRoleMap(set workloads.ConsensusSet) map[string]workloads.ConsensusRole {
	roleMap := make(map[string]workloads.ConsensusRole, 0)
	for _, role := range set.Spec.Roles {
		roleMap[strings.ToLower(role.Name)] = role
	}
	return roleMap
}

func setMembersStatus(set *workloads.ConsensusSet, pods []corev1.Pod) {
	rolePriorityMap := composeRolePriorityMap(*set)
	sortPods(pods, rolePriorityMap, true)
	roleMap := composeRoleMap(*set)

	newMembersStatus := make([]workloads.ConsensusMemberStatus, 0)
	for _, pod := range pods {
		if !intctrlutil.PodIsReadyWithLabel(pod) {
			continue
		}
		roleName := getRoleName(pod)
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		memberStatus := workloads.ConsensusMemberStatus{
			PodName:       pod.Name,
			ConsensusRole: role,
		}
		newMembersStatus = append(newMembersStatus, memberStatus)
	}
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
