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
	"reflect"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

const (
	rolloutNameClusterLabel = "apps.kubeblocks.io/rollout-name"
)

type rolloutSetupTransformer struct{}

var _ graph.Transformer = &rolloutSetupTransformer{}

func (t *rolloutSetupTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) || isRolloutSucceed(transCtx.RolloutOrig) {
		return nil
	}

	var (
		rollout = transCtx.Rollout
	)

	// pre-check
	if err := t.precheck(rollout); err != nil {
		return err
	}

	// check and add the rollout label to the cluster
	if err := t.patchClusterLabel(transCtx, dag, rollout); err != nil {
		return err
	}

	// init the rollout status
	return t.initRolloutStatus(transCtx, dag, rollout)
}

func (t *rolloutSetupTransformer) precheck(rollout *appsv1alpha1.Rollout) error {
	if err := t.compPrecheck(rollout); err != nil {
		return err
	}
	return t.shardingPrecheck(rollout)
}

func (t *rolloutSetupTransformer) compPrecheck(rollout *appsv1alpha1.Rollout) error {
	for _, comp := range rollout.Spec.Components {
		// target serviceVersion & componentDef
		if comp.ServiceVersion == nil && comp.CompDef == nil {
			return fmt.Errorf("neither serviceVersion nor compDef is defined for component %s", comp.Name)
		}
		if err := t.checkRolloutStrategy("component", comp.Name, comp.Strategy); err != nil {
			return err
		}
	}
	return nil
}

func (t *rolloutSetupTransformer) shardingPrecheck(rollout *appsv1alpha1.Rollout) error {
	for _, sharding := range rollout.Spec.Shardings {
		// target shardingDef & serviceVersion & componentDef
		if sharding.ShardingDef == nil && sharding.ServiceVersion == nil && sharding.CompDef == nil {
			return fmt.Errorf("neither shardingDef, serviceVersion nor compDef is defined for sharding %s", sharding.Name)
		}
		if err := t.checkRolloutStrategy("sharding", sharding.Name, sharding.Strategy); err != nil {
			return err
		}
	}
	return nil
}

func (t *rolloutSetupTransformer) checkRolloutStrategy(kind, name string, strategy appsv1alpha1.RolloutStrategy) error {
	cnt := 0
	if strategy.Inplace != nil {
		cnt++
	}
	if strategy.Replace != nil {
		cnt++
	}
	if strategy.Create != nil {
		cnt++
	}
	if cnt == 0 {
		return fmt.Errorf("the rollout strategy of %s %s is not defined", kind, name)
	}
	if cnt > 1 {
		return fmt.Errorf("more than one rollout strategy is defined for %s %s", kind, name)
	}
	return nil
}

func (t *rolloutSetupTransformer) patchClusterLabel(transCtx *rolloutTransformContext, dag *graph.DAG, rollout *appsv1alpha1.Rollout) error {
	var (
		graphCli = transCtx.Client.(model.GraphClient)
		cluster  = transCtx.Cluster
	)
	if cluster.Labels == nil {
		cluster.Labels = make(map[string]string)
	}
	rolloutName, ok := cluster.Labels[rolloutNameClusterLabel]
	if ok && rolloutName != rollout.Name {
		errorMsg := fmt.Sprintf("the cluster %s is already bound to rollout %s", cluster.Name, rolloutName)
		rollout.Status.State = appsv1alpha1.ErrorRolloutState
		rollout.Status.Message = errorMsg
		graphCli.Status(dag, transCtx.RolloutOrig, rollout)
		return fmt.Errorf("%s", errorMsg)
	}
	if !ok {
		cluster.Labels[rolloutNameClusterLabel] = rollout.Name
	}
	if !reflect.DeepEqual(transCtx.ClusterOrig.Labels, cluster.Labels) {
		graphCli.Update(dag, transCtx.ClusterOrig, cluster)
		return graph.ErrPrematureStop
	}
	return nil
}

func (t *rolloutSetupTransformer) initRolloutStatus(transCtx *rolloutTransformContext, dag *graph.DAG, rollout *appsv1alpha1.Rollout) error {
	var (
		graphCli = transCtx.Client.(model.GraphClient)
	)
	for _, comp := range rollout.Spec.Components {
		if err := t.initCompStatus(transCtx, rollout, comp); err != nil {
			return err
		}
	}
	for _, sharding := range rollout.Spec.Shardings {
		if err := t.initShardingStatus(transCtx, rollout, sharding); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(transCtx.RolloutOrig.Status, rollout.Status) {
		graphCli.Status(dag, transCtx.RolloutOrig, rollout)
		return graph.ErrPrematureStop
	}
	return nil
}

func (t *rolloutSetupTransformer) initCompStatus(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	if spec == nil {
		return fmt.Errorf("the component %s is not found in cluster", comp.Name)
	}
	for _, status := range rollout.Status.Components {
		if status.Name == comp.Name {
			return nil // has been initialized
		}
	}
	rollout.Status.Components = append(rollout.Status.Components, appsv1alpha1.RolloutComponentStatus{
		Name:           comp.Name,
		ServiceVersion: spec.ServiceVersion,
		CompDef:        spec.ComponentDef,
		Replicas:       spec.Replicas,
	})
	return nil
}

func (t *rolloutSetupTransformer) initShardingStatus(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) error {
	spec := transCtx.ClusterShardings[sharding.Name]
	if spec == nil {
		return fmt.Errorf("the sharding %s is not found in cluster", sharding.Name)
	}
	for _, status := range rollout.Status.Shardings {
		if status.Name == sharding.Name {
			return nil // has been initialized
		}
	}
	rollout.Status.Shardings = append(rollout.Status.Shardings, appsv1alpha1.RolloutShardingStatus{
		Name:           sharding.Name,
		ShardingDef:    spec.ShardingDef,
		ServiceVersion: spec.Template.ServiceVersion,
		CompDef:        spec.Template.ComponentDef,
		Replicas:       spec.Template.Replicas,
	})
	return nil
}
