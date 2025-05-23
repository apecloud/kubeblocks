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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type rolloutInplaceTransformer struct{}

var _ graph.Transformer = &rolloutInplaceTransformer{}

func (t *rolloutInplaceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) {
		return nil
	}
	if !model.IsObjectUpdating(transCtx.RolloutOrig) {
		return nil
	}
	return t.rollout(transCtx, dag)
}

func (t *rolloutInplaceTransformer) rollout(transCtx *rolloutTransformContext, dag *graph.DAG) error {
	if err := t.components(transCtx, dag); err != nil {
		return err
	}
	// TODO: sharding
	return nil
}

func (t *rolloutInplaceTransformer) components(transCtx *rolloutTransformContext, dag *graph.DAG) error {
	rollout := transCtx.Rollout
	for _, comp := range rollout.Spec.Components {
		if comp.Strategy.Inplace != nil {
			if err := t.component(transCtx, comp); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutInplaceTransformer) component(transCtx *rolloutTransformContext, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	if spec == nil {
		return fmt.Errorf("the component %s is not found in cluster", comp.Name)
	}

	var replicas int
	var err error
	if comp.Replicas != nil {
		replicas, err = intstr.GetScaledValueFromIntOrPercent(comp.Replicas, int(spec.Replicas), false)
		if err != nil {
			return errors.Wrapf(err, "failed to get scaled value for replicas of component %s", comp.Name)
		}
	}
	if replicas != 0 && replicas != int(spec.Replicas) {
		return fmt.Errorf("partially rollout with the inplace strategy not supported, component: %s", comp.Name)
	}

	if len(comp.ServiceVersion) > 0 && comp.ServiceVersion != spec.ServiceVersion {
		spec.ServiceVersion = comp.ServiceVersion
		spec.ComponentDef = comp.CompDef
	}
	// the case that only upgrade the component definition
	if len(comp.CompDef) > 0 && comp.CompDef != spec.ComponentDef {
		spec.ComponentDef = comp.CompDef
	}
	return nil
}
