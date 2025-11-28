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
	PostProvision(ctx context.Context, cli client.Reader, opts *Options) error

	PreTerminate(ctx context.Context, cli client.Reader, opts *Options) error

	ShardAdd(ctx context.Context, cli client.Reader, opts *Options) error

	ShardRemove(ctx context.Context, cli client.Reader, opts *Options) error
}

func NewShardingLifecycle(namespace, clusterName, compName, shardingName string, lifecycleActions *appsv1.ShardingLifecycleActions,
	templateVars map[string]string, pod *corev1.Pod, pods []*corev1.Pod) (ShardingLifecycle, error) {
	agent, err := New(namespace, clusterName, compName, nil, templateVars, pod, pods)
	if err != nil {
		return nil, err
	}

	return &shardingAgent{
		kbagent:                  agent.(*kbagent),
		shardingName:             shardingName,
		shardingLifecycleActions: lifecycleActions,
	}, nil
}
