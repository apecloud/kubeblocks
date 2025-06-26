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

package sharding

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func BuildShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) (map[string][]*appsv1.ClusterComponentSpec, error) {
	shardingComps, err := ListShardingComponents(ctx, cli, cluster, sharding.Name)
	if err != nil {
		return nil, err
	}

	generator := &shardIDGenerator{
		clusterName:  cluster.Name,
		shardingName: sharding.Name,
		running:      shardingComps,
		offline:      sharding.Offline,
	}

	templates := buildShardTemplates(cluster.Name, sharding, shardingComps)
	for i := range templates {
		if err = templates[i].align(generator, sharding.Name); err != nil {
			return nil, err
		}
	}

	shards := map[string][]*appsv1.ClusterComponentSpec{}
	for i, tpl := range templates {
		shards[tpl.name] = templates[i].shards
	}
	return shards, nil
}

func ListShardingComponents(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster, shardingName string) ([]appsv1.Component, error) {
	compList := &appsv1.ComponentList{}
	labels := constant.GetClusterLabels(cluster.Name, map[string]string{constant.KBAppShardingNameLabelKey: shardingName})
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	return compList.Items, nil
}
