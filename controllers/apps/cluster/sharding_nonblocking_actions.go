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
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type nonBlockingShardingActions struct {
	shardingHandler *clusterShardingHandler
	transCtx        *clusterTransformContext
	shardingName    string
	actions         *appsv1.ShardingLifecycleActions
	runningComps    map[string]*appsv1.Component
	toCreate        sets.Set[string]
	toDelete        sets.Set[string]
	toUpdate        sets.Set[string]
}

func newNonBlockingShardingActions(shardingHandler *clusterShardingHandler,
	transCtx *clusterTransformContext, shardingName string, runningComps map[string]*appsv1.Component,
	toCreate, toDelete, toUpdate sets.Set[string]) *nonBlockingShardingActions {
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
	return &nonBlockingShardingActions{
		shardingHandler: shardingHandler,
		transCtx:        transCtx,
		shardingName:    shardingName,
		actions:         actions,
		runningComps:    runningComps,
		toCreate:        toCreate,
		toDelete:        toDelete,
		toUpdate:        toUpdate,
	}
}

func (h *nonBlockingShardingActions) reconcile() (sets.Set[string], error) {
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

	return topologyBlocked, err
}

func (h *nonBlockingShardingActions) reconcileActions() (sets.Set[string], error) {
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
func (h *nonBlockingShardingActions) advanceAction(comp *appsv1.Component, remove bool,
	blocked sets.Set[string]) (sets.Set[string], error) {
	var err error
	if remove {
		err = h.shardingHandler.handleShardRemove(h.transCtx, h.shardingName, maps.Values(h.runningComps), comp)
	} else {
		err = h.shardingHandler.handleShardAdd(h.transCtx, h.shardingName, maps.Values(h.runningComps), comp)
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
