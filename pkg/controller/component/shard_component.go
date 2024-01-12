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

package component

import (
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func GenShardCompNameList(clusterCompSpec *appsv1alpha1.ClusterComponentSpec) []string {
	compList := make([]string, 0)
	compList = append(compList, clusterCompSpec.Name)
	if clusterCompSpec.Shards != nil && *clusterCompSpec.Shards > 1 {
		for i := 1; i < int(*clusterCompSpec.Shards); i++ {
			compList = append(compList, fmt.Sprintf("%s-%d", clusterCompSpec.Name, i))
		}
	}
	return compList
}

func GenShardCompSpecList(clusterCompSpec *appsv1alpha1.ClusterComponentSpec) []*appsv1alpha1.ClusterComponentSpec {
	compSpecList := make([]*appsv1alpha1.ClusterComponentSpec, 0)
	if clusterCompSpec.Shards != nil && *clusterCompSpec.Shards > 1 {
		for i := 0; i < int(*clusterCompSpec.Shards); i++ {
			shardClusterCompSpec := clusterCompSpec.DeepCopy()
			shardClusterCompSpec.Shards = nil
			if i == 0 {
				compSpecList = append(compSpecList, shardClusterCompSpec)
				continue
			}
			shardClusterCompSpec.Shards = nil
			shardClusterCompSpec.Name = fmt.Sprintf("%s-%d", clusterCompSpec.Name, i)
			compSpecList = append(compSpecList, shardClusterCompSpec)
		}
	} else {
		genClusterCompSpec := clusterCompSpec.DeepCopy()
		genClusterCompSpec.Shards = nil
		compSpecList = append(compSpecList, genClusterCompSpec)
	}
	return compSpecList
}
