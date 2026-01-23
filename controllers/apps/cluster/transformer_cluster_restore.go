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
	"sort"

	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
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
				`the source targets count of the backup "%s" must be equal to or less than the count of the shard components "%s"`,
				backup.Name, spec.Name)
		}
		shardComponents, err := sharding.ListShardingComponents(c.Context, c.Client, c.Cluster, spec.Name)
		if err != nil {
			return err
		}
		if int(spec.Shards) > len(backup.Status.Targets) && len(shardComponents) < int(spec.Shards) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRestoreFailed,
				`wait for all shard components of sharding "%s" to be created before restoring from backup "%s"`,
				backup.Name, spec.Name)
		}
		targets := backup.Status.Targets
		// obtain components that have already been assigned targets.
		allocateTargetMap := map[string]string{}
		restoreDoneForShardComponents := true
		for _, v := range shardComponents {
			if model.IsObjectDeleting(&v) {
				continue
			}

			compName := v.Labels[constant.KBAppComponentLabelKey]
			compAnnotations := c.initClusterAnnotations(compName)

			if v.Annotations[constant.BackupSourceTargetAnnotationKey] != "" && v.Annotations[constant.RestoreDoneAnnotationKey] != "true" {
				restoreDoneForShardComponents = false
			}
			if targetName, ok := v.Annotations[constant.BackupSourceTargetAnnotationKey]; ok {
				allocateTargetMap[targetName] = compName
				compAnnotations[constant.BackupSourceTargetAnnotationKey] = targetName
			}
		}
		if len(allocateTargetMap) == len(targets) {
			// check if the restore is completed when all source target have allocated.
			if err = c.cleanupRestoreAnnotationForSharding(dag, spec.Name, restoreDoneForShardComponents); err != nil {
				return err
			}
		}
		// guarantee that when available targets are fewer than shards, the first shards are prioritized for restore.
		sort.Slice(c.shardingComps[spec.Name], func(i, j int) bool {
			return c.shardingComps[spec.Name][i].Name < c.shardingComps[spec.Name][j].Name
		})
		for _, target := range targets {
			if _, ok = allocateTargetMap[target.Name]; ok {
				continue
			}
			for _, compSpec := range c.shardingComps[spec.Name] {
				compAnnotations := c.initClusterAnnotations(compSpec.Name)
				if _, ok = compAnnotations[constant.BackupSourceTargetAnnotationKey]; ok {
					continue
				}
				compAnnotations[constant.BackupSourceTargetAnnotationKey] = target.Name
				break
			}
		}
		for _, compSpec := range c.shardingComps[spec.Name] {
			compAnnotations := c.initClusterAnnotations(compSpec.Name)
			if compAnnotations[constant.BackupSourceTargetAnnotationKey] == "" {
				compAnnotations[constant.SkipRestoreAnnotationKey] = "true"
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

func (c *clusterRestoreTransformer) initClusterAnnotations(compName string) map[string]string {
	if c.annotations == nil {
		c.annotations = make(map[string]map[string]string)
	}
	if c.annotations[compName] == nil {
		c.annotations[compName] = make(map[string]string)
	}
	return c.annotations[compName]
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
