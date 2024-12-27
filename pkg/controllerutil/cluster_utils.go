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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func GetComponentSpecByName(ctx context.Context, cli client.Reader,
	cluster *appsv1.Cluster, componentName string) (*appsv1.ClusterComponentSpec, error) {
	compSpec := cluster.Spec.GetComponentByName(componentName)
	if compSpec != nil {
		return compSpec, nil
	}
	for _, sharding := range cluster.Spec.Shardings {
		shardingCompList, err := listAllShardingCompSpecs(ctx, cli, cluster, &sharding)
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

func ConvertAppsV1PersistentVolumeClaimsToCoreV1(vcts []appsv1.ClusterComponentVolumeClaimTemplate) []corev1.PersistentVolumeClaim {
	storageClassName := func(spec appsv1.PersistentVolumeClaimSpec, defaultStorageClass string) *string {
		if spec.StorageClassName != nil && *spec.StorageClassName != "" {
			return spec.StorageClassName
		}
		if defaultStorageClass != "" {
			return &defaultStorageClass
		}
		return nil
	}
	var pvcs []corev1.PersistentVolumeClaim
	for _, v := range vcts {
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: v.Name,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      v.Spec.AccessModes,
				Resources:        v.Spec.Resources,
				StorageClassName: storageClassName(v.Spec, viper.GetString(constant.CfgKeyDefaultStorageClass)),
				VolumeMode:       v.Spec.VolumeMode,
			},
		})
	}
	return pvcs
}
