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

package component

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

func ListPodOwnedByComponent(ctx context.Context, cli client.Reader, namespace string, labels client.MatchingLabels, opts ...client.ListOption) ([]*corev1.Pod, error) {
	return ListObjWithLabelsInNamespace(ctx, cli, generics.PodSignature, namespace, labels, opts...)
}

// GetComponentPodList gets the pod list by cluster and componentName
func GetComponentPodList(ctx context.Context, cli client.Reader, cluster appsv1alpha1.Cluster, componentName string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	labels := constant.GetComponentWellKnownLabels(cluster.Name, componentName)
	if err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	return podList, nil
}

// GetComponentPodListWithRole gets the pod list with target role by cluster and componentName
func GetComponentPodListWithRole(ctx context.Context, cli client.Reader, cluster appsv1alpha1.Cluster, compSpecName, role string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	labels := constant.GetComponentWellKnownLabels(cluster.Name, compSpecName)
	labels[constant.RoleLabelKey] = role
	if err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	return podList, nil
}
