/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	GenerateNameMaxRetryTimes = 1000000
)

func GenShardingCompSpecList(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster, shardingSpec *appsv1alpha1.ShardingSpec) ([]*appsv1alpha1.ClusterComponentSpec, error) {
	compSpecList := make([]*appsv1alpha1.ClusterComponentSpec, 0)
	existShardingCompSpecs, err := listShardingCompSpecs(ctx, cli, cluster, shardingSpec)
	if err != nil {
		return nil, err
	}
	compSpecList = append(compSpecList, existShardingCompSpecs...)
	compNameMap := make(map[string]string)
	for _, existShardingCompSpec := range existShardingCompSpecs {
		compNameMap[existShardingCompSpec.Name] = existShardingCompSpec.Name
	}
	shardTpl := shardingSpec.Template
	switch {
	case len(existShardingCompSpecs) == int(shardingSpec.Shards):
		return existShardingCompSpecs, err
	case len(existShardingCompSpecs) < int(shardingSpec.Shards):
		for i := len(existShardingCompSpecs); i < int(shardingSpec.Shards); i++ {
			shardClusterCompSpec := shardTpl.DeepCopy()
			genCompName, err := genRandomShardName(shardingSpec.Name, compNameMap)
			if err != nil {
				return nil, err
			}
			shardClusterCompSpec.Name = genCompName
			compSpecList = append(compSpecList, shardClusterCompSpec)
			compNameMap[genCompName] = genCompName
		}
	case len(existShardingCompSpecs) > int(shardingSpec.Shards):
		// TODO: order by?
		compSpecList = compSpecList[:int(shardingSpec.Shards)]
	}
	return compSpecList, nil
}

func ListShardingCompNames(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster, shardingSpec *appsv1alpha1.ShardingSpec) ([]string, error) {
	compNameList := make([]string, 0)
	if shardingSpec == nil {
		return compNameList, nil
	}

	existShardingComps, err := listNCheckShardingComponents(ctx, cli, cluster, shardingSpec)
	if err != nil {
		return compNameList, err
	}
	for _, comp := range existShardingComps {
		compShortName, err := parseCompShortName(cluster.Name, comp.Name)
		if err != nil {
			return nil, err
		}
		compNameList = append(compNameList, compShortName)
	}
	return compNameList, nil
}

func listNCheckShardingComponents(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster, shardingSpec *appsv1alpha1.ShardingSpec) ([]appsv1alpha1.Component, error) {
	shardingComps, err := listShardingComponents(ctx, cli, cluster, shardingSpec)
	if err != nil {
		return nil, err
	}
	if cluster.Generation == cluster.Status.ObservedGeneration && len(shardingComps) != int(shardingSpec.Shards) {
		return nil, errors.New("sharding components are not correct when cluster is not updating")
	}
	return shardingComps, nil
}

func listShardingComponents(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster, shardingSpec *appsv1alpha1.ShardingSpec) ([]appsv1alpha1.Component, error) {
	compList := &appsv1alpha1.ComponentList{}
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:       cluster.Name,
		constant.KBAppShardingNameLabelKey: shardingSpec.Name,
	}
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), ml); err != nil {
		return nil, err
	}
	return compList.Items, nil
}

func listShardingCompSpecs(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster, shardingSpec *appsv1alpha1.ShardingSpec) ([]*appsv1alpha1.ClusterComponentSpec, error) {
	compSpecList := make([]*appsv1alpha1.ClusterComponentSpec, 0)
	compNameMap := make(map[string]string)
	if shardingSpec == nil {
		return compSpecList, nil
	}

	existShardingComps, err := listNCheckShardingComponents(ctx, cli, cluster, shardingSpec)
	if err != nil {
		return nil, err
	}

	shardTpl := shardingSpec.Template
	for _, existShardingComp := range existShardingComps {
		existShardingCompShortName, err := parseCompShortName(cluster.Name, existShardingComp.Name)
		if err != nil {
			return nil, err
		}
		shardClusterCompSpec := shardTpl.DeepCopy()
		shardClusterCompSpec.Name = existShardingCompShortName
		compSpecList = append(compSpecList, shardClusterCompSpec)
		compNameMap[existShardingCompShortName] = existShardingCompShortName
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
