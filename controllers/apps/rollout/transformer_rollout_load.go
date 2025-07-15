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
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

const (
	componentNotReadyRequeueDuration = 10 * time.Second
	infiniteDelayRequeueDuration     = 3600 * time.Second
)

type rolloutLoadTransformer struct{}

var _ graph.Transformer = &rolloutLoadTransformer{}

func (t *rolloutLoadTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) || isRolloutSucceed(transCtx.RolloutOrig) {
		return nil
	}

	var err error
	rollout := transCtx.Rollout
	transCtx.Cluster, err = t.getNCheckCluster(transCtx.Context, transCtx.Client, rollout)
	if err != nil {
		return err
	}
	transCtx.ClusterOrig = transCtx.Cluster.DeepCopy()
	transCtx.ClusterComps, transCtx.ClusterShardings = t.clusterCompNSharding(transCtx.Cluster)

	transCtx.Components, transCtx.ShardingComps, err = t.getNCheckComponents(transCtx.Context, transCtx.Client, rollout)
	if err != nil {
		return err
	}
	return nil
}

func (t *rolloutLoadTransformer) getNCheckCluster(ctx context.Context, cli client.Reader, rollout *appsv1alpha1.Rollout) (*appsv1.Cluster, error) {
	clusterKey := types.NamespacedName{
		Namespace: rollout.Namespace,
		Name:      rollout.Spec.ClusterName,
	}
	cluster := &appsv1.Cluster{}
	if err := cli.Get(ctx, clusterKey, cluster); err != nil {
		return nil, err
	}
	// TODO: check cluster status
	return cluster, nil
}

func (t *rolloutLoadTransformer) clusterCompNSharding(cluster *appsv1.Cluster) (map[string]*appsv1.ClusterComponentSpec, map[string]*appsv1.ClusterSharding) {
	comps := make(map[string]*appsv1.ClusterComponentSpec)
	for i, spec := range cluster.Spec.ComponentSpecs {
		comps[spec.Name] = &cluster.Spec.ComponentSpecs[i]
	}
	shardings := make(map[string]*appsv1.ClusterSharding)
	for i, spec := range cluster.Spec.Shardings {
		shardings[spec.Name] = &cluster.Spec.Shardings[i]
	}
	return comps, shardings
}

func (t *rolloutLoadTransformer) getNCheckComponents(ctx context.Context, cli client.Reader,
	rollout *appsv1alpha1.Rollout) (map[string]*appsv1.Component, map[string][]*appsv1.Component, error) {
	if len(rollout.Spec.Components) == 0 && len(rollout.Spec.Shardings) == 0 {
		return nil, nil, nil
	}
	comps := make(map[string]*appsv1.Component)
	for _, comp := range rollout.Spec.Components {
		obj, err := t.getNCheckComponent(ctx, cli, rollout, comp.Name)
		if err != nil {
			return nil, nil, err
		}
		comps[comp.Name] = obj
	}
	shardingComps := make(map[string][]*appsv1.Component)
	for _, sharding := range rollout.Spec.Shardings {
		objs, err := t.getNCheckShardingComps(ctx, cli, rollout, sharding.Name)
		if err != nil {
			return nil, nil, err
		}
		shardingComps[sharding.Name] = objs
	}
	return comps, shardingComps, nil
}

func (t *rolloutLoadTransformer) getNCheckComponent(ctx context.Context, cli client.Reader, rollout *appsv1alpha1.Rollout, compName string) (*appsv1.Component, error) {
	compKey := types.NamespacedName{
		Namespace: rollout.Namespace,
		Name:      constant.GenerateClusterComponentName(rollout.Spec.ClusterName, compName),
	}
	comp := &appsv1.Component{}
	if err := cli.Get(ctx, compKey, comp); err != nil {
		return nil, err
	}
	// TODO: check component status
	return comp, nil
}

func (t *rolloutLoadTransformer) getNCheckShardingComps(ctx context.Context, cli client.Reader, rollout *appsv1alpha1.Rollout, shardingName string) ([]*appsv1.Component, error) {
	compList := &appsv1.ComponentList{}
	opts := []client.ListOption{
		client.InNamespace(rollout.Namespace),
		client.MatchingLabels(constant.GetClusterLabels(rollout.Spec.ClusterName, map[string]string{
			constant.KBAppShardingNameLabelKey: shardingName,
		})),
	}
	if err := cli.List(ctx, compList, opts...); err != nil {
		return nil, err
	}
	shardingComps := make([]*appsv1.Component, 0)
	for i := range compList.Items {
		shardingComps = append(shardingComps, &compList.Items[i])
	}
	// TODO: check component status
	return shardingComps, nil
}

func checkClusterNCompRunning(transCtx *rolloutTransformContext, compName string) bool {
	cluster := transCtx.ClusterOrig
	compStatus := cluster.Status.Components[compName]
	if cluster.Generation != cluster.Status.ObservedGeneration || compStatus.Phase != appsv1.RunningComponentPhase {
		return false
	}
	compObj, ok := transCtx.Components[compName]
	if !ok || compObj == nil {
		return false
	}
	return compObj.Generation == compObj.Status.ObservedGeneration && compObj.Status.Phase == appsv1.RunningComponentPhase
}

func checkClusterNShardingRunning(transCtx *rolloutTransformContext, shardingName string) bool {
	cluster := transCtx.ClusterOrig
	status := cluster.Status.Shardings[shardingName]
	if cluster.Generation != cluster.Status.ObservedGeneration || status.Phase != appsv1.RunningComponentPhase {
		return false
	}
	compObjs, ok := transCtx.ShardingComps[shardingName]
	if !ok {
		return false
	}
	for _, compObj := range compObjs {
		if compObj.Generation != compObj.Status.ObservedGeneration || compObj.Status.Phase != appsv1.RunningComponentPhase {
			return false
		}
	}
	spec := transCtx.ClusterShardings[shardingName]
	return len(compObjs) == int(spec.Shards)
}
