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

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	generateNameMaxRetryTimes = 1000000
)

// GenShardingCompSpecList4Test - TODO: remove this function
func GenShardingCompSpecList4Test(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	offline := make([]string, 0)
	if sharding != nil && len(sharding.Offline) > 0 {
		for _, name := range sharding.Offline {
			shortName, err := parseCompShortName(cluster.Name, name)
			if err != nil {
				return nil, err
			}
			offline = append(offline, shortName)
		}
	}

	// list undeleted sharding component specs, the deleting ones are not included
	undeletedShardingCompSpecs, err := listUndeletedShardingCompSpecs(ctx, cli, cluster, sharding)
	if err != nil {
		return nil, err
	}
	shards := removeOfflineShards(undeletedShardingCompSpecs, offline)

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
			name, err := genRandomShardName(sharding.Name, shardNames)
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

func BuildShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) (map[string][]*appsv1.ClusterComponentSpec, error) {
	return nil, nil
	// offline := make([]string, 0)
	// if sharding != nil && len(sharding.Offline) > 0 {
	//	for _, name := range sharding.Offline {
	//		shortName, err := parseCompShortName(cluster.Name, name)
	//		if err != nil {
	//			return nil, err
	//		}
	//		offline = append(offline, shortName)
	//	}
	// }
	//
	//// list undeleted sharding component specs, the deleting ones are not included
	// undeletedShardingCompSpecs, err := listUndeletedShardingCompSpecs(ctx, cli, cluster, sharding)
	// if err != nil {
	//	return nil, err
	// }
	// shards := removeOfflineShards(undeletedShardingCompSpecs, offline)
	//
	// shardNames := sets.Set[string]{}
	// for _, existShardingCompSpec := range undeletedShardingCompSpecs {
	//	shardNames.Insert(existShardingCompSpec.Name)
	// }
	// shardNames.Insert(offline...) // exclude offline shard names
	//
	// shardTpl := sharding.Template
	// switch {
	// case len(shards) == int(sharding.Shards):
	//	return shards, nil
	// case len(shards) < int(sharding.Shards):
	//	for i := len(shards); i < int(sharding.Shards); i++ {
	//		name, err := genRandomShardName(sharding.Name, shardNames)
	//		if err != nil {
	//			return nil, err
	//		}
	//		spec := shardTpl.DeepCopy()
	//		spec.Name = name
	//		shards = append(shards, spec)
	//		shardNames.Insert(name)
	//	}
	// case len(shards) > int(sharding.Shards):
	//	slices.SortFunc(shards, func(a, b *appsv1.ClusterComponentSpec) int {
	//		return strings.Compare(a.Name, b.Name)
	//	})
	//	shards = shards[:int(sharding.Shards)]
	// }
	// return shards, nil
}

func ListShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	return listShardingCompSpecs(ctx, cli, cluster, sharding, true)
}

func ListShardingComponents(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, shardingName string) ([]appsv1.Component, error) {
	compList := &appsv1.ComponentList{}
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:       cluster.Name,
		constant.KBAppShardingNameLabelKey: shardingName,
	}
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), ml); err != nil {
		return nil, err
	}
	return compList.Items, nil
}

func parseCompShortName(clusterName, compName string) (string, error) {
	name, found := strings.CutPrefix(compName, fmt.Sprintf("%s-", clusterName))
	if !found {
		return "", fmt.Errorf("the component name has no cluster name as prefix: %s", compName)
	}
	return name, nil
}

func removeOfflineShards(shards []*appsv1.ClusterComponentSpec, offline []string) []*appsv1.ClusterComponentSpec {
	if len(offline) > 0 {
		s := sets.New(offline...)
		return slices.DeleteFunc(shards, func(shard *appsv1.ClusterComponentSpec) bool {
			return s.Has(shard.Name)
		})
	}
	return shards
}

func genRandomShardName(shardingName string, shardNames sets.Set[string]) (string, error) {
	shardingNamePrefix := constant.GenerateShardingNamePrefix(shardingName)
	for i := 0; i < generateNameMaxRetryTimes; i++ {
		name := common.SimpleNameGenerator.GenerateName(shardingNamePrefix)
		if !shardNames.Has(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("failed to generate a unique random name for sharding component: %s after %d retries", shardingName, generateNameMaxRetryTimes)
}

func listUndeletedShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	return listShardingCompSpecs(ctx, cli, cluster, sharding, false)
}

func listShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding, includeDeleting bool) ([]*appsv1.ClusterComponentSpec, error) {
	if sharding == nil {
		return nil, nil
	}

	undeletedShardingComps, deletingShardingComps, err := listNCheckShardingComponents(ctx, cli, cluster, sharding)
	if err != nil {
		return nil, err
	}

	compSpecList := make([]*appsv1.ClusterComponentSpec, 0, len(undeletedShardingComps)+len(deletingShardingComps))
	shardTpl := sharding.Template

	processComps := func(comps []appsv1.Component) error {
		for _, comp := range comps {
			compShortName, err := parseCompShortName(cluster.Name, comp.Name)
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

func listNCheckShardingComponents(ctx context.Context, cli client.Reader,
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
