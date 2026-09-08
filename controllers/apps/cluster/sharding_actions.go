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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	shardingActionTargetsVersion   = 1
	shardingAddActionTargetsKey    = "kubeblocks.io/sharding-add-action-targets"
	shardingRemoveActionTargetsKey = "kubeblocks.io/sharding-remove-action-targets"
)

type shardingActionTargets struct {
	Version int                    `json:"version"`
	Targets []shardingActionTarget `json:"targets"`
}

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

func (h *clusterShardingHandler) nonBlockingShardingAction(transCtx *clusterTransformContext,
	shardingName, actionName, targetsAnnotation string, action *appsv1.ShardingAction,
	args map[string]string, runningComps []*appsv1.Component, sourceComp *appsv1.Component) error {
	targets, changed, err := h.resolveShardingActionTargets(
		transCtx, action, targetsAnnotation, runningComps, sourceComp)
	if err != nil {
		return err
	}
	if changed {
		if err := setShardingActionTargets(sourceComp, targetsAnnotation, targets); err != nil {
			return err
		}
		return pendingShardingAction(actionName, "targets selected")
	}

	comps := make(map[string]*appsv1.Component, len(runningComps))
	for _, comp := range runningComps {
		comps[comp.Name] = comp
	}

	var callErrors []error
	pending := false
	for i := range targets.Targets {
		target := &targets.Targets[i]
		lfa, err := h.newLifecycle(transCtx, comps[target.Component], target.TemplateVars)
		if err != nil {
			callErrors = append(callErrors, err)
			continue
		}
		for j := range target.Pods {
			pod := &target.Pods[j]
			opts := &lifecycle.Options{
				Query:         pod.Query,
				TargetPodName: pod.Name,
				PreConditionObjectSelector: constant.GetClusterLabels(transCtx.Cluster.Name,
					map[string]string{constant.KBAppShardingNameLabelKey: shardingName}),
			}
			err := lfa.UserDefined(transCtx.Context, transCtx.Client, opts, actionName, &action.Action, args)
			if err = lifecycle.IgnoreNotDefined(err); err == nil {
				pod.Query = true
				continue
			}
			switch {
			case errors.Is(err, lifecycle.ErrActionInProgress):
				pod.Query = true
				pending = true
			case errors.Is(err, lifecycle.ErrActionBusy):
				pending = true
			case errors.Is(err, lifecycle.ErrActionResultNotFound):
				// Persist the restart decision first. The next call must pass
				// startup preconditions again before it can execute anything.
				pod.Query = false
				pending = true
			case isTerminalShardingActionError(err):
				pod.Query = false
				callErrors = append(callErrors, err)
			default:
				callErrors = append(callErrors, err)
			}
		}
	}
	if err := setShardingActionTargets(sourceComp, targetsAnnotation, targets); err != nil {
		return err
	}
	if len(callErrors) > 0 {
		return errors.Join(callErrors...)
	}
	if pending {
		return pendingShardingAction(actionName, "still running")
	}
	if targetsAnnotation == shardingAddActionTargetsKey {
		delete(sourceComp.Annotations, targetsAnnotation)
	}
	return nil
}

func pendingShardingAction(actionName, reason string) error {
	return ictrlutil.NewDelayedRequeueError(3*time.Second,
		fmt.Sprintf("action %s is %s", actionName, reason))
}
func isTerminalShardingActionError(err error) bool {
	return errors.Is(err, lifecycle.ErrActionFailed) ||
		errors.Is(err, lifecycle.ErrActionTimedOut) ||
		errors.Is(err, lifecycle.ErrActionInternalError)
}

func (h *clusterShardingHandler) resolveShardingActionTargets(transCtx *clusterTransformContext,
	action *appsv1.ShardingAction, targetsAnnotation string, runningComps []*appsv1.Component,
	sourceComp *appsv1.Component) (*shardingActionTargets, bool, error) {
	targets, found, err := getShardingActionTargets(sourceComp, targetsAnnotation)
	if err != nil {
		return nil, false, err
	}
	if !found {
		targets, err = h.selectShardingActionTargets(transCtx, action, runningComps, sourceComp)
		return targets, true, err
	}

	comps := make(map[string]*appsv1.Component, len(runningComps))
	for _, comp := range runningComps {
		comps[comp.Name] = comp
	}
	changed := false
	for i := range targets.Targets {
		target := &targets.Targets[i]
		comp := comps[target.Component]
		if comp == nil {
			return nil, false, pendingShardingAction("sharding",
				fmt.Sprintf("waiting for target shard %s", target.Component))
		}
		pods, err := component.ListOwnedInstances(transCtx.Context, transCtx.Client, comp)
		if err != nil {
			return nil, false, err
		}
		existing := sets.New[string]()
		for _, pod := range pods {
			existing.Insert(pod.Name)
		}
		surviving := sets.New[string]()
		missing := 0
		for _, pod := range target.Pods {
			if existing.Has(pod.Name) {
				surviving.Insert(pod.Name)
			} else {
				missing++
			}
		}
		if missing == 0 {
			continue
		}
		selected, err := selectShardingActionPods(action, pods, comp.Name)
		if err != nil {
			return nil, false, pendingShardingAction("sharding",
				fmt.Sprintf("waiting for replacement pods on shard %s", comp.Name))
		}
		replacements := make([]shardingActionTargetPod, 0, missing)
		for _, pod := range selected {
			if !surviving.Has(pod.Name) {
				replacements = append(replacements, pod)
			}
		}
		if len(replacements) < missing {
			return nil, false, pendingShardingAction("sharding",
				fmt.Sprintf("waiting for %d replacement pods on shard %s", missing, comp.Name))
		}
		next := 0
		for j := range target.Pods {
			if !existing.Has(target.Pods[j].Name) {
				target.Pods[j] = replacements[next]
				next++
			}
		}
		changed = true
	}
	return targets, changed, nil
}

