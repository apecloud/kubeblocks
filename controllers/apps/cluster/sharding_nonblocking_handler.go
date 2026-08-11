/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

KubeBlocks is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

KubeBlocks is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with KubeBlocks.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"fmt"
	"slices"
	"time"

	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type nonBlockingShardingHandler struct {
	shardingHandler *clusterShardingHandler
	transCtx        *clusterTransformContext
	dag             *graph.DAG
	shardingName    string
	actions         *appsv1.ShardingLifecycleActions
	runningComps    map[string]*appsv1.Component
	protoComps      map[string]*appsv1.Component
	toCreate        sets.Set[string]
	toDelete        sets.Set[string]
	toUpdate        sets.Set[string]
}

func newNonBlockingShardingHandler(shardingHandler *clusterShardingHandler,
	transCtx *clusterTransformContext, dag *graph.DAG, shardingName string,
	runningComps, protoComps map[string]*appsv1.Component,
	toCreate, toDelete, toUpdate sets.Set[string]) *nonBlockingShardingHandler {
	shardingDef := shardingHandler.shardingDef(transCtx, shardingName)
	var actions *appsv1.ShardingLifecycleActions
	if shardingDef != nil {
		actions = shardingDef.Spec.LifecycleActions
	}
	enabled := actions != nil &&
		((actions.ShardAdd != nil && actions.ShardAdd.NonBlocking) ||
			(actions.ShardRemove != nil && actions.ShardRemove.NonBlocking))
	if !enabled {
		enabled = slices.ContainsFunc(maps.Values(runningComps), hasPendingNonBlockingAction)
	}
	if !enabled {
		return nil
	}
	return &nonBlockingShardingHandler{
		shardingHandler: shardingHandler,
		transCtx:        transCtx,
		dag:             dag,
		shardingName:    shardingName,
		actions:         actions,
		runningComps:    runningComps,
		protoComps:      protoComps,
		toCreate:        toCreate,
		toDelete:        toDelete,
		toUpdate:        toUpdate,
	}
}

func (h *nonBlockingShardingHandler) update() error {
	originalComps := h.actionStateSources()
	topologyBlocked, err := h.reconcileActions()

	deleteNow := h.toDelete.Difference(topologyBlocked)
	h.shardingHandler.deleteComps(h.transCtx, h.dag, h.runningComps, deleteNow)
	h.shardingHandler.updateComps(h.transCtx, h.dag, h.runningComps, h.protoComps,
		h.toUpdate.Difference(topologyBlocked))
	h.persistActionState(originalComps, sets.KeySet(originalComps).Difference(deleteNow), topologyBlocked)
	h.shardingHandler.createComps(h.transCtx, h.dag, h.protoComps,
		h.toCreate.Difference(topologyBlocked))
	return err
}

func (h *nonBlockingShardingHandler) actionStateSources() map[string]*appsv1.Component {
	originalComps := make(map[string]*appsv1.Component)
	for name, comp := range h.runningComps {
		active := comp.Annotations[shardingAddActionTargetsKey] != "" ||
			comp.Annotations[shardingRemoveActionTargetsKey] != ""
		startsAdd := h.actions != nil && h.actions.ShardAdd != nil && h.actions.ShardAdd.NonBlocking &&
			h.toUpdate.Has(name) && comp.Annotations[shardingAddShardKey] != ""
		startsRemove := h.actions != nil && h.actions.ShardRemove != nil && h.actions.ShardRemove.NonBlocking &&
			h.toDelete.Has(name)
		if active || startsAdd || startsRemove {
			originalComps[name] = comp.DeepCopy()
		}
	}
	return originalComps
}

func (h *nonBlockingShardingHandler) persistActionState(originalComps map[string]*appsv1.Component,
	updateSet, topologyBlocked sets.Set[string]) {
	graphCli, _ := h.transCtx.Client.(model.GraphClient)
	for name := range updateSet {
		original, running := originalComps[name], h.runningComps[name]
		if original == nil || !shardingActionStateChanged(original, running) {
			continue
		}
		obj := running.DeepCopy()
		if proto := h.protoComps[name]; proto != nil && !topologyBlocked.Has(name) {
			if merged := copyAndMergeComponent(running, proto); merged != nil {
				obj = merged
			}
		}
		graphCli.Update(h.dag, original, obj, &model.ReplaceIfExistingOption{})
	}
}

func shardingActionStateChanged(original, current *appsv1.Component) bool {
	return original.Annotations[shardingAddActionTargetsKey] != current.Annotations[shardingAddActionTargetsKey] ||
		original.Annotations[shardingRemoveActionTargetsKey] != current.Annotations[shardingRemoveActionTargetsKey] ||
		original.Annotations[shardingAddShardKey] != current.Annotations[shardingAddShardKey]
}

