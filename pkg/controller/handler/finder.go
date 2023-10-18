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
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type FinderContext struct {
	context.Context
	client.Reader
	Scheme runtime.Scheme
}

// Finder finds a new object by an old object
type Finder interface {
	Find(*FinderContext, client.Object) *model.GVKNObjKey
}

type baseFinder struct {
	objectType runtime.Object
	gvkMutex   sync.Mutex
	gvkCache   *schema.GroupVersionKind
}

func getObjectFromKey(ctx *FinderContext, key *model.GVKNObjKey) client.Object {
	objectRT, err := ctx.Scheme.New(key.GroupVersionKind)
	if err != nil {
		return nil
	}
	object, ok := objectRT.(client.Object)
	if !ok {
		return nil
	}
	if err = ctx.Reader.Get(ctx, key.ObjectKey, object); err != nil {
		return nil
	}
	return object
}

func (finder *baseFinder) getGroupVersionKind(scheme *runtime.Scheme) *schema.GroupVersionKind {
	finder.gvkMutex.Lock()
	defer finder.gvkMutex.Unlock()
	if finder.gvkCache == nil {
		// Get the kinds of the type
		kinds, _, err := scheme.ObjectKinds(finder.objectType)
		if err != nil {
			return nil
		}
		// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
		if len(kinds) != 1 {
			// expected exactly 1 kind for object
			return nil
		}
		finder.gvkCache = &kinds[0]
	}
	return finder.gvkCache
}
