package rsm2

import (
	"context"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
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

// isPending returns true if pod has a Phase of PodPending
func isPending(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodPending
}

// isFailed returns true if pod has a Phase of PodFailed
func isFailed(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodFailed
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
