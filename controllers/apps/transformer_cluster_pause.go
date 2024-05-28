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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/extensions"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// clusterPauseTransformer handles cluster pause and resume
type clusterPauseTransformer struct {
	client.Client
}

var _ graph.Transformer = &clusterPauseTransformer{}

func (t *clusterPauseTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.OrigCluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	if checkPaused(cluster) {
		// set paused for all components
		compList := &appsv1alpha1.ComponentList{}
		labels := constant.GetClusterWellKnownLabels(cluster.Name)
		if err := transCtx.Client.List(transCtx.Context, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
			return err
		}
		notPaused := false
		for _, comp := range compList.Items {
			newComp := comp.DeepCopy()
			if !checkPaused(newComp) {
				annotations := comp.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations[extensions.ControllerPaused] = "true"
				newComp.SetAnnotations(annotations)
				graphCli.Update(dag, comp.DeepCopy(), newComp)
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

		// remove resumed annotation of configmaps
		var cmList, err = t.listConfigMaps(transCtx, cluster)
		if err != nil {
			return err
		}
		for _, cm := range cmList.Items {
			if val, ok := cm.GetAnnotations()[extensions.ControllerResumed]; ok && val == "true" {
				newCm := cm.DeepCopy()
				delete(newCm.Annotations, extensions.ControllerResumed)
				graphCli.Update(dag, cm.DeepCopy(), newCm)
			}
		}

		for _, comp := range compList.Items {
			newComp := comp.DeepCopy()
			if checkPaused(newComp) {
				delete(newComp.Annotations, extensions.ControllerPaused)
				hasPaused = true
				graphCli.Update(dag, comp.DeepCopy(), newComp)
			}
		}

		if hasPaused {
			// add annotations to all configmaps for reconfiguring after resumed
			var cmList, err = t.listConfigMaps(transCtx, cluster)
			if err != nil {
				return err
			}
			for _, cm := range cmList.Items {
				newCm := cm.DeepCopy()
				annotations := cm.GetAnnotations()
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations[extensions.ControllerResumed] = "true"
				newCm.SetAnnotations(annotations)
				graphCli.Update(dag, cm.DeepCopy(), newCm)
			}

			// add event
			transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, "Resumed",
				"cluster is resumed")
		}
		return nil
	}
}

func (t *clusterPauseTransformer) listConfigMaps(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) (*corev1.ConfigMapList, error) {
	cmList := &corev1.ConfigMapList{}
	ml := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppInstanceLabelKey:  cluster.Name,
	}
	listOpts := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(ml),
	}
	err := t.Client.List(transCtx, cmList, listOpts...)
	if err != nil {
		return nil, err
	}
	return cmList, nil
}
