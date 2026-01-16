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

	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type rolloutTearDownTransformer struct{}

var _ graph.Transformer = &rolloutTearDownTransformer{}

func (t *rolloutTearDownTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) || isRolloutSucceed(transCtx.RolloutOrig) {
		return nil
	}
	return t.tearDown(transCtx)
}

func (t *rolloutTearDownTransformer) tearDown(transCtx *rolloutTransformContext) error {
	if err := t.components(transCtx); err != nil {
		return err
	}
	return t.shardings(transCtx)
}

func (t *rolloutTearDownTransformer) components(transCtx *rolloutTransformContext) error {
	rollout := transCtx.Rollout
	for _, comp := range rollout.Spec.Components {
		if comp.Strategy.Inplace != nil {
			if err := t.compInplace(transCtx, rollout, comp); err != nil {
				return err
			}
		}
		if comp.Strategy.Replace != nil {
			if err := t.compReplace(transCtx, rollout, comp); err != nil {
				return err
			}
		}
		if comp.Strategy.Create != nil {
			if err := t.compCreate(transCtx, rollout, comp); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutTearDownTransformer) compInplace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	return nil // do nothing
}

func (t *rolloutTearDownTransformer) compReplace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	replicas, _, err := replaceCompReplicas(rollout, comp, spec)
	if err != nil {
		return err
	}
	tpls, _, err := replaceCompInstanceTemplates(rollout, comp, spec)
	if err != nil {
		return err
	}
	newReplicas := replaceInstanceTemplateReplicas(tpls)
	if newReplicas == replicas && spec.Replicas == replicas && checkClusterNCompRunning(transCtx, comp.Name) {
		tpl := tpls[""] // use the default template
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
		spec.Instances = slices.DeleteFunc(spec.Instances, func(tpl appsv1.InstanceTemplate) bool {
			if ptr.Deref(tpl.Replicas, 0) > 0 {
				return false
			}
			_, ok := tpl.Annotations[instanceTemplateCreatedByAnnotationKey]
			return ok
		})
		for i := range spec.Instances {
			spec.Instances[i].ServiceVersion = tpl.ServiceVersion
			spec.Instances[i].CompDef = tpl.CompDef
		}
	}
	return nil
}

func (t *rolloutTearDownTransformer) compCreate(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	// TODO: impl
	return createStrategyNotSupportedError
}

func (t *rolloutTearDownTransformer) shardings(transCtx *rolloutTransformContext) error {
	rollout := transCtx.Rollout
	for _, sharding := range rollout.Spec.Shardings {
		if sharding.Strategy.Inplace != nil {
			if err := t.shardingInplace(transCtx, rollout, sharding); err != nil {
				return err
			}
		}
		if sharding.Strategy.Replace != nil {
			if err := t.shardingReplace(transCtx, rollout, sharding); err != nil {
				return err
			}
		}
		if sharding.Strategy.Create != nil {
			if err := t.shardingCreate(transCtx, rollout, sharding); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutTearDownTransformer) shardingInplace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) error {
	return nil // do nothing
}

func (t *rolloutTearDownTransformer) shardingReplace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) error {
	spec := transCtx.ClusterShardings[sharding.Name]
	replicas := replaceShardingReplicas(rollout, sharding, spec)
	tpls, _, err := replaceShardingInstanceTemplates(rollout, sharding, spec)
	if err != nil {
		return err
	}
	newReplicas := replaceInstanceTemplateReplicas(tpls)
	if newReplicas == replicas && spec.Template.Replicas == replicas && checkClusterNShardingRunning(transCtx, sharding.Name) {
		tpl := tpls[""] // use the default template
		spec.Template.ServiceVersion = tpl.ServiceVersion
		spec.Template.ComponentDef = tpl.CompDef
		spec.Template.OfflineInstances = slices.DeleteFunc(spec.Template.OfflineInstances, func(instance string) bool {
			for _, status := range rollout.Status.Shardings {
				if status.Name == sharding.Name {
					return slices.Contains(status.ScaleDownInstances, instance)
				}
			}
			return false
		})
		spec.Template.Instances = slices.DeleteFunc(spec.Template.Instances, func(tpl appsv1.InstanceTemplate) bool {
			if ptr.Deref(tpl.Replicas, 0) > 0 {
				return false
			}
			_, ok := tpl.Annotations[instanceTemplateCreatedByAnnotationKey]
			return ok
		})
		for i := range spec.Template.Instances {
			spec.Template.Instances[i].ServiceVersion = tpl.ServiceVersion
			spec.Template.Instances[i].CompDef = tpl.CompDef
		}
	}
	return nil
}

func (t *rolloutTearDownTransformer) shardingCreate(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) error {
	// TODO: impl
	return createStrategyNotSupportedError
}
