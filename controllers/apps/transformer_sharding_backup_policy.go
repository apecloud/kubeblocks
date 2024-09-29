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
	"strings"

	"k8s.io/gengo/examples/set-gen/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
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
	backupPolicyTPLs, err := r.getBackupPolicyTemplates(transCtx)
	if err != nil {
		return err
	}

	bpCtx := newBackupPolicyCtx(transCtx.Context, transCtx.Client, transCtx.Logger,
		transCtx.EventRecorder, transCtx.OrigCluster, len(backupPolicyTPLs.Items))
	return r.reconcileBackupPolicyTemplates(dag, graphCli, bpCtx, backupPolicyTPLs)
}

// getBackupPolicyTemplates gets the backupPolicyTemplate for the cluster.
func (r *shardingBackupPolicyTransformer) getBackupPolicyTemplates(transCtx *shardingTransformContext) (*appsv1alpha1.BackupPolicyTemplateList, error) {
	backupPolicyTPLs := &appsv1alpha1.BackupPolicyTemplateList{}
	tplMap := map[string]sets.Empty{}
	for _, v := range transCtx.ComponentDefs {
		tmpTPLs := &appsv1alpha1.BackupPolicyTemplateList{}
		// TODO: prefix match for componentDef name?
		if err := transCtx.Client.List(transCtx.Context, tmpTPLs, client.MatchingLabels{v.Name: v.Name}); err != nil {
			return nil, err
		}
		for i := range tmpTPLs.Items {
			if _, ok := tplMap[tmpTPLs.Items[i].Name]; !ok {
				backupPolicyTPLs.Items = append(backupPolicyTPLs.Items, tmpTPLs.Items[i])
				tplMap[tmpTPLs.Items[i].Name] = sets.Empty{}
			}
		}
	}
	return backupPolicyTPLs, nil
}

func (r *shardingBackupPolicyTransformer) reconcileBackupPolicyTemplates(dag *graph.DAG, graphCli model.GraphClient, backupPolicyCtx *backupPolicyCtx,
	bptList *appsv1alpha1.BackupPolicyTemplateList) error {
	backupPolicyMap := map[string]struct{}{}
	backupScheduleMap := map[string]struct{}{}
	for _, tpl := range bptList.Items {
		backupPolicyCtx.isDefaultTemplate = tpl.Annotations[dptypes.DefaultBackupPolicyTemplateAnnotationKey]
		backupPolicyCtx.tplIdentifier = tpl.Spec.Identifier
		backupPolicyCtx.backupPolicyTpl = &tpl
		for i := range tpl.Spec.BackupPolicies {
			backupPolicyCtx.backupPolicy = &tpl.Spec.BackupPolicies[i]
			compItems := r.getClusterComponentItems(backupPolicyCtx)
			if err := reconcileBackupPolicyTemplate(dag, graphCli, backupPolicyCtx, compItems, backupPolicyMap, backupScheduleMap); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *shardingBackupPolicyTransformer) getClusterComponentItems(backupPolicyCtx *backupPolicyCtx) []componentItem {
	matchedCompDef := func(compSpec appsv1.ClusterComponentSpec) bool {
		// TODO: support to create bp when using cluster topology and componentDef is empty
		if len(compSpec.ComponentDef) == 0 {
			return false
		}
		for _, compDef := range backupPolicyCtx.backupPolicy.ComponentDefs {
			if strings.HasPrefix(compSpec.ComponentDef, compDef) || strings.HasPrefix(compDef, compSpec.ComponentDef) {
				return true
			}
		}
		return false
	}
	var compSpecItems []componentItem
	for i, v := range backupPolicyCtx.cluster.Spec.ShardingSpecs {
		shardComponents, _ := intctrlutil.ListShardingComponents(backupPolicyCtx.ctx, backupPolicyCtx.cli, backupPolicyCtx.cluster, v.Name)
		if len(shardComponents) == 0 {
			// waiting for sharding component to be created
			continue
		}
		if matchedCompDef(v.Template) {
			compSpecItems = append(compSpecItems, componentItem{
				compSpec:      &backupPolicyCtx.cluster.Spec.ShardingSpecs[i].Template,
				componentName: v.Name,
				isSharding:    true,
			})
		}
	}
	return compSpecItems
}
