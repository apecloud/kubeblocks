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
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const shardingActionTargetsVersion = 1

type shardingActionTargets struct {
	Version int                    `json:"version"`
	Targets []shardingActionTarget `json:"targets"`
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
