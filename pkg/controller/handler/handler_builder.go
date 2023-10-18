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

package handler

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// Builder defines an EventHandler builder
type Builder interface {
	AddFinder(Finder) Builder
	Build() handler.EventHandler
}

type realBuilder struct {
	ctx     *FinderContext
	finders []Finder
}

var _ Builder = &realBuilder{}

func NewBuilder(ctx *FinderContext) Builder {
	return &realBuilder{ctx: ctx}
}

func (builder *realBuilder) AddFinder(finder Finder) Builder {
	builder.finders = append(builder.finders, finder)
	return builder
}

func (builder *realBuilder) Build() handler.EventHandler {
	fn := func(ctx context.Context, obj client.Object) []reconcile.Request {
		var key *model.GVKNObjKey
		for i, finder := range builder.finders {
			key = finder.Find(builder.ctx, obj)
			if key == nil {
				return nil
			}
			if i < len(builder.finders)-1 {
				obj = getObjectFromKey(builder.ctx, key)
				if obj == nil {
					return nil
				}
			}
		}
		if key == nil {
			return nil
		}
		return []reconcile.Request{{NamespacedName: key.ObjectKey}}
	}

	return handler.EnqueueRequestsFromMapFunc(fn)
}
