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

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type componentAssureMetaTransformer struct{}

var _ graph.Transformer = &componentAssureMetaTransformer{}

func (t *componentAssureMetaTransformer) Transform(ictx graph.TransformContext, dag *graph.DAG) error {
	ctx, _ := ictx.(*componentTransformContext)
	comp := ctx.Comp

	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	if !controllerutil.ContainsFinalizer(comp, componentFinalizerName) {
		controllerutil.AddFinalizer(comp, componentFinalizerName)
	}

	// patch the label to prevent the label from being modified by the user.
	labels := comp.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labelName := labels[constant.ComponentDefinitionLabelKey]
	if labelName != comp.Spec.CompDef {
		labels[constant.ComponentDefinitionLabelKey] = comp.Spec.CompDef
		comp.Labels = labels
	}
	return nil
}
