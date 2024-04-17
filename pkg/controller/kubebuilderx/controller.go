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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO(free6om): this is a new reconciler framework in the very early stage leaving the following tasks to do:
// 1. don't expose the client.Client to the Reconciler, to prevent write operation in Prepare and Do stages.
// 2. expose EventRecorder
// 3. expose Logger

type Controller interface {
	Prepare(TreeLoader) Controller
	Do(...Reconciler) Controller
	Commit() error
}

type controller struct {
	ctx      context.Context
	cli      client.Client
	req      ctrl.Request
	recorder record.EventRecorder
	logger   logr.Logger

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

	for _, reconciler := range reconcilers {
		switch result := reconciler.PreCondition(c.tree); {
		case result.Err != nil:
			c.err = result.Err
			return c
		case !result.Satisfied:
			return c
		}

		c.tree, c.err = reconciler.Reconcile(c.tree)
		if c.err != nil {
			return c
		}
	}

	return c
}

func (c *controller) Commit() error {
	if c.err != nil {
		return c.err
	}
	if c.oldTree.GetRoot() == nil {
		return nil
	}
	builder := NewPlanBuilder(c.ctx, c.cli, c.oldTree, c.tree, c.recorder, c.logger)
	if err := builder.Init(); err != nil {
		return err
	}
	plan, err := builder.Build()
	if err != nil {
		return err
	}
	err = plan.Execute()
	return err
}

func NewController(ctx context.Context, cli client.Client, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) Controller {
	return &controller{
		ctx:      ctx,
		cli:      cli,
		req:      req,
		recorder: recorder,
		logger:   logger,
	}
}

var _ Controller = &controller{}
