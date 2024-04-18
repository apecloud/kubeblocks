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

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// GetPodListByRSM gets rsm pod list.
func GetPodListByRSM(ctx context.Context, cli client.Client, rsm *workloads.InstanceSet) ([]corev1.Pod, error) {
	selector, err := metav1.LabelSelectorAsMap(rsm.Spec.Selector)
	if err != nil {
		return nil, err
	}
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: rsm.Namespace},
		client.MatchingLabels(selector)); err != nil {
		return nil, err
	}
	return podList.Items, nil
}
