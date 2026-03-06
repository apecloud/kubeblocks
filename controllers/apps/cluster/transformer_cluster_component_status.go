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
	"fmt"
	"math"
	"strconv"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

const (
	// clusterCompPhaseTransition the event reason indicates that the cluster component transits to a new phase.
	clusterCompPhaseTransition = "ClusterComponentPhaseTransition"
)

// clusterComponentStatusTransformer transforms cluster components' status.
type clusterComponentStatusTransformer struct{}

var _ graph.Transformer = &clusterComponentStatusTransformer{}

func (t *clusterComponentStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if transCtx.OrigCluster.IsDeleting() {
		return nil
	}
	return t.transform(transCtx)
}

func (t *clusterComponentStatusTransformer) transform(transCtx *clusterTransformContext) error {
	comps, shardingComps, err := listClusterComponents(transCtx.Context, transCtx.Client, transCtx.Cluster)
	if err != nil {
		return err
	}

	t.transformCompStatus(transCtx, comps)
	t.transformShardingStatus(transCtx, shardingComps)

	return nil
}

func (t *clusterComponentStatusTransformer) transformCompStatus(transCtx *clusterTransformContext, comps map[string]*appsv1.Component) {
	var (
		cluster = transCtx.Cluster
	)

	if len(transCtx.components) == 0 && len(comps) == 0 {
		cluster.Status.Components = nil
		return
	}

	runningSet := sets.New[string]()
	if comps != nil {
		runningSet.Insert(maps.Keys(comps)...)
	}
	protoSet := sets.New[string]()
	for _, spec := range transCtx.components {
		protoSet.Insert(spec.Name)
	}
	createSet, deleteSet, updateSet := setDiff(runningSet, protoSet)

	// reset the status
	cluster.Status.Components = make(map[string]appsv1.ClusterComponentStatus)
	for name := range createSet {
		cluster.Status.Components[name] = appsv1.ClusterComponentStatus{
			Phase: "",
			Message: map[string]string{
				"reason": "the component to be created",
			},
			ObservedGeneration: cluster.Generation,
			UpToDate:           false,
		}
	}
	for name := range deleteSet {
		cluster.Status.Components[name] = appsv1.ClusterComponentStatus{
			Phase: appsv1.DeletingComponentPhase,
			Message: map[string]string{
				"reason": "the component is under deleting",
			},
			ObservedGeneration: cluster.Generation,
			UpToDate:           false,
		}
	}
	for name := range updateSet {
		cluster.Status.Components[name] = t.buildClusterCompStatus(transCtx, name, comps[name])
	}
}

func (t *clusterComponentStatusTransformer) buildClusterCompStatus(transCtx *clusterTransformContext,
	compName string, comp *appsv1.Component) appsv1.ClusterComponentStatus {
	var (
		cluster = transCtx.Cluster
		status  = cluster.Status.Components[compName]
	)

	phase := status.Phase
	status = t.clusterCompStatus(cluster, comp)

	if phase != status.Phase {
		msg := clusterCompNShardingPhaseTransitionMsg("component", compName, status.Phase)
		if transCtx.GetRecorder() != nil && msg != "" {
			transCtx.GetRecorder().Eventf(transCtx.Cluster, corev1.EventTypeNormal, clusterCompPhaseTransition, msg)
		}
		transCtx.GetLogger().Info(fmt.Sprintf("cluster component phase transition: %s -> %s (%s)", phase, status.Phase, msg))
	}

	return status
}

func (t *clusterComponentStatusTransformer) clusterCompStatus(cluster *appsv1.Cluster, comp *appsv1.Component) appsv1.ClusterComponentStatus {
	status := appsv1.ClusterComponentStatus{
		Phase:              comp.Status.Phase,
		Message:            comp.Status.Message,
		ObservedGeneration: 0,
		UpToDate:           false,
	}
	generation, ok := comp.Annotations[constant.KubeBlocksGenerationKey]
	if ok {
		ig, _ := strconv.ParseInt(generation, 10, 64)
		status.ObservedGeneration = ig
		status.UpToDate = comp.Generation == comp.Status.ObservedGeneration && ig == cluster.Generation
	}
	return status
}

