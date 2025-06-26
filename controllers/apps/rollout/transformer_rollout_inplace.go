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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type rolloutInplaceTransformer struct{}

var _ graph.Transformer = &rolloutInplaceTransformer{}

func (t *rolloutInplaceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) || isRolloutSucceed(transCtx.RolloutOrig) {
		return nil
	}
	return t.rollout(transCtx)
}

func (t *rolloutInplaceTransformer) rollout(transCtx *rolloutTransformContext) error {
	if err := t.components(transCtx); err != nil {
		return err
	}
	// TODO: sharding
	return nil
}

func (t *rolloutInplaceTransformer) components(transCtx *rolloutTransformContext) error {
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
	var replicas int
	var err error
	spec := transCtx.ClusterComps[comp.Name]
	if comp.Replicas != nil {
		replicas, err = intstr.GetScaledValueFromIntOrPercent(comp.Replicas, int(spec.Replicas), false)
		if err != nil {
			return errors.Wrapf(err, "failed to get scaled value for replicas of component %s", comp.Name)
		}
	}
	if replicas != 0 && replicas != int(spec.Replicas) {
		return fmt.Errorf("partially rollout with the inplace strategy not supported, component: %s", comp.Name)
	}

	serviceVersion, compDef := serviceVersionNCompDef(transCtx.Rollout, comp, spec)
	if serviceVersion != spec.ServiceVersion || compDef != spec.ComponentDef {
		return nil
	}
	// TODO: how about the target service version and component definition are same with the original ones?

	if !checkClusterNCompRunning(transCtx, comp.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the component %s is not ready", comp.Name))
	}

	if comp.ServiceVersion != nil {
		spec.ServiceVersion = *comp.ServiceVersion
	}
	if comp.CompDef != nil {
		spec.ComponentDef = *comp.CompDef
	}
	return nil
}

// serviceVersionNCompDef obtains the original service version and component definition.
func serviceVersionNCompDef(rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) (string, string) {
	serviceVer, compDef := spec.ServiceVersion, spec.ComponentDef
	for _, status := range rollout.Status.Components {
		if status.Name == comp.Name {
			serviceVer = status.ServiceVersion
			compDef = status.CompDef
			break
		}
	}
	return serviceVer, compDef
}