func (h *clusterShardingHandler) selectShardingActionTargets(transCtx *clusterTransformContext,
	action *appsv1.ShardingAction, runningComps []*appsv1.Component,
	sourceComp *appsv1.Component) (*shardingActionTargets, error) {
	shards, err := h.selectTargetShard(action, runningComps, sourceComp)
	if err != nil {
		return nil, err
	}
	targets := &shardingActionTargets{Version: shardingActionTargetsVersion}
	for _, shard := range shards {
		pods, err := component.ListOwnedInstances(transCtx.Context, transCtx.Client, shard)
		if err != nil {
			return nil, err
		}
		// Do not freeze a partial scale-out or surplus scale-in replica into
		// a request. The topology may still be converging from an earlier update.
		if shard.Generation != shard.Status.ObservedGeneration || len(pods) != int(shard.Spec.Replicas) {
			return nil, pendingShardingAction("sharding", fmt.Sprintf("waiting for shard %s pod topology", shard.Name))
		}
		for _, pod := range pods {
			if !pod.DeletionTimestamp.IsZero() {
				return nil, pendingShardingAction("sharding", fmt.Sprintf("waiting for shard %s pod deletion", shard.Name))
			}
		}
		compDef := transCtx.componentDefs[shard.Spec.CompDef]
		if compDef == nil {
			return nil, fmt.Errorf("component definition not found for shard %s", shard.Name)
		}
		synthesized, err := component.BuildSynthesizedComponent(transCtx.Context, transCtx.Client, compDef, shard)
		if err != nil {
			return nil, err
		}
		// Resolve once for this request. Secret-backed environment references are
		// kept as references by the existing template-variable resolver.
		vars, _, err := component.ResolveTemplateNEnvVars(transCtx.Context, transCtx.Client, synthesized, compDef.Spec.Vars)
		if err != nil {
			return nil, err
		}
		selectedPods, err := selectShardingActionPods(action, pods, shard.Name)
		if err != nil {
			return nil, err
		}
		targets.Targets = append(targets.Targets, shardingActionTarget{
			Component:    shard.Name,
			Pods:         selectedPods,
			TemplateVars: vars,
		})
	}
	return targets, nil
}

func selectShardingActionPods(action *appsv1.ShardingAction, pods []*corev1.Pod,
	componentName string) ([]shardingActionTargetPod, error) {
	if len(pods) == 0 {
		return nil, fmt.Errorf("shard %s has no pods to execute action", componentName)
	}
	selected, err := lifecycle.SelectTargetPods(pods, pods[0], &action.Action)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("shard %s has no pod matching the action target selector", componentName)
	}
	targets := make([]shardingActionTargetPod, 0, len(selected))
	for _, pod := range selected {
		targets = append(targets, shardingActionTargetPod{Name: pod.Name})
	}
	return targets, nil
}

type shardingActionTarget struct {
	Component    string                    `json:"component"`
	Pods         []shardingActionTargetPod `json:"pods"`
	TemplateVars map[string]string         `json:"templateVars,omitempty"`
}

type shardingActionTargetPod struct {
	Name  string `json:"name"`
	Query bool   `json:"query,omitempty"`
}

func getShardingActionTargets(comp *appsv1.Component, annotation string) (*shardingActionTargets, bool, error) {
	value, found := comp.Annotations[annotation]
	if !found {
		return nil, false, nil
	}

	targets := &shardingActionTargets{}
	if err := json.Unmarshal([]byte(value), targets); err != nil {
		return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, comp.Name, err)
	}
	if err := validateShardingActionTargets(targets); err != nil {
		return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, comp.Name, err)
	}
	return targets, true, nil
}

func setShardingActionTargets(comp *appsv1.Component, annotation string, targets *shardingActionTargets) error {
	sortShardingActionTargets(targets)
	data, err := json.Marshal(targets)
	if err != nil {
		return err
	}
	if comp.Annotations == nil {
		comp.Annotations = map[string]string{}
	}
	comp.Annotations[annotation] = string(data)
	return nil
}

func sortShardingActionTargets(targets *shardingActionTargets) {
	for i := range targets.Targets {
		sort.Slice(targets.Targets[i].Pods, func(j, k int) bool {
			return targets.Targets[i].Pods[j].Name < targets.Targets[i].Pods[k].Name
		})
	}
	sort.Slice(targets.Targets, func(i, j int) bool {
		return targets.Targets[i].Component < targets.Targets[j].Component
	})
}

func validateShardingActionTargets(targets *shardingActionTargets) error {
	if targets.Version != shardingActionTargetsVersion {
		return fmt.Errorf("unsupported version %d", targets.Version)
	}
	if len(targets.Targets) == 0 {
		return fmt.Errorf("targets must not be empty")
	}
	components := sets.New[string]()
	pods := sets.New[string]()
	for _, target := range targets.Targets {
		if target.Component == "" {
			return fmt.Errorf("target component must not be empty")
		}
		if components.Has(target.Component) {
			return fmt.Errorf("duplicate target component %s", target.Component)
		}
		components.Insert(target.Component)
		if len(target.Pods) == 0 {
			return fmt.Errorf("target component %s has no pods", target.Component)
		}
		for _, pod := range target.Pods {
			if pod.Name == "" {
				return fmt.Errorf("target pod name must not be empty")
			}
			if pods.Has(pod.Name) {
				return fmt.Errorf("duplicate target pod %s", pod.Name)
			}
			pods.Insert(pod.Name)
		}
	}
	return nil
}
