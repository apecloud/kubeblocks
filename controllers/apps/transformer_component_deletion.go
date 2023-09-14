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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

// componentDeletionTransformer handles component deletion
type componentDeletionTransformer struct{}

var _ graph.Transformer = &componentDeletionTransformer{}

func (t *componentDeletionTransformer) Transform(ictx graph.TransformContext, dag *graph.DAG) error {
	ctx, _ := ictx.(*componentTransformContext)
	if ctx.Comp.GetDeletionTimestamp().IsZero() {
		return nil
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	ctx.Comp.Status.Phase = appsv1alpha1.DeletingClusterCompPhase
	root.Action = ictrltypes.ActionDeletePtr()

	ctx.GetRecorder().Eventf(ctx.Comp, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s:%s",
		strings.ToLower(ctx.Comp.GetObjectKind().GroupVersionKind().Kind), ctx.Comp.GetName())

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}
