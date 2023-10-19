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

package utils

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PeriodicalEnqueueSource is an implementation of interface sigs.k8s.io/controller-runtime/pkg/source/Source
// It reads the specific resources from cache and enqueue them into the queue to trigger
// the reconcile procedure periodically.
type PeriodicalEnqueueSource struct {
	client.Client
	log     logr.Logger
	objList client.ObjectList
	period  time.Duration
	option  PeriodicalEnqueueSourceOption
}

type PeriodicalEnqueueSourceOption struct {
	OrderFunc func(objList client.ObjectList) client.ObjectList
}

func NewPeriodicalEnqueueSource(
	client client.Client,
	objList client.ObjectList,
	period time.Duration,
	option PeriodicalEnqueueSourceOption) *PeriodicalEnqueueSource {
	return &PeriodicalEnqueueSource{
		log:     log.Log.WithValues("resource", reflect.TypeOf(objList).String()),
		Client:  client,
		objList: objList,
		period:  period,
		option:  option,
	}
}

func (p *PeriodicalEnqueueSource) Start(
	ctx context.Context,
	_ handler.EventHandler,
	q workqueue.RateLimitingInterface,
	predicates ...predicate.Predicate) error {
	go wait.Until(func() {
		p.log.V(1).Info("enqueueing resources ...")
		if err := p.List(ctx, p.objList); err != nil {
			p.log.Error(err, "error listing resources")
			return
		}

		if meta.LenList(p.objList) == 0 {
			p.log.V(1).Info("no resources found, skip")
			return
		}

		if p.option.OrderFunc != nil {
			p.objList = p.option.OrderFunc(p.objList)
		}

		if err := meta.EachListItem(p.objList, func(object runtime.Object) error {
			obj, ok := object.(client.Object)
			if !ok {
				p.log.Error(nil, "object is not a client.Object", "object", object)
				return nil
			}
			e := event.GenericEvent{Object: obj}
			for _, pred := range predicates {
				if !pred.Generic(e) {
					p.log.V(1).Info("skip enqueue object due to the predicate", "object", obj)
					return nil
				}
			}

			q.Add(ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      obj.GetName(),
				},
			})
			p.log.V(1).Info("resource enqueued", "object", obj)
			return nil
		}); err != nil {
			p.log.Error(err, "error enqueueing resources")
			return
		}
	}, p.period, ctx.Done())

	return nil
}

func (p *PeriodicalEnqueueSource) String() string {
	if p.objList != nil {
		return fmt.Sprintf("periodical enqueue source: %T", p.objList)
	}
	return "periodical enqueue source: unknown type"
}
