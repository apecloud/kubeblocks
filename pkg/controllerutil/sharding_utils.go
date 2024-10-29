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

package controllerutil

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	GenerateNameMaxRetryTimes = 1000000
)

func GenShardingCompSpecList(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	compSpecList := make([]*appsv1.ClusterComponentSpec, 0)
	// list undeleted sharding component specs, the deleting ones are not included
	undeletedShardingCompSpecs, err := listUndeletedShardingCompSpecs(ctx, cli, cluster, sharding)
	if err != nil {
		return nil, err
	}
	compSpecList = append(compSpecList, undeletedShardingCompSpecs...)
	compNameMap := make(map[string]string)
	for _, existShardingCompSpec := range undeletedShardingCompSpecs {
		compNameMap[existShardingCompSpec.Name] = existShardingCompSpec.Name
	}
	shardTpl := sharding.Template
	switch {
	case len(undeletedShardingCompSpecs) == int(sharding.Shards):
		return undeletedShardingCompSpecs, err
	case len(undeletedShardingCompSpecs) < int(sharding.Shards):
		for i := len(undeletedShardingCompSpecs); i < int(sharding.Shards); i++ {
			shardClusterCompSpec := shardTpl.DeepCopy()
			genCompName, err := genRandomShardName(sharding.Name, compNameMap)
			if err != nil {
				return nil, err
			}
			shardClusterCompSpec.Name = genCompName
			compSpecList = append(compSpecList, shardClusterCompSpec)
			compNameMap[genCompName] = genCompName
		}
	case len(undeletedShardingCompSpecs) > int(sharding.Shards):
		// TODO: order by?
		compSpecList = compSpecList[:int(sharding.Shards)]
	}
	return compSpecList, nil
}

// listNCheckShardingComponents lists sharding components and checks if the sharding components are correct. It returns undeleted and deleting sharding components.
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

// listUndeletedShardingCompSpecs lists undeleted sharding component specs.
func listUndeletedShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	return listShardingCompSpecs(ctx, cli, cluster, sharding, false)
}

// listAllShardingCompSpecs lists all sharding component specs, including undeleted and deleting ones.
func listAllShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, sharding *appsv1.ClusterSharding) ([]*appsv1.ClusterComponentSpec, error) {
	return listShardingCompSpecs(ctx, cli, cluster, sharding, true)
}

// listShardingCompSpecs lists sharding component specs, with an option to include those marked for deletion.
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

// genRandomShardName generates a random name for sharding component.
func genRandomShardName(shardingName string, existShardNamesMap map[string]string) (string, error) {
	shardingNamePrefix := constant.GenerateShardingNamePrefix(shardingName)
	for i := 0; i < GenerateNameMaxRetryTimes; i++ {
		genName := common.SimpleNameGenerator.GenerateName(shardingNamePrefix)
		if _, ok := existShardNamesMap[genName]; !ok {
			return genName, nil
		}
	}
	return "", fmt.Errorf("failed to generate a unique random name for sharding component: %s after %d retries", shardingName, GenerateNameMaxRetryTimes)
}

func parseCompShortName(clusterName, compName string) (string, error) {
	name, found := strings.CutPrefix(compName, fmt.Sprintf("%s-", clusterName))
	if !found {
		return "", fmt.Errorf("the component name has no cluster name as prefix: %s", compName)
	}
	return name, nil
}
