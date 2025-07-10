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
	return t.shardings(transCtx)
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
		for i := range spec.Instances {
			if len(spec.Instances[i].ServiceVersion) > 0 || len(spec.Instances[i].CompDef) > 0 {
				spec.Instances[i].ServiceVersion = *comp.ServiceVersion
			}
		}
	}
	if comp.CompDef != nil {
		spec.ComponentDef = *comp.CompDef
		for i := range spec.Instances {
			if len(spec.Instances[i].ServiceVersion) > 0 || len(spec.Instances[i].CompDef) > 0 {
				spec.Instances[i].CompDef = *comp.CompDef
			}
		}
	}
	return nil
}

func (t *rolloutInplaceTransformer) shardings(transCtx *rolloutTransformContext) error {
	rollout := transCtx.Rollout
	for _, sharding := range rollout.Spec.Shardings {
		if sharding.Strategy.Inplace != nil {
			if err := t.sharding(transCtx, sharding); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutInplaceTransformer) sharding(transCtx *rolloutTransformContext, sharding appsv1alpha1.RolloutSharding) error {
	spec := transCtx.ClusterShardings[sharding.Name]
	shardingDef, serviceVersion, compDef := shardingDefNServiceVersionNCompDef(transCtx.Rollout, sharding, spec)
	if shardingDef != spec.ShardingDef || serviceVersion != spec.Template.ServiceVersion || compDef != spec.Template.ComponentDef {
		return nil
	}
	// TODO: how about the target sharding definition, service version and component definition are same with the original ones?

	if !checkClusterNShardingRunning(transCtx, sharding.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the sharding %s is not ready", sharding.Name))
	}

	if sharding.ShardingDef != nil {
		spec.ShardingDef = *sharding.ShardingDef
		for i, tpl := range spec.ShardTemplates {
			if tpl.ShardingDef != nil {
				spec.ShardTemplates[i].ShardingDef = sharding.ShardingDef
			}
		}
	}
	if sharding.ServiceVersion != nil {
		spec.Template.ServiceVersion = *sharding.ServiceVersion
		for i := range spec.Template.Instances {
			if len(spec.Template.Instances[i].ServiceVersion) > 0 || len(spec.Template.Instances[i].CompDef) > 0 {
				spec.Template.Instances[i].ServiceVersion = *sharding.ServiceVersion
			}
		}
		for i, tpl := range spec.ShardTemplates {
			if tpl.ServiceVersion != nil || tpl.CompDef != nil {
				spec.ShardTemplates[i].ServiceVersion = sharding.ServiceVersion
			}
		}
	}
	if sharding.CompDef != nil {
		spec.Template.ComponentDef = *sharding.CompDef
		for i := range spec.Template.Instances {
			if len(spec.Template.Instances[i].ServiceVersion) > 0 || len(spec.Template.Instances[i].CompDef) > 0 {
				spec.Template.Instances[i].CompDef = *sharding.CompDef
			}
		}
		for i, tpl := range spec.ShardTemplates {
			if tpl.ServiceVersion != nil || tpl.CompDef != nil {
				spec.ShardTemplates[i].CompDef = sharding.CompDef
			}
		}
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

func shardingDefNServiceVersionNCompDef(rollout *appsv1alpha1.Rollout,
	sharding appsv1alpha1.RolloutSharding, spec *appsv1.ClusterSharding) (string, string, string) {
	shardingDef, serviceVer, compDef := spec.ShardingDef, spec.Template.ServiceVersion, spec.Template.ComponentDef
	for _, status := range rollout.Status.Shardings {
		if status.Name == sharding.Name {
			shardingDef = status.ShardingDef
			serviceVer = status.ServiceVersion
			compDef = status.CompDef
			break
		}
	}
	return shardingDef, serviceVer, compDef
}
