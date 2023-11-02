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

package apps

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type componentMetaTransformer struct{}

var _ graph.Transformer = &componentMetaTransformer{}

func (t *componentMetaTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	component := transCtx.Component
	componentOrig := transCtx.ComponentOrig
	needUpdate := false

	// if !controllerutil.ContainsFinalizer(component, constant.DBComponentFinalizerName) {
	//	controllerutil.AddFinalizer(component, constant.DBComponentFinalizerName)
	//	needUpdate = true
	// }
	if !controllerutil.ContainsFinalizer(component, constant.DBClusterFinalizerName) {
		controllerutil.AddFinalizer(component, constant.DBClusterFinalizerName)
		needUpdate = true
	}

	labels := component.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labelName := labels[constant.ComponentDefinitionLabelKey]
	if labelName != component.Spec.CompDef {
		labels[constant.ComponentDefinitionLabelKey] = component.Spec.CompDef
		needUpdate = true
	}

	if !needUpdate {
		return nil
	}

	component.Labels = labels
	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Update(dag, componentOrig, component)

	return graph.ErrPrematureStop
}
