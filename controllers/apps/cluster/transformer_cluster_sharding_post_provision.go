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

package cluster

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type clusterShardingPostProvisionTransformer struct{}

var _ graph.Transformer = &clusterShardingPostProvisionTransformer{}

func (t *clusterShardingPostProvisionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.OrigCluster
	if !cluster.IsDeleting() {
		return nil
	}

	if common.IsCompactMode(transCtx.Cluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create post provision related objects", "cluster", client.ObjectKeyFromObject(transCtx.Cluster))
		return nil
	}

	return t.reconcileShardingPostProvision(transCtx, dag)
}

func (t *clusterShardingPostProvisionTransformer) reconcileShardingPostProvision(transCtx *clusterTransformContext, dag *graph.DAG) error {
	for _, shard := range transCtx.shardings {
		shardDef, ok := transCtx.shardingDefs[shard.ShardingDef]
		if !ok {
			continue
		}

		if shardDef.Spec.LifecycleActions == nil || shardDef.Spec.LifecycleActions.PostProvision == nil {
			continue
		}

		runningComps, err := sharding.ListShardingComponents(transCtx.Context, transCtx.Client, transCtx.Cluster, shard.Name)
		if err != nil {
			return err
		}

		unfinishedComponents := checkPostProvisionDone(runningComps)
		if len(unfinishedComponents) == 0 {
			continue
		}

		finishedComponents, err := t.shardingPostProvision(transCtx, unfinishedComponents, shardDef.Spec.LifecycleActions)
		if err != nil {
			err = lifecycle.IgnoreNotDefined(err)
			if errors.Is(err, lifecycle.ErrPreconditionFailed) {
				err = fmt.Errorf("%w: %w", intctrlutil.NewDelayedRequeueError(time.Second*10, "wait for sharding lifecycle action precondition"), err)
			}
			return err
		}

		t.markShardingPostProvisionDone(transCtx, dag, finishedComponents, runningComps)
	}
	return nil
}

func checkPostProvisionDone(comps []v1.Component) []v1.Component {
	var unfinished []v1.Component
	for _, comp := range comps {
		if model.IsObjectDeleting(&comp) {
			continue
		}

		if comp.Annotations == nil {
			unfinished = append(unfinished, comp)
			continue
		}

		needPostProvision, ok := comp.Annotations[kbShardingPostProvisionKey]
		if ok && needPostProvision == "true" {
			unfinished = append(unfinished, comp)
		}
	}
	return unfinished
}

func (t *clusterShardingPostProvisionTransformer) shardingPostProvision(transCtx *clusterTransformContext, comps []v1.Component, lifecycleAction *v1.ShardingLifecycleActions) ([]string, error) {
	lfa, err := lifecycleAction4Sharding(transCtx, comps, lifecycleAction)
	if err != nil {
		return nil, err
	}
	return lfa.PostProvision(transCtx.Context, transCtx.Client, nil)
}

func (t *clusterShardingPostProvisionTransformer) markShardingPostProvisionDone(transCtx *clusterTransformContext, dag *graph.DAG, comps []string, runningComps []v1.Component) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	compsMap := make(map[string]*v1.Component)
	for _, comp := range runningComps {
		compsMap[comp.Name] = &comp
	}

	for _, comp := range comps {
		compCopyObj := compsMap[comp].DeepCopy()
		if compCopyObj.Annotations == nil {
			compCopyObj.Annotations = make(map[string]string)
		}

		_, ok := compCopyObj.Annotations[kbShardingPostProvisionKey]
		if !ok {
			return
		}

		compCopyObj.Annotations[kbShardingPostProvisionKey] = "false"
		graphCli.Update(dag, compsMap[comp], compCopyObj)
	}
}
