/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// RebuildInstanceTaskPodRequests maps RebuildInstance task Pod events back to their OpsRequest.
func RebuildInstanceTaskPodRequests(ctx context.Context, cli client.Client, pod *corev1.Pod) []reconcile.Request {
	opsName := pod.Labels[constant.OpsRequestNameLabelKey]
	opsNamespace := pod.Labels[constant.OpsRequestNamespaceLabelKey]
	if opsName == "" || opsNamespace == "" {
		return nil
	}
	key := types.NamespacedName{Namespace: opsNamespace, Name: opsName}
	opsRequest := &opsv1alpha1.OpsRequest{}
	if err := cli.Get(ctx, key, opsRequest); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return []reconcile.Request{{NamespacedName: key}}
	}
	if opsRequest.Spec.Type != opsv1alpha1.RebuildInstanceType {
		return nil
	}
	return []reconcile.Request{{NamespacedName: key}}
}