func (h *nonBlockingShardingHandler) reconcileActions() (sets.Set[string], error) {
	var (
		topologyBlocked = sets.New[string]()
		completedRemove = sets.New[string]()
	)
	blockAll := func() {
		topologyBlocked.Insert(h.toCreate.UnsortedList()...)
		topologyBlocked.Insert(h.toDelete.UnsortedList()...)
		topologyBlocked.Insert(h.toUpdate.UnsortedList()...)
	}

	// An already-started action takes precedence over the latest topology diff.
	for _, name := range sets.List(sets.KeySet(h.runningComps)) {
		comp := h.runningComps[name]
		switch {
		case comp.Annotations[shardingAddActionTargetsKey] != "":
			if err := h.handleShardAdd(comp); err != nil {
				blockAll()
				return topologyBlocked, err
			}
		case comp.Annotations[shardingRemoveActionTargetsKey] != "":
			if err := h.handleShardRemove(comp); err != nil {
				blockAll()
				return topologyBlocked, err
			}
			completedRemove.Insert(name)
			if h.protoComps[name] != nil && h.actions != nil && h.actions.ShardAdd != nil {
				if comp.Annotations == nil {
					comp.Annotations = make(map[string]string)
				}
				comp.Annotations[shardingAddShardKey] = time.Now().Format(time.RFC3339Nano)
				blockAll()
				return topologyBlocked, pendingShardingAction(shardingAddShardAction,
					"waiting for completed shard remove state to persist")
			}
		}
	}

	h.markNewShards()
	addErr := h.reconcileShardAdds(topologyBlocked)
	if addErr != nil && h.actions != nil && h.actions.ShardAdd != nil && h.actions.ShardAdd.NonBlocking {
		topologyBlocked.Insert(h.toDelete.UnsortedList()...)
		return topologyBlocked, addErr
	}
	removeErr := h.reconcileShardRemoves(topologyBlocked, completedRemove)
	if addErr != nil {
		return topologyBlocked, addErr
	}
	return topologyBlocked, removeErr
}

func (h *nonBlockingShardingHandler) markNewShards() {
	if h.actions == nil || h.actions.ShardAdd == nil {
		return
	}
	now := time.Now().Format(time.RFC3339Nano)
	for name := range h.toCreate {
		comp := h.protoComps[name]
		if comp.Annotations == nil {
			comp.Annotations = make(map[string]string)
		}
		comp.Annotations[shardingAddShardKey] = now
	}
}

func (h *nonBlockingShardingHandler) reconcileShardAdds(topologyBlocked sets.Set[string]) error {
	var result error
	for _, name := range sets.List(h.toUpdate) {
		comp := h.runningComps[name]
		if err := h.handleShardAdd(comp); err != nil {
			if comp.Annotations[shardingAddActionTargetsKey] != "" {
				topologyBlocked.Insert(name)
			}
			if !ictrlutil.IsDelayedRequeueError(err) {
				h.transCtx.Logger.Error(err, "failed to call the shard add action", "shard", name)
			}
			result = preferredShardingActionError(result, err)
		}
	}
	return result
}

func (h *nonBlockingShardingHandler) reconcileShardRemoves(topologyBlocked, completed sets.Set[string]) error {
	var result error
	for _, name := range sets.List(h.toDelete) {
		if completed.Has(name) {
			continue
		}
		if err := h.handleShardRemove(h.runningComps[name]); err != nil {
			if !ictrlutil.IsDelayedRequeueError(err) {
				h.transCtx.Logger.Error(err, "failed to call the shard remove action", "shard", name)
			}
			result = preferredShardingActionError(result, err)
			topologyBlocked.Insert(name)
		}
	}
	return result
}

func preferredShardingActionError(current, next error) error {
	if current == nil || (ictrlutil.IsDelayedRequeueError(current) && !ictrlutil.IsDelayedRequeueError(next)) {
		return next
	}
	return current
}

func (h *nonBlockingShardingHandler) handleShardAdd(comp *appsv1.Component) error {
	if h.actions == nil || h.actions.ShardAdd == nil {
		return nil
	}
	pending := comp.Annotations[shardingAddShardKey] != "" ||
		(h.actions.ShardAdd.NonBlocking && comp.Annotations[shardingAddActionTargetsKey] != "")
	if pending {
		args := map[string]string{shardingAddShardNameVar: comp.Name}
		if err := h.callAction(shardingAddShardAction, h.actions.ShardAdd, args, comp); err != nil {
			return err
		}
	}
	delete(comp.Annotations, shardingAddShardKey)
	return nil
}

func (h *nonBlockingShardingHandler) handleShardRemove(comp *appsv1.Component) error {
	removeStarted := comp.Annotations[shardingRemoveActionTargetsKey] != ""
	if !removeStarted && comp.Annotations[shardingAddShardKey] != "" {
		if err := h.handleShardAdd(comp); err != nil {
			return err
		}
	}

	if h.actions == nil || h.actions.ShardRemove == nil {
		return nil
	}
	if comp.DeletionTimestamp.IsZero() {
		args := map[string]string{shardingRemoveShardNameVar: comp.Name}
		return h.callAction(shardingRemoveShardAction, h.actions.ShardRemove, args, comp)
	}
	return nil
}

func (h *nonBlockingShardingHandler) callAction(actionName string, action *appsv1.ShardingAction,
	args map[string]string, comp *appsv1.Component) error {
	if !action.NonBlocking {
		return h.shardingHandler.shardingAction(h.transCtx, h.shardingName, actionName,
			action, args, maps.Values(h.runningComps), comp)
	}
	annotations := map[string]string{
		shardingAddShardAction:    shardingAddActionTargetsKey,
		shardingRemoveShardAction: shardingRemoveActionTargetsKey,
	}
	annotation, ok := annotations[actionName]
	if !ok {
		return fmt.Errorf("action %s does not support non-blocking mode", actionName)
	}
	return h.shardingHandler.nonBlockingShardingAction(h.transCtx, h.shardingName, actionName,
		annotation, action, args, maps.Values(h.runningComps), comp)
}

func hasPendingNonBlockingAction(comp *appsv1.Component) bool {
	if comp == nil || comp.Annotations == nil {
		return false
	}
	if comp.Annotations[shardingAddActionTargetsKey] != "" ||
		comp.Annotations[shardingRemoveActionTargetsKey] != "" {
		return true
	}
	if comp.Annotations[shardingAddShardKey] == "" {
		return false
	}
	return slices.ContainsFunc(comp.Spec.CustomActions, func(action appsv1.CustomAction) bool {
		return action.Name == shardingAddShardAction && action.Action != nil && action.Action.NonBlocking
	})
}
