package rsm2

import (
	"context"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	"sort"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
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

func CurrentReplicaProvider(ctx context.Context, cli client.Reader, rsm *workloads.ReplicatedStateMachine) (ReplicaProvider, error) {
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
	switch err := cli.Get(ctx, client.ObjectKeyFromObject(rsm), sts); {
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
