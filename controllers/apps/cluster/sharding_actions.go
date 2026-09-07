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

type shardingActions struct {
	shardingHandler *clusterShardingHandler
	transCtx        *clusterTransformContext
	shardingName    string
	actions         *appsv1.ShardingLifecycleActions
	runningComps    map[string]*appsv1.Component
	toCreate        sets.Set[string]
	toDelete        sets.Set[string]
	toUpdate        sets.Set[string]
}

func (h *clusterShardingHandler) handleShardActions(transCtx *clusterTransformContext,
	shardingName string, runningComps map[string]*appsv1.Component,
	toCreate, toDelete, toUpdate sets.Set[string]) (sets.Set[string], error) {
	actions := &shardingActions{
		shardingHandler: h,
		transCtx:        transCtx,
		shardingName:    shardingName,
		runningComps:    runningComps,
		toCreate:        toCreate,
		toDelete:        toDelete,
		toUpdate:        toUpdate,
	}
	if shardingDef := h.shardingDef(transCtx, shardingName); shardingDef != nil {
		actions.actions = shardingDef.Spec.LifecycleActions
	}
	return actions.reconcile()
}

func (h *shardingActions) reconcile() (sets.Set[string], error) {
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

func (h *shardingActions) reconcileActions() (sets.Set[string], error) {
	all := h.toCreate.Union(h.toDelete).Union(h.toUpdate)
	names := sets.List(sets.KeySet(h.runningComps))
	// An already-started action takes precedence over the latest topology diff.
	for _, name := range names {
		comp := h.runningComps[name]
		switch {
		case comp.Annotations[shardingAddActionTargetsKey] != "":
			return h.advanceAction(comp, false, all)
		case comp.Annotations[shardingRemoveActionTargetsKey] != "":
			return h.advanceAction(comp, true, all)
		}
	}

	blocked, completedRemoves := sets.New[string](), sets.New[string]()
	var result error
	for _, name := range append(sets.List(h.toUpdate), sets.List(h.toDelete)...) {
		comp := h.runningComps[name]
		deleting := h.toDelete.Has(name)
		for _, remove := range []bool{false, true} {
			if (remove && !deleting) || (!remove && comp.Annotations[shardingAddShardKey] == "") {
				continue
			}
			var action *appsv1.ShardingAction
			if h.actions != nil {
				action = h.actions.ShardAdd
				if remove {
					action = h.actions.ShardRemove
				}
			}
			if action != nil && action.NonBlocking {
				blocked = all.Difference(completedRemoves)
				if result != nil {
					return blocked, result
				}
				// Complete earlier deletes before a new request freezes its targets.
				if len(completedRemoves) > 0 {
					return blocked, pendingShardingAction("sharding", "waiting for shard deletion")
				}
				// Finish partial creation before selecting potential participants.
				// Pod startup and template variables can require all Components.
				if len(h.toCreate) > 0 {
					return blocked.Difference(h.toCreate), pendingShardingAction("sharding", "waiting for shard creation")
				}
				return h.advanceAction(comp, remove, blocked)
			}
			if err := h.callAction(comp, remove); err != nil {
				h.transCtx.Logger.Error(err, "failed to call sharding action", "shard", name)
				if result == nil {
					result = err
				}
				if deleting {
					blocked.Insert(name)
				}
				break
			}
			if remove {
				completedRemoves.Insert(name)
			}
		}
	}
	return blocked, result
}

// Complete and persist one source request before selecting the next. The same
// topology guard applies to first selection, polling, retries and completion.
func (h *shardingActions) advanceAction(comp *appsv1.Component, remove bool,
	blocked sets.Set[string]) (sets.Set[string], error) {
	// A DELETE from a preceding reconciliation may still be in progress.
	for _, target := range h.runningComps {
		if !target.DeletionTimestamp.IsZero() {
			return blocked, pendingShardingAction("sharding", "waiting for shard deletion")
		}
	}
	if err := h.callAction(comp, remove); err != nil {
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

func (h *shardingActions) callAction(comp *appsv1.Component, remove bool) error {
	if remove {
		return h.shardingHandler.handleShardRemove(h.transCtx, h.shardingName, maps.Values(h.runningComps), comp)
	}
	return h.shardingHandler.handleShardAdd(h.transCtx, h.shardingName, maps.Values(h.runningComps), comp)
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
