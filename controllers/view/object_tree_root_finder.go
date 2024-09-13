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
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ObjectTreeRootFinder interface {
	GetEventChannel() chan event.GenericEvent
	GetEventHandler() handler.EventHandler
}

type rootFinder struct {
	client.Client
}

func (r *rootFinder) GetEventChannel() chan event.GenericEvent {
	//TODO implement me
	panic("implement me")
}

func (r *rootFinder) GetEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(r.findRoots)
}

func (r *rootFinder) findRoots(ctx context.Context, object client.Object) []reconcile.Request {
	//TODO implement me
	panic("implement me")

	// new a waiting list W
	// put 'object' into W

	// list all view definitions
	// build primary type list P
	// build ownership rule list O

	// while(len(W) > 0) {
	// get and remove head of W
	// traverse P
	// if object type matched, put object to root object list R, continue
	// traverse O
	// if ownedResources contains type of 'object'
	// list all primary objects of this rule
	// traverse the list
	// try applying ownership rule to the primary object
	// if matched, append to W
	// }
	// return R

	return nil
}

func NewObjectTreeRootFinder(cli client.Client) ObjectTreeRootFinder {
	return &rootFinder{Client: cli}
}

var _ ObjectTreeRootFinder = &rootFinder{}
