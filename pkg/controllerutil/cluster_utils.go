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

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// GetOriginalOrGeneratedComponentSpecByName get an original or generated cluster component spec by componentName.
func GetOriginalOrGeneratedComponentSpecByName(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster, componentName string) (*appsv1alpha1.ClusterComponentSpec, error) {
	compSpec := cluster.Spec.GetComponentByName(componentName)
	if compSpec != nil {
		return compSpec, nil
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		shardingCompList, err := listAllShardingCompSpecs(ctx, cli, cluster, &shardingSpec)
		if err != nil {
			return nil, err
		}
		for i, shardingComp := range shardingCompList {
			if shardingComp.Name == componentName {
				compSpec = shardingCompList[i]
				return compSpec, nil
			}
		}
	}
	return nil, nil
}

func TemplateReplicas(template appsv1alpha1.InstanceTemplate) int32 {
	replicas := int32(1)
	if template.Replicas != nil {
		replicas = *template.Replicas
	}
	return replicas
}
