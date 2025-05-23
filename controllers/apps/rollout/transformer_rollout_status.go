/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package rollout

import (
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type rolloutStatusTransformer struct{}

var _ graph.Transformer = &rolloutStatusTransformer{}

func (t *rolloutStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) {
		return nil
	}

	if err := t.components(transCtx, dag); err != nil {
		return err
	}

	// TODO: sharding

	transCtx.Rollout.Status.ObservedGeneration = transCtx.Rollout.Generation

	// TODO: phase & message, conditions

	return nil
}

func (t *rolloutStatusTransformer) components(transCtx *rolloutTransformContext, dag *graph.DAG) error {
	rollout := transCtx.Rollout
	for _, comp := range rollout.Spec.Components {
		if err := t.component(transCtx, rollout, comp); err != nil {
			return err
		}
	}
	return nil
}

func (t *rolloutStatusTransformer) component(transCtx *rolloutTransformContext, rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	if spec == nil {
		return fmt.Errorf("the component %s is not found in cluster", comp.Name)
	}

	func() {
		for _, status := range rollout.Status.Components {
			if status.Name == comp.Name {
				return
			}
		}
		rollout.Status.Components = append(rollout.Status.Components, appsv1alpha1.RolloutComponentStatus{
			Name:     comp.Name,
			Replicas: spec.Replicas,
		})
	}()

	return nil
}