func (t *clusterComponentStatusTransformer) transformShardingStatus(transCtx *clusterTransformContext, shardingComps map[string][]*appsv1.Component) {
	var (
		cluster = transCtx.Cluster
	)

	if len(transCtx.shardings) == 0 && len(shardingComps) == 0 {
		cluster.Status.Shardings = nil
		return
	}

	runningSet := sets.New[string]()
	if shardingComps != nil {
		runningSet.Insert(maps.Keys(shardingComps)...)
	}
	protoSet := sets.New[string]()
	for _, spec := range transCtx.shardings {
		protoSet.Insert(spec.Name)
	}
	createSet, deleteSet, updateSet := setDiff(runningSet, protoSet)

	shardingStatus := make(map[string]appsv1.ClusterShardingStatus)
	for name := range createSet {
		status := cluster.Status.Shardings[name]
		status.Phase = ""
		status.Message = map[string]string{
			"reason": "the sharding to be created",
		}
		status.ObservedGeneration = cluster.Generation
		status.UpToDate = false
		shardingStatus[name] = status
	}
	for name := range deleteSet {
		status := cluster.Status.Shardings[name]
		status.Phase = appsv1.DeletingComponentPhase
		status.Message = map[string]string{
			"reason": "the sharding is under deleting",
		}
		status.ObservedGeneration = cluster.Generation
		status.UpToDate = false
		shardingStatus[name] = status
	}
	for name := range updateSet {
		shardingStatus[name] = t.buildClusterShardingStatus(transCtx, name, shardingComps[name])
	}

	// reset the status
	cluster.Status.Shardings = shardingStatus
}

func (t *clusterComponentStatusTransformer) buildClusterShardingStatus(transCtx *clusterTransformContext,
	shardingName string, comps []*appsv1.Component) appsv1.ClusterShardingStatus {
	var (
		cluster   = transCtx.Cluster
		oldStatus = cluster.Status.Shardings[shardingName]
	)

	status := t.clusterShardingStatus(cluster, comps)

	if oldStatus.Phase != status.Phase {
		msg := clusterCompNShardingPhaseTransitionMsg("sharding", shardingName, status.Phase)
		if transCtx.GetRecorder() != nil && msg != "" {
			transCtx.GetRecorder().Eventf(transCtx.Cluster, corev1.EventTypeNormal, clusterCompPhaseTransition, msg)
		}
		transCtx.GetLogger().Info(fmt.Sprintf("cluster sharding phase transition: %s -> %s (%s)", oldStatus.Phase, status.Phase, msg))
	}

	status.ShardingDef = oldStatus.ShardingDef
	status.PostProvision = oldStatus.PostProvision
	status.PreTerminate = oldStatus.PreTerminate

	return status
}

func (t *clusterComponentStatusTransformer) clusterShardingStatus(cluster *appsv1.Cluster, comps []*appsv1.Component) appsv1.ClusterShardingStatus {
	var (
		statusList    = make([]appsv1.ClusterComponentStatus, 0)
		phasedMessage = map[appsv1.ComponentPhase]map[string]string{}
		generation    = int64(math.MaxInt64)
		upToDate      = true
	)
	for _, comp := range comps {
		status := t.clusterCompStatus(cluster, comp)
		statusList = append(statusList, status)
		if _, ok := phasedMessage[status.Phase]; !ok {
			phasedMessage[status.Phase] = status.Message
		}
		generation = min(status.ObservedGeneration, generation)
		upToDate = upToDate && status.UpToDate
	}
	if len(phasedMessage) == 0 {
		// ???
		return appsv1.ClusterShardingStatus{
			Phase:              "",
			Message:            map[string]string{"reason": "the component objects are not found"},
			ObservedGeneration: 0,
			UpToDate:           false,
		}
	}

	composedPhase := composeClusterPhase(statusList)
	if composedPhase == appsv1.AbnormalClusterPhase {
		composedPhase = appsv1.FailedClusterPhase
	}
	phase := appsv1.ComponentPhase(composedPhase)
	return appsv1.ClusterShardingStatus{
		Phase:              phase,
		Message:            phasedMessage[phase],
		ObservedGeneration: generation,
		UpToDate:           upToDate,
	}
}

func clusterCompNShardingPhaseTransitionMsg(kind, name string, phase appsv1.ComponentPhase) string {
	if len(phase) == 0 {
		return ""
	}
	return fmt.Sprintf("cluster %s %s is %s", kind, name, phase)
}
