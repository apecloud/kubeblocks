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
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	if err := t.transformComps(transCtx); err != nil {
		return err
	}
	return t.transformShardings(transCtx)
}

func (t *clusterComponentStatusTransformer) transformComps(transCtx *clusterTransformContext) error {
	var (
		cluster = transCtx.Cluster
	)
	if len(transCtx.components) == 0 {
		cluster.Status.Components = nil
		return nil
	}

	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]appsv1.ClusterComponentStatus)
	}
	for _, spec := range transCtx.components {
		compKey := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      component.FullName(cluster.Name, spec.Name),
		}
		comp := &appsv1.Component{}
		if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		cluster.Status.Components[spec.Name] = t.buildClusterCompStatus(transCtx, spec, comp)
	}
	return nil
}

func (t *clusterComponentStatusTransformer) buildClusterCompStatus(transCtx *clusterTransformContext,
	spec *appsv1.ClusterComponentSpec, comp *appsv1.Component) appsv1.ClusterComponentStatus {
	var (
		cluster = transCtx.Cluster
		status  = cluster.Status.Components[spec.Name]
	)

	phase := status.Phase
	if string(status.Phase) != string(comp.Status.Phase) {
		status.Phase = comp.Status.Phase
		status.Message = comp.Status.Message
	}

	if phase != status.Phase {
		msg := clusterCompNShardingPhaseTransitionMsg("component", spec.Name, status.Phase)
		if transCtx.GetRecorder() != nil && msg != "" {
			transCtx.GetRecorder().Eventf(transCtx.Cluster, corev1.EventTypeNormal, componentPhaseTransition, msg)
		}
		transCtx.GetLogger().Info(fmt.Sprintf("cluster component phase transition: %s -> %s (%s)", phase, status.Phase, msg))
	}

	return status
}

func (t *clusterComponentStatusTransformer) transformShardings(transCtx *clusterTransformContext) error {
	var (
		cluster = transCtx.Cluster
	)
	if len(transCtx.shardings) == 0 {
		cluster.Status.Shardings = nil
		return nil
	}

	if cluster.Status.Shardings == nil {
		cluster.Status.Shardings = make(map[string]appsv1.ClusterComponentStatus)
	}
	for _, sharding := range transCtx.shardings {
		comps, err := controllerutil.ListShardingComponents(transCtx.Context, transCtx.Client, cluster, sharding.Name)
		if err != nil {
			return err
		}
		cluster.Status.Shardings[sharding.Name] = t.buildClusterShardingStatus(transCtx, sharding, comps)
	}
	return nil
}

func (t *clusterComponentStatusTransformer) buildClusterShardingStatus(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, comps []appsv1.Component) appsv1.ClusterComponentStatus {
	var (
		cluster = transCtx.Cluster
		status  = cluster.Status.Shardings[sharding.Name]
	)

	phase := status.Phase
	newPhase, newMessage := t.shardingPhaseNMessage(comps)
	if status.Phase != newPhase {
		status.Phase = newPhase
		status.Message = newMessage
	}

	if phase != status.Phase {
		msg := clusterCompNShardingPhaseTransitionMsg("sharding", sharding.Name, status.Phase)
		if transCtx.GetRecorder() != nil && msg != "" {
			transCtx.GetRecorder().Eventf(transCtx.Cluster, corev1.EventTypeNormal, componentPhaseTransition, msg)
		}
		transCtx.GetLogger().Info(fmt.Sprintf("cluster sharding phase transition: %s -> %s (%s)", phase, status.Phase, msg))
	}

	return status
}

func (t *clusterComponentStatusTransformer) shardingPhaseNMessage(comps []appsv1.Component) (appsv1.ClusterComponentPhase, map[string]string) {
	statusList := make([]appsv1.ClusterComponentStatus, 0)
	phasedMessage := map[appsv1.ClusterComponentPhase]map[string]string{}
	for _, comp := range comps {
		phase := comp.Status.Phase
		message := comp.Status.Message
		if _, ok := phasedMessage[phase]; !ok {
			phasedMessage[phase] = message
		}
		statusList = append(statusList, appsv1.ClusterComponentStatus{Phase: phase})
	}
	if len(phasedMessage) == 0 {
		return "", nil
	}

	phase := appsv1.ClusterComponentPhase(composeClusterPhase(statusList))
	return phase, phasedMessage[phase]
}

func clusterCompNShardingPhaseTransitionMsg(kind, name string, phase appsv1.ClusterComponentPhase) string {
	if len(phase) == 0 {
		return ""
	}
	return fmt.Sprintf("cluster %s %s is %s", kind, name, phase)
}
