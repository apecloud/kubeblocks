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
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// clusterPauseTransformer handles cluster pause and resume
type clusterPauseTransformer struct {
}

var _ graph.Transformer = &clusterPauseTransformer{}

func (t *clusterPauseTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.OrigCluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	componentList, err := component.ListClusterComponents(transCtx.Context, transCtx.Client, cluster)
	if model.IsReconciliationPaused(cluster) {
		// set paused for all components
		if err != nil {
			return err
		}
		needPaused := false
		for i := range componentList {
			if comp, needUpdate := setPauseAnnotation(&componentList[i]); needUpdate {
				graphCli.Update(dag, nil, comp)
				needPaused = true
			}
		}
		if needPaused {
			transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Paused",
				"cluster is paused")
		}
		return graph.ErrPrematureStop

	} else {
		// set resumed for all components
		needResume := false
		for i := range componentList {
			if comp, needUpdate := removePauseAnnotation(&componentList[i]); needUpdate {
				graphCli.Update(dag, nil, comp)
				needResume = true
			}
		}
		if needResume {
			transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Resumed",
				"cluster is resumed")
		}
		return nil
	}
}
