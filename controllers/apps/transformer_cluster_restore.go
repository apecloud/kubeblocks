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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type clusterRestoreTransformer struct {
	*clusterTransformContext
}

var _ graph.Transformer = &clusterRestoreTransformer{}

func (c *clusterRestoreTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	c.clusterTransformContext = ctx.(*clusterTransformContext)
	restoreAnt := c.Cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
	if restoreAnt == "" {
		return nil
	}
	backupMap := map[string]map[string]string{}
	err := json.Unmarshal([]byte(restoreAnt), &backupMap)
	if err != nil {
		return err
	}

	// when restoring a sharded cluster, it is essential to specify the 'sourceTarget' from which data should be restored for each sharded component.
	// to achieve this, we allocate the source target for each component using annotations.
	for i := range c.Cluster.Spec.Shardings {
		spec := c.Cluster.Spec.Shardings[i]
		backupSource, ok := backupMap[spec.Name]
		if !ok {
			continue
		}
		backup, err := plan.GetBackupFromClusterAnnotation(c.Context, c.Client, backupSource, spec.Name, c.Cluster.Namespace)
		if err != nil {
			return err
		}
		if len(backup.Status.Targets) > int(spec.Shards) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed,
				`the source targets count of the backup "%s" must be equal to or greater than the count of the shard components "%s"`,
				backup.Name, spec.Name)
		}
		shardComponents, err := intctrlutil.ListShardingComponents(c.Context, c.Client, c.Cluster, spec.Name)
		if err != nil {
			return err
		}
		// obtain components that have already been assigned targets.
		allocateTargetMap := map[string]string{}
		restoreDoneForShardComponents := true
		for _, v := range shardComponents {
			if model.IsObjectDeleting(&v) {
				continue
			}
			if v.Annotations[constant.RestoreDoneAnnotationKey] != "true" {
				restoreDoneForShardComponents = false
			}
			if targetName, ok := v.Annotations[constant.BackupSourceTargetAnnotationKey]; ok {
				compName := v.Labels[constant.KBAppComponentLabelKey]
				allocateTargetMap[targetName] = compName
				c.Annotations[compName][constant.BackupSourceTargetAnnotationKey] = targetName
			}
		}
		if len(allocateTargetMap) == len(backup.Status.Targets) {
			// check if the restore is completed when all source target have allocated.
			if err = c.cleanupRestoreAnnotationForSharding(dag, spec.Name, restoreDoneForShardComponents); err != nil {
				return err
			}
			continue
		}
		for _, target := range backup.Status.Targets {
			if _, ok = allocateTargetMap[target.Name]; ok {
				continue
			}
			for _, compSpec := range c.ShardingComponentSpecs[spec.Name] {
				if _, ok = c.Annotations[compSpec.Name][constant.BackupSourceTargetAnnotationKey]; ok {
					continue
				}
				c.Annotations[compSpec.Name][constant.BackupSourceTargetAnnotationKey] = target.Name
				break
			}
		}
	}
	// if component needs to do post ready restore after cluster is running, annotate component
	if c.Cluster.Status.Phase == appsv1.RunningClusterPhase {
		for _, compSpec := range c.Cluster.Spec.ComponentSpecs {
			backupSource, ok := backupMap[compSpec.Name]
			if !ok {
				continue
			}
			if backupSource[constant.DoReadyRestoreAfterClusterRunning] != "true" {
				continue
			}
			compObjName := component.FullName(c.Cluster.Name, compSpec.Name)
			compObj := &appsv1.Component{}
			if err = c.Client.Get(c.GetContext(), client.ObjectKey{Name: compObjName, Namespace: c.Cluster.Namespace}, compObj); err != nil {
				return err
			}
			// annotate component to reconcile for postReady restore.
			c.annotateComponent(dag, compObj)
		}
	}
	return nil
}

func (c *clusterRestoreTransformer) cleanupRestoreAnnotationForSharding(dag *graph.DAG,
	shardName string,
	restoreDoneForShardComponents bool) error {
	if c.Cluster.Status.Phase != appsv1.RunningClusterPhase {
		return nil
	}
	if !restoreDoneForShardComponents {
		return nil
	}
	needCleanup, err := plan.CleanupClusterRestoreAnnotation(c.Cluster, shardName)
	if err != nil {
		return err
	}
	if needCleanup {
		graphCli, _ := c.Client.(model.GraphClient)
		graphCli.Patch(dag, c.OrigCluster, c.Cluster, &model.ReplaceIfExistingOption{})
	}
	return nil
}

func (c *clusterRestoreTransformer) annotateComponent(dag *graph.DAG, compObj *appsv1.Component) {
	// annotate component to reconcile for postReady restore.
	compObj.Labels[constant.ReconcileAnnotationKey] = "DoPostReadyRestore"
	graphCli, _ := c.Client.(model.GraphClient)
	graphCli.Update(dag, nil, compObj)
}
