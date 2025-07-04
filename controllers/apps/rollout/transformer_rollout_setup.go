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
	for _, comp := range rollout.Spec.Components {
		if err := t.precheck(comp); err != nil {
			return err
		}
	}

	// check and add the rollout label to the cluster
	if err := t.patchClusterLabel(transCtx, dag, rollout); err != nil {
		return err
	}

	// init the rollout component status
	return t.initRolloutStatus(transCtx, dag, rollout)
}

func (t *rolloutSetupTransformer) precheck(comp appsv1alpha1.RolloutComponent) error {
	// target serviceVersion & componentDef
	if comp.ServiceVersion == nil && comp.CompDef == nil {
		return fmt.Errorf("neither serviceVersion nor compDef is defined for component %s", comp.Name)
	}

	// rollout strategy
	cnt := 0
	if comp.Strategy.Inplace != nil {
		cnt++
	}
	if comp.Strategy.Replace != nil {
		cnt++
	}
	if comp.Strategy.Create != nil {
		cnt++
	}
	if cnt == 0 {
		return fmt.Errorf("the rollout strategy of component %s is not defined", comp.Name)
	}
	if cnt > 1 {
		return fmt.Errorf("more than one rollout strategy is defined for component %s", comp.Name)
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
