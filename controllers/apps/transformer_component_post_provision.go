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

package apps

import (
	"fmt"
	"time"

	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/component/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	kbCompPostProvisionDoneKey = "kubeblocks.io/post-provision-done"
)

type componentPostProvisionTransformer struct{}

var _ graph.Transformer = &componentPostProvisionTransformer{}

func (t *componentPostProvisionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	if t.checkPostProvisionDone(transCtx, dag) {
		return nil
	}
	err := t.postProvision(transCtx)
	if err != nil {
		return lifecycle.IgnoreNotDefined(err)
	}
	return t.markPostProvisionDone(transCtx, dag)
}

func (t *componentPostProvisionTransformer) checkPostProvisionDone(transCtx *componentTransformContext, dag *graph.DAG) bool {
	comp := transCtx.Component
	if comp.Annotations == nil {
		return false
	}
	// TODO: condition
	_, ok := comp.Annotations[kbCompPostProvisionDoneKey]
	return ok
}

func (t *componentPostProvisionTransformer) markPostProvisionDone(transCtx *componentTransformContext, dag *graph.DAG) error {
	comp := transCtx.Component
	if comp.Annotations == nil {
		comp.Annotations = make(map[string]string)
	}
	_, ok := comp.Annotations[kbCompPostProvisionDoneKey]
	if ok {
		return nil
	}
	compObj := comp.DeepCopy()
	timeStr := time.Now().Format(time.RFC3339Nano)
	comp.Annotations[kbCompPostProvisionDoneKey] = timeStr

	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Update(dag, compObj, comp, &model.ReplaceIfExistingOption{})
	return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for post-provision annotation to be set")
}

func (t *componentPostProvisionTransformer) postProvision(transCtx *componentTransformContext) error {
	lfa, err := lifecycleAction4Component(transCtx)
	if err != nil {
		return err
	}
	return lfa.PostProvision(transCtx.Context, transCtx.Client, nil)
}

func lifecycleAction4Component(transCtx *componentTransformContext) (lifecycle.Lifecycle, error) {
	synthesizedComp := transCtx.SynthesizeComponent

	pods, err := component.ListOwnedPods(transCtx.Context, transCtx.Client,
		synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		// TODO: (good-first-issue) we should handle the case that the component has no pods
		return nil, fmt.Errorf("has no pods to running the post-provision action")
	}

	return lifecycle.New(transCtx.SynthesizeComponent, nil, pods...)
}
