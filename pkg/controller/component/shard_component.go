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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func GenShardCompNameList(shardSpec *appsv1alpha1.ShardSpec) []string {
	compNameList := make([]string, 0)
	if shardSpec == nil {
		return compNameList
	}
	for i := 0; i < int(shardSpec.Shards); i++ {
		compNameList = append(compNameList, constant.GenerateShardComponentName(shardSpec.Name, i))
	}
	return compNameList
}

func GenShardCompSpecList(shardSpec *appsv1alpha1.ShardSpec) []*appsv1alpha1.ClusterComponentSpec {
	compSpecList := make([]*appsv1alpha1.ClusterComponentSpec, 0)
	if shardSpec == nil {
		return compSpecList
	}
	shardTpl := shardSpec.Template
	for i := 0; i < int(shardSpec.Shards); i++ {
		shardClusterCompSpec := shardTpl.DeepCopy()
		shardClusterCompSpec.Name = constant.GenerateShardComponentName(shardSpec.Name, i)
		compSpecList = append(compSpecList, shardClusterCompSpec)
	}
	return compSpecList
}
