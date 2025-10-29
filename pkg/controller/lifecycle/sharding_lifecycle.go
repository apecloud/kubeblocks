package lifecycle

import (
	"context"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ShardingLifecycle interface {
	PostProvision(ctx context.Context, cli client.Reader, opts *Options) ([]string, error)

	PreTerminate(ctx context.Context, cli client.Reader, opts *Options) ([]string, error)

	ShardAdd(ctx context.Context, cli client.Reader, opts *Options) error

	ShardRemove(ctx context.Context, cli client.Reader, opts *Options) error
}

func NewShardingLifecycle(namespace, clusterName string,
	lifecycleActions *appsv1.ShardingLifecycleActions,
	compTemplateVarsMap map[string]map[string]string,
	compPodMap map[string]*corev1.Pod,
	compPodsMap map[string][]*corev1.Pod) (ShardingLifecycle, error) {
	agents := make([]kbagent, 0)

	for comp, templateVars := range compTemplateVarsMap {
		agent := kbagent{
			namespace:    namespace,
			clusterName:  clusterName,
			compName:     comp,
			templateVars: templateVars,
		}

		if compPodMap != nil {
			agent.pod = compPodMap[comp]
		}
		if compPodsMap != nil {
			agent.pods = compPodsMap[comp]
		}
		agents = append(agents, agent)
	}

	return &shardingAgent{
		compAgents:               agents,
		shardingLifecycleActions: lifecycleActions,
	}, nil
}
