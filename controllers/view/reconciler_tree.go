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
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
)

type ReconcilerTree interface {
	Run() error
}

type reconcilerTree struct {
	tree        *graph.DAG
	reconcilers map[viewv1.ObjectType]reconcile.Reconciler
}

func (r *reconcilerTree) Run() error {
	r.tree.WalkTopoOrder(func(v graph.Vertex) error {
		objType, _ := v.(viewv1.ObjectType)
		reconciler, _ := r.reconcilers[objType]
		return reconciler.Reconcile()
	}, nil)
	//TODO implement me
	panic("implement me")
}

func newReconcilerTree(ctx context.Context, mClient client.Client, recorder record.EventRecorder, rules []viewv1.OwnershipRule) (ReconcilerTree, error) {
	dag := graph.NewDAG()
	reconcilers := make(map[viewv1.ObjectType]reconcile.Reconciler)
	for _, rule := range rules {
		dag.AddVertex(rule.Primary)
		reconcilers[rule.Primary] = newReconciler(ctx, mClient, recorder, rule.Primary)
		for _, resource := range rule.OwnedResources {
			dag.AddVertex(resource.Secondary)
			dag.Connect(rule.Primary, resource.Secondary)
			reconcilers[resource.Secondary] = newReconciler(ctx, mClient, recorder, resource.Secondary)
		}
	}
	// DAG should be valid(one and only one root without cycle)
	if err := dag.Validate(); err != nil {
		return nil, err
	}

	return &reconcilerTree{
		tree:        dag,
		reconcilers: reconcilers,
	}, nil
}

var _ ReconcilerTree = &reconcilerTree{}
