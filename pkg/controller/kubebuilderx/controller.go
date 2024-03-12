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

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This framework builds upon the original DAG framework, which abstracts the reconciliation process into two stages.
// The first stage is the pure computation phase, where the goal is to generate an execution plan.
// The second stage is the plan execution phase, responsible for applying the changes computed in the first stage to the K8s API server.
// The design choice of making the first stage purely computational serves two purposes.
// Firstly, it allows leveraging the experience and patterns from functional programming to make the code more robust.
// Secondly, it enables breaking down complex business logic into smaller units, facilitating testing.
// The new framework retains this concept while attempting to address the following issues of the original approach:
// 1. The low-level exposure of the DAG data structure, which should be abstracted away.
// 2. The execution of business logic code being deferred, making step-by-step tracing and debugging challenging.
// Additionally, the new framework further abstracts the concept of object snapshots into an ObjectTree,
// making it easier to apply the experience of editing a group of related objects using kubectl.

// TODO(free6om): this is a new reconciler framework in the very early stage leaving the following tasks to do:
// 1. don't expose the client.Client to the Reconciler, to prevent write operation in Prepare and Do stages.
// 2. expose EventRecorder
// 3. expose Logger

type Controller interface {
	Prepare(reader TreeReader) Controller
	Do(...Reconciler) Controller
	Commit(writer client.Client) error
}

type controller struct {
	ctx      context.Context
	reader   client.Reader
	req      ctrl.Request
	recorder record.EventRecorder
	logger   logr.Logger

	err error

	oldTree *ObjectTree
	tree    *ObjectTree
}

func (c *controller) Prepare(reader TreeReader) Controller {
	c.oldTree, c.err = reader.Read(c.ctx, c.reader, c.req, c.recorder, c.logger)
	if c.err != nil {
		return c
	}
	c.tree, c.err = c.oldTree.DeepCopy()
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

func (c *controller) Commit(writer client.Client) error {
	if c.err != nil {
		return c.err
	}
	builder := NewPlanBuilder(c.ctx, writer, c.oldTree, c.tree)
	if err := builder.Init(); err != nil {
		return err
	}
	builder.AddTransformer(newObjectTree2DAGTransformer(c.oldTree, c.tree))
	plan, err := builder.Build()
	if err != nil {
		return err
	}
	return plan.Execute()
}

func NewController(ctx context.Context, reader client.Reader, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) Controller {
	return &controller{
		ctx:      ctx,
		reader:   reader,
		req:      req,
		recorder: recorder,
		logger:   logger,
	}
}

var _ Controller = &controller{}
