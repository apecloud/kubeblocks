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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
	if c.Cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
		return nil
	}
	graphCli, _ := c.Client.(model.GraphClient)
	for compName, source := range backupMap {
		if source[constant.DoReadyRestoreAfterClusterRunning] != "true" {
			continue
		}
		compObjName := component.FullName(c.Cluster.Name, compName)
		compObj := &appsv1alpha1.Component{}
		if err = c.Client.Get(c.GetContext(), client.ObjectKey{Name: compObjName, Namespace: c.Cluster.Namespace}, compObj); err != nil {
			return err
		}
		// annotate component to reconcile for postReady restore.
		compObj.Labels[constant.ReconcileAnnotationKey] = "DoPostReadyRestore"
		graphCli.Update(dag, nil, compObj)
	}
	return nil
}
