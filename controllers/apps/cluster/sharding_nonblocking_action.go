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
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const shardingActionTargetsVersion = 1

type shardingActionTargets struct {
	Version int                    `json:"version"`
	Targets []shardingActionTarget `json:"targets"`
}

func (h *clusterShardingHandler) nonBlockingShardingAction(transCtx *clusterTransformContext,
	shardingName, actionName, targetsAnnotation string, action *appsv1.ShardingAction,
	args map[string]string, runningComps []*appsv1.Component, sourceComp *appsv1.Component) error {
	for _, comp := range runningComps {
		addStarted := comp.Annotations[shardingAddActionTargetsKey] != ""
		removeStarted := comp.Annotations[shardingRemoveActionTargetsKey] != ""
		if (addStarted && (comp.Name != sourceComp.Name || targetsAnnotation != shardingAddActionTargetsKey)) ||
			(removeStarted && (comp.Name != sourceComp.Name || targetsAnnotation != shardingRemoveActionTargetsKey)) {
			return pendingShardingAction(actionName, "waiting for another sharding action")
		}
	}

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
		lfa, err := h.newLifecycle(transCtx, comps[target.Component])
		if err != nil {
			callErrors = append(callErrors, err)
			continue
		}
		for j := range target.Pods {
			pod := &target.Pods[j]
			opts := &lifecycle.Options{
				Rerun:         pod.Rerun,
				TargetPodName: pod.Name,
				PreConditionObjectSelector: constant.GetClusterLabels(transCtx.Cluster.Name,
					map[string]string{constant.KBAppShardingNameLabelKey: shardingName}),
			}
			err := lfa.UserDefined(transCtx.Context, transCtx.Client, opts, actionName, &action.Action, args)
			if err = lifecycle.IgnoreNotDefined(err); err == nil {
				pod.Rerun = false
				continue
			}
			switch {
			case errors.Is(err, lifecycle.ErrActionInProgress):
				pod.Rerun = false
				pending = true
			case errors.Is(err, lifecycle.ErrActionBusy):
				pending = true
			case isTerminalShardingActionError(err):
				pod.Rerun = true
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
	delete(sourceComp.Annotations, targetsAnnotation)
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
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].Name < shards[j].Name
	})

	targets := &shardingActionTargets{Version: shardingActionTargetsVersion}
	for _, shard := range shards {
		pods, err := component.ListOwnedInstances(transCtx.Context, transCtx.Client, shard)
		if err != nil {
			return nil, err
		}
		selectedPods, err := selectShardingActionPods(action, pods, shard.Name)
		if err != nil {
			return nil, err
		}
		targets.Targets = append(targets.Targets, shardingActionTarget{
			Component: shard.Name,
			Pods:      selectedPods,
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
		// A newly selected target belongs to a new request,
		// so it must not reuse a terminal result cached for an older request.
		targets = append(targets, shardingActionTargetPod{Name: pod.Name, Rerun: true})
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets, nil
}

type shardingActionTarget struct {
	Component string                    `json:"component"`
	Pods      []shardingActionTargetPod `json:"pods"`
}

type shardingActionTargetPod struct {
	Name  string `json:"name"`
	Rerun bool   `json:"rerun,omitempty"`
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
