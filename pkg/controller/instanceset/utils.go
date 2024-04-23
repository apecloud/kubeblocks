/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package instanceset

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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

// ComposeRolePriorityMap generates a priority map based on roles.
func ComposeRolePriorityMap(roles []workloads.ReplicaRole) map[string]int {
	rolePriorityMap := make(map[string]int)
	rolePriorityMap[""] = emptyPriority
	for _, role := range roles {
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

// SortPods sorts pods by their role priority
// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name > follower2.Name
// reverse it if reverse==true
func SortPods(pods []corev1.Pod, rolePriorityMap map[string]int, reverse bool) {
	getRolePriorityFunc := func(i int) int {
		role := GetRoleName(pods[i])
		return rolePriorityMap[role]
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return ParseParentNameAndOrdinal(pods[i].GetName())
	}
	baseSort(pods, getNameNOrdinalFunc, getRolePriorityFunc, reverse)
}

// GetRoleName gets role name of pod 'pod'
func GetRoleName(pod corev1.Pod) string {
	return strings.ToLower(pod.Labels[constant.RoleLabelKey])
}

// IsInstanceSetReady gives rsm level 'ready' state:
// 1. all replicas exist
// 2. all members have role set
func IsInstanceSetReady(rsm *workloads.InstanceSet) bool {
	if rsm == nil {
		return false
	}
	// check whether the rsm cluster has been initialized
	if rsm.Status.ReadyInitReplicas != rsm.Status.InitReplicas {
		return false
	}
	// check whether latest spec has been sent to the underlying workload(sts)
	if rsm.Status.ObservedGeneration != rsm.Generation || rsm.Status.CurrentGeneration != rsm.Generation {
		return false
	}
	// check whether the underlying workload(sts) is ready
	if rsm.Spec.Replicas == nil {
		return false
	}
	replicas := *rsm.Spec.Replicas
	if rsm.Status.Replicas != replicas ||
		rsm.Status.ReadyReplicas != replicas ||
		rsm.Status.UpdatedReplicas != replicas {
		return false
	}
	// check availableReplicas only if minReadySeconds is set
	if rsm.Spec.MinReadySeconds > 0 && rsm.Status.AvailableReplicas != replicas {
		return false
	}
	// check whether role probe has done
	if rsm.Spec.Roles == nil || rsm.Spec.RoleProbe == nil {
		return true
	}
	membersStatus := rsm.Status.MembersStatus
	if len(membersStatus) != int(*rsm.Spec.Replicas) {
		return false
	}
	hasLeader := false
	for _, status := range membersStatus {
		if status.ReadyWithoutPrimary {
			return true
		}
		if status.ReplicaRole != nil && status.ReplicaRole.IsLeader {
			hasLeader = true
			break
		}
	}
	return hasLeader
}

// AddAnnotationScope will add AnnotationScope defined by 'scope' to all keys in map 'annotations'.
func AddAnnotationScope(scope AnnotationScope, annotations map[string]string) map[string]string {
	if annotations == nil {
		return nil
	}
	scopedAnnotations := make(map[string]string, len(annotations))
	for k, v := range annotations {
		scopedAnnotations[fmt.Sprintf("%s%s", k, scope)] = v
	}
	return scopedAnnotations
}

// ParseAnnotationsOfScope parses all annotations with AnnotationScope defined by 'scope'.
// the AnnotationScope suffix of keys in result map will be trimmed.
func ParseAnnotationsOfScope(scope AnnotationScope, scopedAnnotations map[string]string) map[string]string {
	if scopedAnnotations == nil {
		return nil
	}

	annotations := make(map[string]string, 0)
	if scope == RootScope {
		for k, v := range scopedAnnotations {
			if strings.HasSuffix(k, scopeSuffix) {
				continue
			}
			annotations[k] = v
		}
		return annotations
	}

	for k, v := range scopedAnnotations {
		if strings.HasSuffix(k, string(scope)) {
			annotations[strings.TrimSuffix(k, string(scope))] = v
		}
	}
	return annotations
}

func GetEnvConfigMapName(rsmName string) string {
	return fmt.Sprintf("%s-its-env", rsmName)
}

func composeRoleMap(rsm workloads.InstanceSet) map[string]workloads.ReplicaRole {
	roleMap := make(map[string]workloads.ReplicaRole)
	for _, role := range rsm.Spec.Roles {
		roleMap[strings.ToLower(role.Name)] = role
	}
	return roleMap
}

func mergeMap[K comparable, V any](src, dst *map[K]V) {
	if len(*src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[K]V)
	}
	for k, v := range *src {
		(*dst)[k] = v
	}
}

func mergeList[E any](src, dst *[]E, f func(E) func(E) bool) {
	if len(*src) == 0 {
		return
	}
	for i := range *src {
		item := (*src)[i]
		index := slices.IndexFunc(*dst, f(item))
		if index >= 0 {
			(*dst)[index] = item
		} else {
			*dst = append(*dst, item)
		}
	}
}

func getMatchLabels(name string) map[string]string {
	return map[string]string{
		WorkloadsManagedByLabelKey: workloads.Kind,
		WorkloadsInstanceLabelKey:  name,
	}
}

func getSvcSelector(its *workloads.InstanceSet, headless bool) map[string]string {
	selectors := make(map[string]string)

	if !headless {
		for _, role := range its.Spec.Roles {
			if role.IsLeader && len(role.Name) > 0 {
				selectors[constant.RoleLabelKey] = role.Name
				break
			}
		}
	}

	for k, v := range its.Spec.Selector.MatchLabels {
		selectors[k] = v
	}
	return selectors
}
