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
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func GenShardingCompSpecList4Test(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	offline := make([]string, 0)
	if sharding != nil && len(sharding.Offline) > 0 {
		for _, name := range sharding.Offline {
			shortName, err := parseCompShortName4Test(cluster.Name, name)
			if err != nil {
				return nil, err
			}
			offline = append(offline, shortName)
		}
	}

	// list undeleted sharding component specs, the deleting ones are not included
	undeletedShardingCompSpecs, err := listUndeletedShardingCompSpecs4Test(ctx, cli, cluster, sharding)
	if err != nil {
		return nil, err
	}
	shards := removeOfflineShards4Test(undeletedShardingCompSpecs, offline)

	shardNames := sets.Set[string]{}
	for _, existShardingCompSpec := range undeletedShardingCompSpecs {
		shardNames.Insert(existShardingCompSpec.Name)
	}
	shardNames.Insert(offline...) // exclude offline shard names

	shardTpl := sharding.Template
	switch {
	case len(shards) == int(sharding.Shards):
		return shards, nil
	case len(shards) < int(sharding.Shards):
		for i := len(shards); i < int(sharding.Shards); i++ {
			name, err := genRandomShardName4Test(sharding.Name, shardNames)
			if err != nil {
				return nil, err
			}
			spec := shardTpl.DeepCopy()
			spec.Name = name
			shards = append(shards, spec)
			shardNames.Insert(name)
		}
	case len(shards) > int(sharding.Shards):
		slices.SortFunc(shards, func(a, b *appsv1.ClusterComponentSpec) int {
			return strings.Compare(a.Name, b.Name)
		})
		shards = shards[:int(sharding.Shards)]
	}
	return shards, nil
}

func parseCompShortName4Test(clusterName, compName string) (string, error) {
	name, found := strings.CutPrefix(compName, fmt.Sprintf("%s-", clusterName))
	if !found {
		return "", fmt.Errorf("the component name has no cluster name as prefix: %s", compName)
	}
	return name, nil
}

func listUndeletedShardingCompSpecs4Test(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	return listShardingCompSpecs4Test(ctx, cli, cluster, sharding, false)
}

func removeOfflineShards4Test(shards []*appsv1.ClusterComponentSpec, offline []string) []*appsv1.ClusterComponentSpec {
	if len(offline) > 0 {
		s := sets.New(offline...)
		return slices.DeleteFunc(shards, func(shard *appsv1.ClusterComponentSpec) bool {
			return s.Has(shard.Name)
		})
	}
	return shards
}

func genRandomShardName4Test(shardingName string, shardNames sets.Set[string]) (string, error) {
	for i := 0; i < generateShardIDMaxRetryTimes; i++ {
		id := rand.String(ShardIDLength)
		name := fmt.Sprintf("%s-%s", shardingName, id)
		if !shardNames.Has(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("failed to generate a unique random name for sharding component: %s after %d retries", shardingName, generateShardIDMaxRetryTimes)
}

func ListShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	return listShardingCompSpecs4Test(ctx, cli, cluster, sharding, true)
}

func listShardingCompSpecs4Test(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding, includeDeleting bool) ([]*appsv1.ClusterComponentSpec, error) {
	if sharding == nil {
		return nil, nil
	}

	undeletedShardingComps, deletingShardingComps, err := listNCheckShardingComponents4Test(ctx, cli, cluster, sharding)
	if err != nil {
		return nil, err
	}

	compSpecList := make([]*appsv1.ClusterComponentSpec, 0, len(undeletedShardingComps)+len(deletingShardingComps))
	shardTpl := sharding.Template

	processComps := func(comps []appsv1.Component) error {
		for _, comp := range comps {
			compShortName, err := parseCompShortName4Test(cluster.Name, comp.Name)
			if err != nil {
				return err
			}
			shardClusterCompSpec := shardTpl.DeepCopy()
			shardClusterCompSpec.Name = compShortName
			compSpecList = append(compSpecList, shardClusterCompSpec)
		}
		return nil
	}

	err = processComps(undeletedShardingComps)
	if err != nil {
		return nil, err
	}

	if includeDeleting {
		err = processComps(deletingShardingComps)
		if err != nil {
			return nil, err
		}
	}

	return compSpecList, nil
}

func listNCheckShardingComponents4Test(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]appsv1.Component, []appsv1.Component, error) {
	shardingComps, err := ListShardingComponents(ctx, cli, cluster, sharding.Name)
	if err != nil {
		return nil, nil, err
	}

	deletingShardingComps := make([]appsv1.Component, 0)
	undeletedShardingComps := make([]appsv1.Component, 0)
	for _, comp := range shardingComps {
		if comp.GetDeletionTimestamp().IsZero() {
			undeletedShardingComps = append(undeletedShardingComps, comp)
		} else {
			deletingShardingComps = append(deletingShardingComps, comp)
		}
	}

	// TODO: ???
	// if cluster.Generation == cluster.Status.ObservedGeneration && len(undeletedShardingComps) != int(sharding.Shards) {
	//	return nil, nil, errors.New("sharding components are not correct when cluster is not updating")
	// }

	return undeletedShardingComps, deletingShardingComps, nil
}
