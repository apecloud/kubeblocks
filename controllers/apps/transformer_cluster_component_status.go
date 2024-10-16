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

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
	comps, shardingComps, err := t.listClusterComponents(transCtx)
	if err != nil {
		return err
	}

	t.transformCompStatus(transCtx, comps)
	t.transformShardingStatus(transCtx, shardingComps)

	return nil
}

func (t *clusterComponentStatusTransformer) listClusterComponents(
	transCtx *clusterTransformContext) (map[string]*appsv1.Component, map[string][]*appsv1.Component, error) {
	var (
		cluster = transCtx.Cluster
	)

	compList := &appsv1.ComponentList{}
	ml := client.MatchingLabels(constant.GetClusterLabels(cluster.Name))
	if err := transCtx.Client.List(transCtx.Context, compList, client.InNamespace(cluster.Namespace), ml); err != nil {
		return nil, nil, err
	}

	if len(compList.Items) == 0 {
		return nil, nil, nil
	}

	comps := make(map[string]*appsv1.Component)
	shardingComps := make(map[string][]*appsv1.Component)

	sharding := func(comp *appsv1.Component) bool {
		shardingName := shardingCompNName(comp)
		if len(shardingName) == 0 {
			return false
		}

		if _, ok := shardingComps[shardingName]; !ok {
			shardingComps[shardingName] = []*appsv1.Component{comp}
		} else {
			shardingComps[shardingName] = append(shardingComps[shardingName], comp)
		}
		return true
	}

	for i, comp := range compList.Items {
		if sharding(&compList.Items[i]) {
			continue
		}
		compName, err := component.ShortName(cluster.Name, comp.Name)
		if err != nil {
			return nil, nil, err
		}
		if _, ok := comps[compName]; ok {
			return nil, nil, fmt.Errorf("duplicate component name: %s", compName)
		}
		comps[compName] = &compList.Items[i]
	}
	return comps, shardingComps, nil
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
		}
	}
	for name := range deleteSet {
		cluster.Status.Components[name] = appsv1.ClusterComponentStatus{
			Phase: appsv1.DeletingClusterCompPhase,
			Message: map[string]string{
				"reason": "the component is under deleting",
			},
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
	if string(status.Phase) != string(comp.Status.Phase) {
		status.Phase = comp.Status.Phase
		status.Message = comp.Status.Message
	}

	if phase != status.Phase {
		msg := clusterCompNShardingPhaseTransitionMsg("component", compName, status.Phase)
		if transCtx.GetRecorder() != nil && msg != "" {
			transCtx.GetRecorder().Eventf(transCtx.Cluster, corev1.EventTypeNormal, componentPhaseTransition, msg)
		}
		transCtx.GetLogger().Info(fmt.Sprintf("cluster component phase transition: %s -> %s (%s)", phase, status.Phase, msg))
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

	// reset the status
	cluster.Status.Shardings = make(map[string]appsv1.ClusterComponentStatus)
	for name := range createSet {
		cluster.Status.Shardings[name] = appsv1.ClusterComponentStatus{
			Phase: "",
			Message: map[string]string{
				"reason": "the sharding to be created",
			},
		}
	}
	for name := range deleteSet {
		cluster.Status.Shardings[name] = appsv1.ClusterComponentStatus{
			Phase: appsv1.DeletingClusterCompPhase,
			Message: map[string]string{
				"reason": "the sharding is under deleting",
			},
		}
	}
	for name := range updateSet {
		cluster.Status.Shardings[name] = t.buildClusterShardingStatus(transCtx, name, shardingComps[name])
	}
}

func (t *clusterComponentStatusTransformer) buildClusterShardingStatus(transCtx *clusterTransformContext,
	shardingName string, comps []*appsv1.Component) appsv1.ClusterComponentStatus {
	var (
		cluster = transCtx.Cluster
		status  = cluster.Status.Shardings[shardingName]
	)

	phase := status.Phase
	newPhase, newMessage := t.shardingPhaseNMessage(comps)
	if status.Phase != newPhase {
		status.Phase = newPhase
		status.Message = newMessage
	}

	if phase != status.Phase {
		msg := clusterCompNShardingPhaseTransitionMsg("sharding", shardingName, status.Phase)
		if transCtx.GetRecorder() != nil && msg != "" {
			transCtx.GetRecorder().Eventf(transCtx.Cluster, corev1.EventTypeNormal, componentPhaseTransition, msg)
		}
		transCtx.GetLogger().Info(fmt.Sprintf("cluster sharding phase transition: %s -> %s (%s)", phase, status.Phase, msg))
	}

	return status
}

func (t *clusterComponentStatusTransformer) shardingPhaseNMessage(comps []*appsv1.Component) (appsv1.ClusterComponentPhase, map[string]string) {
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
		// ???
		return "", map[string]string{"reason": "the component objects are not found"}
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
