/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package lifecycle

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
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
