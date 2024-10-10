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
)

type clusterRestoreTransformer struct {
	*clusterTransformContext
}

var _ graph.Transformer = &clusterRestoreTransformer{}

func (c *clusterRestoreTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	c.clusterTransformContext = ctx.(*clusterTransformContext)
	restoreAnt := c.Cluster.Annotations[constant.RestoreFromBackupAnnotationKey]
	if len(restoreAnt) == 0 {
		return nil
	}

	backupMap := map[string]map[string]string{}
	err := json.Unmarshal([]byte(restoreAnt), &backupMap)
	if err != nil {
		return err
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

func (c *clusterRestoreTransformer) annotateComponent(dag *graph.DAG, compObj *appsv1.Component) {
	// annotate component to reconcile for postReady restore.
	compObj.Labels[constant.ReconcileAnnotationKey] = "DoPostReadyRestore"
	graphCli, _ := c.Client.(model.GraphClient)
	graphCli.Update(dag, nil, compObj)
}
