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

package rsm2

import (
	"context"
	"sort"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func mergeMap(src, dst *map[string]string) {
	if *src == nil {
		return
	}
	if *dst == nil {
		*dst = make(map[string]string)
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

func CurrentReplicaProvider(ctx context.Context, cli client.Reader, objectKey client.ObjectKey) (ReplicaProvider, error) {
	getDefaultProvider := func() ReplicaProvider {
		provider := defaultReplicaProvider
		if viper.IsSet(FeatureGateRSMReplicaProvider) {
			provider = ReplicaProvider(viper.GetString(FeatureGateRSMReplicaProvider))
			if provider != StatefulSetProvider && provider != PodProvider {
				provider = defaultReplicaProvider
			}
		}
		return provider
	}
	sts := &appsv1.StatefulSet{}
	switch err := cli.Get(ctx, objectKey, sts); {
	case err == nil:
		return StatefulSetProvider, nil
	case !apierrors.IsNotFound(err):
		return "", err
	default:
		return getDefaultProvider(), nil
	}
}

// SortReplicas sorts replicas by their role priority and name
// e.g.: unknown -> empty -> learner -> follower1 -> follower2 -> leader, with follower1.Name < follower2.Name
// reverse it if reverse==true
func SortReplicas(replicas []replica, rolePriorityMap map[string]int, reverse bool) {
	getRoleFunc := func(i int) string {
		return rsm1.GetRoleName(*replicas[i].pod)
	}
	getNameFunc := func(i int) string {
		return replicas[i].pod.Name
	}
	sort.SliceStable(replicas, func(i, j int) bool {
		if reverse {
			i, j = j, i
		}
		roleI := getRoleFunc(i)
		roleJ := getRoleFunc(j)
		if rolePriorityMap[roleI] == rolePriorityMap[roleJ] {
			ordinal1 := getNameFunc(i)
			ordinal2 := getNameFunc(j)
			return ordinal1 < ordinal2
		}
		return rolePriorityMap[roleI] < rolePriorityMap[roleJ]
	})
}

// isRunningAndReady returns true if pod is in the PodRunning Phase, if it has a condition of PodReady.
func isRunningAndReady(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodRunning && podutils.IsPodReady(pod)
}

func isRunningAndAvailable(pod *v1.Pod, minReadySeconds int32) bool {
	return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Now())
}

// isCreated returns true if pod has been created and is maintained by the API server
func isCreated(pod *v1.Pod) bool {
	return pod.Status.Phase != ""
}

// isTerminating returns true if pod's DeletionTimestamp has been set
func isTerminating(pod *v1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// isHealthy returns true if pod is running and ready and has not been terminated
func isHealthy(pod *v1.Pod) bool {
	return isRunningAndReady(pod) && !isTerminating(pod)
}

// getPodRevision gets the revision of Pod by inspecting the StatefulSetRevisionLabel. If pod has no revision the empty
// string is returned.
func getPodRevision(pod *v1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[appsv1.ControllerRevisionHashLabelKey]
}
