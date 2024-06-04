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
	"github.com/apecloud/kubeblocks/controllers/extensions"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
	if model.IsReconciliationPaused(cluster) {
		// set paused for all components
		compList, err := component.ListClusterComponents(transCtx.Context, transCtx.Client, cluster)
		if err != nil {
			return err
		}
		notPaused := false
		for _, comp := range compList {
			if !model.IsReconciliationPaused(&comp) {
				annotations := comp.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations[extensions.ControllerPaused] = trueVal
				comp.SetAnnotations(annotations)
				graphCli.Update(dag, nil, &comp)
				notPaused = true
			}

		}
		if notPaused {
			transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Paused",
				"cluster is paused")
		}
		return graph.ErrPrematureStop

	} else {
		// set resumed for all components
		compList := &appsv1alpha1.ComponentList{}
		labels := constant.GetClusterWellKnownLabels(cluster.Name)
		if err := transCtx.Client.List(transCtx.Context, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
			return err
		}
		hasPaused := false

		for _, comp := range compList.Items {
			if model.IsReconciliationPaused(&comp) {
				delete(comp.Annotations, extensions.ControllerPaused)
				hasPaused = true
				graphCli.Update(dag, nil, &comp)
			}
		}

		if hasPaused {
			transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Resumed",
				"cluster is resumed")
		}
		return nil
	}
}
