/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// shardingBackupPolicyTransformer transforms the backup policy template to the data protection backup policy and backup schedule.
type shardingBackupPolicyTransformer struct{}

var _ graph.Transformer = &shardingBackupPolicyTransformer{}

// Transform transforms the backup policy template to the backup policy and backup schedule.
func (r *shardingBackupPolicyTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*shardingTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) || len(transCtx.Cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}

	if common.IsCompactMode(transCtx.OrigCluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create backup related objects",
			"cluster", client.ObjectKeyFromObject(transCtx.OrigCluster))
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	for i := range transCtx.Cluster.Spec.ShardingSpecs {
		shardingSpec := transCtx.Cluster.Spec.ShardingSpecs[i]
		compDef := transCtx.ComponentDefs[shardingSpec.Template.ComponentDef]
		if err := reconcileBackupPolicyAndSchedule(transCtx.Context, transCtx.Client, transCtx.EventRecorder, transCtx.Logger,
			dag, graphCli, transCtx.Cluster, &shardingSpec.Template, compDef, shardingSpec.Name, true); err != nil {
			return err
		}
	}
	return nil
}
