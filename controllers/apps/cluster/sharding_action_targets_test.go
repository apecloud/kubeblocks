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
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestShardingActionTargetsAnnotationRoundTrip(t *testing.T) {
	comp := &appsv1.Component{}
	comp.Name = "source"
	targets := &shardingActionTargets{
		Version: shardingActionTargetsVersion,
		Targets: []shardingActionTarget{
			{
				Component: "shard-1",
				Pods: []shardingActionTargetPod{
					{Name: "shard-1-1", Rerun: true},
					{Name: "shard-1-0"},
				},
			},
			{Component: "shard-0", Pods: []shardingActionTargetPod{{Name: "shard-0-0"}}},
		},
	}

	if err := setShardingActionTargets(comp, shardingAddActionTargetsKey, targets); err != nil {
		t.Fatalf("setShardingActionTargets() error = %v", err)
	}
	if value := comp.Annotations[shardingAddActionTargetsKey]; value == "" || value[0] != '{' {
		t.Fatalf("targets annotation is not JSON: %q", value)
	}

	got, found, err := getShardingActionTargets(comp, shardingAddActionTargetsKey)
	if err != nil {
		t.Fatalf("getShardingActionTargets() error = %v", err)
	}
	if !found {
		t.Fatal("getShardingActionTargets() found = false")
	}
	if got.Targets[0].Component != "shard-0" || got.Targets[1].Component != "shard-1" ||
		got.Targets[1].Pods[0].Name != "shard-1-0" || !got.Targets[1].Pods[1].Rerun {
		t.Fatalf("getShardingActionTargets() returned unexpected targets: %#v", got.Targets)
	}
}
