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

package experimental

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	experimental "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
)

type nodeScalingHandler struct {
	client.Client
}

func (h *nodeScalingHandler) Create(ctx context.Context, event event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
	h.mapAndEnqueue(ctx, limitingInterface)
}

func (h *nodeScalingHandler) Update(ctx context.Context, event event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
}

func (h *nodeScalingHandler) Delete(ctx context.Context, event event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
	h.mapAndEnqueue(ctx, limitingInterface)
}

func (h *nodeScalingHandler) Generic(ctx context.Context, event event.GenericEvent, limitingInterface workqueue.RateLimitingInterface) {
}

func (h *nodeScalingHandler) mapAndEnqueue(ctx context.Context, q workqueue.RateLimitingInterface) {
	scalerList := &experimental.NodeCountScalerList{}
	if err := h.Client.List(ctx, scalerList); err == nil {
		for _, item := range scalerList.Items {
			q.Add(ctrl.Request{NamespacedName: types.NamespacedName{Namespace: item.Namespace, Name: item.Name}})
		}
	}
}

var _ handler.EventHandler = &nodeScalingHandler{}
