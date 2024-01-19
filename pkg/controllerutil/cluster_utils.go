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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// GetOriginalOrGeneratedComponentSpecByName get an original or generated cluster component spec by componentName.
func GetOriginalOrGeneratedComponentSpecByName(cluster *appsv1alpha1.Cluster, componentName string) *appsv1alpha1.ClusterComponentSpec {
	compSpec := cluster.Spec.GetComponentByName(componentName)
	if compSpec != nil {
		return compSpec
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		genShardingCompList := GenShardingCompSpecList(&shardingSpec)
		for i, shardingComp := range genShardingCompList {
			if shardingComp.Name == componentName {
				compSpec = genShardingCompList[i]
				return compSpec
			}
		}
	}
	return nil
}
