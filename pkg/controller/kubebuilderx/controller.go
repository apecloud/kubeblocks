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

package kubebuilderx

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

// TODO(free6om): this is a new reconciler framework in the very early stage leaving the following tasks to do:
// 1. expose EventRecorder & Logger
// 2. parallel workflow-style reconciler chain

type Controller interface {
	Prepare(TreeLoader) Controller
	Do(...Reconciler) Controller
	Commit() (ctrl.Result, error)
}

type controller struct {
	ctx      context.Context
	cli      client.Client
	req      ctrl.Request
	recorder record.EventRecorder
	logger   logr.Logger

	res Result
	err error

	oldTree *ObjectTree
	tree    *ObjectTree
}

func (c *controller) Prepare(reader TreeLoader) Controller {
	c.oldTree, c.err = reader.Load(c.ctx, c.cli, c.req, c.recorder, c.logger)
	if c.err != nil {
		return c
	}
	if c.oldTree == nil {
		c.err = fmt.Errorf("nil tree loaded")
		return c
	}
	c.tree, c.err = c.oldTree.DeepCopy()

	// init placement
	c.ctx = intoContext(c.ctx, placement(c.oldTree.GetRoot()))

	return c
}

func (c *controller) Do(reconcilers ...Reconciler) Controller {
	if c.err != nil {
		return c
	}
	if c.res.Next != cntn && c.res.Next != cmmt && c.res.Next != rtry {
		c.err = fmt.Errorf("unexpected next action: %s. should be one of Continue, Commit or Retry", c.res.Next)
		return c
	}
	if c.res.Next != cntn {
		return c
	}
	if len(reconcilers) == 0 {
		return c
	}

	reconciler := reconcilers[0]
	switch result := reconciler.PreCondition(c.tree); {
	case result.Err != nil:
		c.err = result.Err
		return c
	case !result.Satisfied:
		return c
	}
	c.res, c.err = reconciler.Reconcile(c.tree)

	return c.Do(reconcilers[1:]...)
}

func (c *controller) Commit() (ctrl.Result, error) {
	defer c.emitFailureEvent()

	if c.err != nil {
		return ctrl.Result{}, c.err
	}
	if c.oldTree.GetRoot() == nil {
		return ctrl.Result{}, nil
	}
	builder := NewPlanBuilder(c.ctx, c.cli, c.oldTree, c.tree, c.recorder, c.logger)
	if c.err = builder.Init(); c.err != nil {
		return ctrl.Result{}, c.err
	}
	var plan graph.Plan
	plan, c.err = builder.Build()
	if c.err != nil {
		return ctrl.Result{}, c.err
	}
	if c.err = plan.Execute(); c.err != nil {
		if apierrors.IsConflict(c.err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, c.err
	}
	if c.res.Next == rtry {
		return ctrl.Result{Requeue: true, RequeueAfter: c.res.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}

func (c *controller) emitFailureEvent() {
	if c.err == nil {
		return
	}
	if c.tree == nil {
		return
	}
	if c.tree.EventRecorder == nil {
		return
	}
	if c.tree.GetRoot() == nil {
		return
	}
	// ignore object update optimistic lock conflict
	if apierrors.IsConflict(c.err) {
		return
	}
	// TODO(free6om): make error message user-friendly
	c.tree.EventRecorder.Eventf(c.tree.GetRoot(), corev1.EventTypeWarning, "FailedReconcile", "reconcile failed: %s", c.err.Error())
}

func NewController(ctx context.Context, cli client.Client, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) Controller {
	return &controller{
		ctx:      ctx,
		cli:      cli,
		req:      req,
		recorder: recorder,
		logger:   logger,
		res:      Continue,
	}
}

var _ Controller = &controller{}
