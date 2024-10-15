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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// clusterComponentStatusTransformer transforms cluster components' status.
type clusterComponentStatusTransformer struct{}

var _ graph.Transformer = &clusterComponentStatusTransformer{}

func (t *clusterComponentStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	if len(transCtx.allComps) == 0 || !transCtx.OrigCluster.IsStatusUpdating() {
		return nil
	}
	return t.transform(transCtx)
}

func (t *clusterComponentStatusTransformer) transform(transCtx *clusterTransformContext) error {
	cluster := transCtx.Cluster
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]appsv1.ClusterComponentStatus)
	}
	for _, compSpec := range transCtx.allComps {
		compKey := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      component.FullName(cluster.Name, compSpec.Name),
		}
		comp := &appsv1.Component{}
		if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		cluster.Status.Components[compSpec.Name] = t.buildClusterCompStatus(transCtx, comp, compSpec.Name)
	}
	return nil
}

func (t *clusterComponentStatusTransformer) buildClusterCompStatus(transCtx *clusterTransformContext,
	comp *appsv1.Component, compName string) appsv1.ClusterComponentStatus {
	var (
		cluster = transCtx.Cluster
		status  = cluster.Status.Components[compName]
	)

	phase := status.Phase
	if string(status.Phase) != string(comp.Status.Phase) {
		status.Phase = comp.Status.Phase
		status.Message = comp.Status.Message
	}

	if phase != status.Phase {
		msg := clusterComponentPhaseTransitionMsg(compName, status.Phase)
		if transCtx.GetRecorder() != nil && msg != "" {
			transCtx.GetRecorder().Eventf(transCtx.Cluster, corev1.EventTypeNormal, componentPhaseTransition, msg)
		}
		transCtx.GetLogger().Info(fmt.Sprintf("cluster component phase transition: %s -> %s (%s)", phase, status.Phase, msg))
	}

	return status
}

func clusterComponentPhaseTransitionMsg(compName string, phase appsv1.ClusterComponentPhase) string {
	if len(phase) == 0 {
		return ""
	}
	return fmt.Sprintf("cluster component %s is %s", compName, phase)
}
