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
	"k8s.io/apimachinery/pkg/util/json"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type shardingRestoreTransformer struct{}

var _ graph.Transformer = &shardingRestoreTransformer{}

func (c *shardingRestoreTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*shardingTransformContext)

	if model.IsObjectDeleting(transCtx.OrigCluster) || len(transCtx.Cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	restoreAnt := transCtx.Cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
	if len(restoreAnt) == 0 {
		return nil
	}
	backupMap := map[string]map[string]string{}
	err := json.Unmarshal([]byte(restoreAnt), &backupMap)
	if err != nil {
		return err
	}

	// when restoring a sharded cluster, it is essential to specify the 'sourceTarget' from which data should be restored for each sharded component.
	// to achieve this, we allocate the source target for each component using annotations.
	for i := range transCtx.Cluster.Spec.ShardingSpecs {
		shardingSpec := transCtx.Cluster.Spec.ShardingSpecs[i]
		backupSource, ok := backupMap[shardingSpec.Name]
		if !ok {
			continue
		}
		shardingComps, err := intctrlutil.ListShardingComponents(transCtx.Context, transCtx.Client, transCtx.Cluster, shardingSpec.Name)
		if err != nil {
			return err
		}
		backup, err := plan.GetBackupFromClusterAnnotation(transCtx.Context, transCtx.Client, backupSource, shardingSpec.Name, transCtx.Cluster.Namespace)
		if err != nil {
			return err
		}
		if len(backup.Status.Targets) != int(shardingSpec.Shards) || len(backup.Status.Targets) != len(shardingComps) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed,
				`the source targets count of the backup "%s" must be equal to the count of the shard components "%s"`,
				backup.Name, shardingSpec.Name)
		}

		// obtain components that have already been assigned targets.
		allocateTargetMap := map[string]string{}
		restoreDoneForShardComponents := true
		for _, shardingComp := range shardingComps {
			if model.IsObjectDeleting(&shardingComp) {
				continue
			}
			if shardingComp.Annotations[constant.RestoreDoneAnnotationKey] != "true" {
				restoreDoneForShardComponents = false
			}
			if targetName, ok := shardingComp.Annotations[constant.BackupSourceTargetAnnotationKey]; ok {
				compName := shardingComp.Labels[constant.KBAppComponentLabelKey]
				allocateTargetMap[targetName] = compName
			}
		}
		if len(allocateTargetMap) == len(backup.Status.Targets) {
			// check if the restore is completed when all source target have allocated.
			if err = c.cleanupRestoreAnnotationForSharding(transCtx, dag, shardingSpec.Name, restoreDoneForShardComponents); err != nil {
				return err
			}
			continue
		}

		// allocate source target for sharding components by index mapping to the target
		for index, target := range backup.Status.Targets {
			if _, ok = allocateTargetMap[target.Name]; ok {
				continue
			}
			shardingComp := shardingComps[index]
			if shardingComp.Annotations == nil {
				shardingComp.Annotations = map[string]string{}
			}
			if _, ok = shardingComp.Annotations[constant.BackupSourceTargetAnnotationKey]; ok {
				continue
			}
			shardingCompCopy := shardingComp.DeepCopy()
			shardingCompCopy.Annotations[constant.BackupSourceTargetAnnotationKey] = target.Name
			graphCli.Patch(dag, &shardingComp, shardingCompCopy, &model.ReplaceIfExistingOption{})
		}
	}
	return nil
}

func (c *shardingRestoreTransformer) cleanupRestoreAnnotationForSharding(transCtx *shardingTransformContext,
	dag *graph.DAG, shardName string, restoreDoneForShardComponents bool) error {
	if transCtx.Cluster.Status.Phase != appsv1.RunningClusterPhase {
		return nil
	}
	if !restoreDoneForShardComponents {
		return nil
	}
	needCleanup, err := plan.CleanupClusterRestoreAnnotation(transCtx.Cluster, shardName)
	if err != nil {
		return err
	}
	if needCleanup {
		graphCli, _ := transCtx.Client.(model.GraphClient)
		graphCli.Patch(dag, transCtx.OrigCluster, transCtx.Cluster, &model.ReplaceIfExistingOption{})
	}
	return nil
}
