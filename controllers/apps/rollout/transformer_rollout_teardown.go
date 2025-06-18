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
	"slices"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type rolloutTearDownTransformer struct{}

var _ graph.Transformer = &rolloutTearDownTransformer{}

func (t *rolloutTearDownTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) {
		return nil
	}
	return t.tearDown(transCtx)
}

func (t *rolloutTearDownTransformer) tearDown(transCtx *rolloutTransformContext) error {
	if err := t.components(transCtx); err != nil {
		return err
	}
	// TODO: sharding
	return nil
}

func (t *rolloutTearDownTransformer) components(transCtx *rolloutTransformContext) error {
	rollout := transCtx.Rollout
	for _, comp := range rollout.Spec.Components {
		if comp.Strategy.Replace != nil {
			if err := t.replace(transCtx, rollout, comp); err != nil {
				return err
			}
		}
		if comp.Strategy.Create != nil {
			if err := t.create(transCtx, rollout, comp); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutTearDownTransformer) replace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	replicas, _, err := replaceReplicas(rollout, comp, spec)
	if err != nil {
		return err
	}
	tpl, err := replaceInstanceTemplate(transCtx, comp, spec)
	if err != nil {
		return err
	}
	if *tpl.Replicas == replicas && spec.Replicas == replicas && replaceStatus(transCtx, comp) {
		spec.ServiceVersion = tpl.ServiceVersion
		spec.ComponentDef = tpl.CompDef
		spec.OfflineInstances = slices.DeleteFunc(spec.OfflineInstances, func(instance string) bool {
			for _, status := range rollout.Status.Components {
				if status.Name == comp.Name {
					return slices.Contains(status.ScaleDownInstances, instance)
				}
			}
			return false
		})
	}
	return nil
}

func (t *rolloutTearDownTransformer) create(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	// TODO: impl
	return nil
}
