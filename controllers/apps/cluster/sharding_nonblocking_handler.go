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
	// Restoring a still-desired participant is not a new topology change: the
	// outstanding request may need this Component in order to make progress.
	for _, comp := range h.runningComps {
		for _, annotation := range []string{shardingAddActionTargetsKey, shardingRemoveActionTargetsKey} {
			targets, found, parseErr := getShardingActionTargets(comp, annotation)
			if parseErr != nil || !found {
				continue // reconcileActions reports invalid snapshots.
			}
			for _, target := range targets.Targets {
				if h.toCreate.Has(target.Component) {
					topologyBlocked.Delete(target.Component)
				}
			}
		}
	}

	deleteNow := h.toDelete.Difference(topologyBlocked)
	h.shardingHandler.deleteComps(h.transCtx, h.dag, h.runningComps, deleteNow)
	h.shardingHandler.updateComps(h.transCtx, h.dag, h.runningComps, h.protoComps,
		h.toUpdate.Difference(topologyBlocked))
	h.persistActionState(originalComps, deleteNow)
	h.shardingHandler.createComps(h.transCtx, h.dag, h.protoComps,
		h.toCreate.Difference(topologyBlocked))
	return err
}

func (h *nonBlockingShardingHandler) actionStateSources() map[string]*appsv1.Component {
	originalComps := make(map[string]*appsv1.Component)
	for name, comp := range h.runningComps {
		active := comp.Annotations[shardingAddActionTargetsKey] != "" ||
			comp.Annotations[shardingRemoveActionTargetsKey] != ""
		if active || comp.Annotations[shardingAddShardKey] != "" || h.toDelete.Has(name) {
			originalComps[name] = comp.DeepCopy()
		}
	}
	return originalComps
}

func (h *nonBlockingShardingHandler) persistActionState(originalComps map[string]*appsv1.Component,
	deleting sets.Set[string]) {
	graphCli, _ := h.transCtx.Client.(model.GraphClient)
	for name, original := range originalComps {
		running := h.runningComps[name]
		if deleting.Has(name) || !shardingActionStateChanged(original, running) {
			continue
		}
		// Topology changes stay blocked while action state is being committed.
		graphCli.Update(h.dag, original, running.DeepCopy(), &model.ReplaceIfExistingOption{})
	}
}

func shardingActionStateChanged(original, current *appsv1.Component) bool {
	return original.Annotations[shardingAddActionTargetsKey] != current.Annotations[shardingAddActionTargetsKey] ||
		original.Annotations[shardingRemoveActionTargetsKey] != current.Annotations[shardingRemoveActionTargetsKey] ||
		original.Annotations[shardingAddShardKey] != current.Annotations[shardingAddShardKey]
}

func (h *nonBlockingShardingHandler) reconcileActions() (sets.Set[string], error) {
	blocked := h.toCreate.Union(h.toDelete).Union(h.toUpdate)
	names := sets.List(sets.KeySet(h.runningComps))
	// A DELETE in the preceding DAG is not necessarily finished yet. In
	// particular, AllShards must not select a terminating shard for a new call.
	for _, name := range names {
		if !h.runningComps[name].DeletionTimestamp.IsZero() {
			return blocked, pendingShardingAction("sharding", "waiting for shard deletion")
		}
	}
	// An already-started action takes precedence over the latest topology diff.
	for _, name := range names {
		comp := h.runningComps[name]
		switch {
		case comp.Annotations[shardingAddActionTargetsKey] != "":
			return h.advanceAction(comp, false, blocked)
		case comp.Annotations[shardingRemoveActionTargetsKey] != "":
			return h.advanceAction(comp, true, blocked)
		}
	}

	h.markNewShards()
	for _, name := range sets.List(h.toUpdate) {
		if comp := h.runningComps[name]; comp.Annotations[shardingAddShardKey] != "" &&
			h.actions != nil && h.actions.ShardAdd != nil {
			return h.advanceAction(comp, false, blocked)
		}
	}
	if deleting := sets.List(h.toDelete); len(deleting) > 0 {
		comp := h.runningComps[deleting[0]]
		if comp.Annotations[shardingAddShardKey] != "" && h.actions != nil && h.actions.ShardAdd != nil {
			return h.advanceAction(comp, false, blocked)
		}
		return h.advanceAction(comp, true, blocked)
	}
	return sets.New[string](), nil
}

// Complete and persist one source request before selecting the next. The same
// topology guard applies to first selection, polling, retries and completion.
func (h *nonBlockingShardingHandler) advanceAction(comp *appsv1.Component, remove bool,
	blocked sets.Set[string]) (sets.Set[string], error) {
	var err error
	if remove {
		err = h.handleShardRemove(comp)
	} else {
		err = h.handleShardAdd(comp)
	}
	if err != nil {
		if !ictrlutil.IsDelayedRequeueError(err) {
			h.transCtx.Logger.Error(err, "failed to call sharding action", "shard", comp.Name)
		}
		return blocked, err
	}
	if remove {
		if h.toDelete.Has(comp.Name) {
			// Keep the successful remove snapshot until deletion so that a failed
			// DELETE can retry the cached result without starting a new request.
			blocked.Delete(comp.Name)
			return blocked, pendingShardingAction("sharding", "waiting for shard deletion")
		}
		delete(comp.Annotations, shardingRemoveActionTargetsKey)
		if h.actions != nil && h.actions.ShardAdd != nil {
			comp.Annotations[shardingAddShardKey] = time.Now().Format(time.RFC3339Nano)
		}
	}
	return blocked, pendingShardingAction("sharding", "waiting for completed action state to persist")
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
	annotation := shardingAddActionTargetsKey
	if actionName == shardingRemoveShardAction {
		annotation = shardingRemoveActionTargetsKey
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
