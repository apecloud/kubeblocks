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

package view

import (
	"container/list"
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
)

type ObjectTreeRootFinder interface {
	GetEventChannel() chan event.GenericEvent
	GetEventHandler() handler.EventHandler
}

type rootFinder struct {
	client.Client
	logger    logr.Logger
	eventChan chan event.GenericEvent
}

func (f *rootFinder) GetEventChannel() chan event.GenericEvent {
	return f.eventChan
}

func (f *rootFinder) GetEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(f.findRoots)
}

// findRoots finds the root(s) object of the 'object' by the object tree.
// The basic idea is, find the parent(s) of the object based on ownership rules defined in view definition,
// and do this recursively until find all the root object(s).
func (f *rootFinder) findRoots(ctx context.Context, object client.Object) []reconcile.Request {
	waitingList := list.New()
	waitingList.PushFront(object)

	var roots []client.Object
	primaryTypeList := []viewv1.ObjectType{rootObjectType}
	for waitingList.Len() > 0 {
		e := waitingList.Front()
		waitingList.Remove(e)
		obj, _ := e.Value.(client.Object)
		objGVK, err := apiutil.GVKForObject(obj, f.Scheme())
		if err != nil {
			f.logger.Error(err, "get GVK of %s/%s failed", obj.GetNamespace(), obj.GetName())
			return nil
		}
		found := false
		for _, primaryType := range primaryTypeList {
			gvk, err := objectTypeToGVK(&primaryType)
			if err != nil {
				f.logger.Error(err, "convert objectType %s to GVK failed", primaryType)
				return nil
			}
			if objGVK == *gvk {
				roots = append(roots, obj)
				found = true
			}
		}
		if found {
			continue
		}
		for i := range kbOwnershipRules {
			rule := &kbOwnershipRules[i]
			for _, resource := range rule.OwnedResources {
				gvk, err := objectTypeToGVK(&resource.Secondary)
				if err != nil {
					f.logger.Error(err, "convert objectType %s to GVK failed", resource.Secondary)
					return nil
				}
				if objGVK != *gvk {
					continue
				}
				primaryGVK, err := objectTypeToGVK(&rule.Primary)
				if err != nil {
					f.logger.Error(err, "convert objectType %s to GVK failed", rule.Primary)
					return nil
				}
				objectList, err := getObjectsByGVK(ctx, f, primaryGVK, nil)
				if err != nil {
					f.logger.Error(err, "getObjectsByGVK for GVK %s failed", primaryGVK)
					return nil
				}
				for _, owner := range objectList {
					opts, err := parseQueryOptions(owner, &resource.Criteria)
					if err != nil {
						f.logger.Error(err, "parse query options failed: %s", resource.Criteria)
						return nil
					}
					if opts.match(obj) {
						waitingList.PushBack(owner)
					}
				}
			}
		}
	}

	clusterKeys := sets.New[client.ObjectKey]()
	for _, root := range roots {
		clusterKeys.Insert(client.ObjectKeyFromObject(root))
	}

	// list all view objects, filter by result Cluster objects.
	viewList := &viewv1.ReconciliationViewList{}
	if err := f.List(ctx, viewList); err != nil {
		f.logger.Error(err, "list view failed", "")
		return nil
	}
	getTargetObjectKey := func(view *viewv1.ReconciliationView) client.ObjectKey {
		key := client.ObjectKeyFromObject(view)
		if view.Spec.TargetObject != nil {
			key.Namespace = view.Spec.TargetObject.Namespace
			key.Name = view.Spec.TargetObject.Name
		}
		return key
	}
	var requests []reconcile.Request
	for i := range viewList.Items {
		view := &viewList.Items[i]
		key := getTargetObjectKey(view)
		if clusterKeys.Has(key) {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(view)})
		}
	}

	return requests
}

func NewObjectTreeRootFinder(cli client.Client) ObjectTreeRootFinder {
	logger := log.FromContext(context.Background()).WithName("ObjectTreeRootFinder")
	return &rootFinder{
		Client:    cli,
		logger:    logger,
		eventChan: make(chan event.GenericEvent),
	}
}

var _ ObjectTreeRootFinder = &rootFinder{}
