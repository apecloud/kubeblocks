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
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/integer"

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
		role := getRoleName(&pods[i])
		return rolePriorityMap[role]
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return ParseParentNameAndOrdinal(pods[i].GetName())
	}
	baseSort(pods, getNameNOrdinalFunc, getRolePriorityFunc, reverse)
}

// getRoleName gets role name of pod 'pod'
func getRoleName(pod *corev1.Pod) string {
	return strings.ToLower(pod.Labels[constant.RoleLabelKey])
}

// IsInstancesReady gives Instance level 'ready' state when all instances are available
func IsInstancesReady(its *workloads.InstanceSet) bool {
	if its == nil {
		return false
	}
	// check whether the cluster has been initialized
	if its.Status.ReadyInitReplicas != its.Status.InitReplicas {
		return false
	}
	// check whether latest spec has been sent to the underlying workload
	if its.Status.ObservedGeneration != its.Generation {
		return false
	}
	// check whether the underlying workload is ready
	if its.Spec.Replicas == nil {
		return false
	}
	replicas := *its.Spec.Replicas
	if its.Status.Replicas != replicas ||
		its.Status.ReadyReplicas != replicas ||
		its.Status.UpdatedReplicas != replicas {
		return false
	}
	// check availableReplicas only if minReadySeconds is set
	if its.Spec.MinReadySeconds > 0 && its.Status.AvailableReplicas != replicas {
		return false
	}

	return true
}

// IsInstanceSetReady gives InstanceSet level 'ready' state:
// 1. all instances are available
// 2. and all members have role set (if they are role-ful)
func IsInstanceSetReady(its *workloads.InstanceSet) bool {
	instancesReady := IsInstancesReady(its)
	if !instancesReady {
		return false
	}

	// check whether role probe has done
	if its.Spec.Roles == nil || its.Spec.RoleProbe == nil {
		return true
	}
	membersStatus := its.Status.MembersStatus
	if len(membersStatus) != int(*its.Spec.Replicas) {
		return false
	}
	if its.Status.ReadyWithoutPrimary {
		return true
	}
	hasLeader := false
	for _, status := range membersStatus {
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

func GetEnvConfigMapName(itsName string) string {
	return fmt.Sprintf("%s-rsm-env", itsName)
}

func composeRoleMap(its workloads.InstanceSet) map[string]workloads.ReplicaRole {
	roleMap := make(map[string]workloads.ReplicaRole)
	for _, role := range its.Spec.Roles {
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

// GetPodNameSetFromInstanceSetCondition get the pod name sets from the InstanceSet conditions
func GetPodNameSetFromInstanceSetCondition(its *workloads.InstanceSet, conditionType workloads.ConditionType) map[string]sets.Empty {
	podSet := map[string]sets.Empty{}
	condition := meta.FindStatusCondition(its.Status.Conditions, string(conditionType))
	if condition != nil &&
		condition.Status == metav1.ConditionFalse &&
		condition.Message != "" {
		var podNames []string
		_ = json.Unmarshal([]byte(condition.Message), &podNames)
		podSet = sets.New(podNames...)
	}
	return podSet
}

// CalculateConcurrencyReplicas returns absolute value of concurrency for workload. This func can solve some
// corner cases about percentage-type concurrency, such as:
// - if concurrency > "0%" and replicas > 0, it will ensure at least 1 pod is reserved.
// - if concurrency < "100%" and replicas > 1, it will ensure at least 1 pod is reserved.
//
// if concurrency is nil, concurrency will be treated as 100%.
func CalculateConcurrencyReplicas(concurrency *intstr.IntOrString, replicas int) (int, error) {
	if concurrency == nil {
		return replicas, nil
	}

	// 'roundUp=true' will ensure at least 1 pod is reserved if concurrency > "0%" and replicas > 0.
	pValue, err := intstr.GetScaledValueFromIntOrPercent(concurrency, replicas, true)
	if err != nil {
		return pValue, err
	}

	// if concurrency < "100%" and replicas > 1, it will ensure at least 1 pod is reserved.
	if replicas > 1 && pValue == replicas && concurrency.Type == intstr.String && concurrency.StrVal != "100%" {
		pValue = replicas - 1
	}

	pValue = integer.IntMax(integer.IntMin(pValue, replicas), 0)
	return pValue, nil
}
