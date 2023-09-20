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
	"strings"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

// ComponentDeletionTransformer handles component deletion
type ComponentDeletionTransformer struct{}

var _ graph.Transformer = &ComponentDeletionTransformer{}

func (t *ComponentDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ComponentTransformContext)
	if transCtx.Component.GetDeletionTimestamp().IsZero() {
		return nil
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	// TODO: get the resource own by component for example rsm and delete them

	transCtx.Component.Status.Phase = appsv1alpha1.DeletingClusterCompPhase
	root.Action = ictrltypes.ActionDeletePtr()

	ctx.GetRecorder().Eventf(transCtx.Component, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s:%s",
		strings.ToLower(transCtx.Component.GetObjectKind().GroupVersionKind().Kind), transCtx.Component.GetName())

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}
