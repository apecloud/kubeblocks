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
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestShardingActionTargetsAnnotationRoundTrip(t *testing.T) {
	comp := &appsv1.Component{}
	comp.Name = "source"
	targets := &shardingActionTargets{Version: shardingActionTargetsVersion}
	for shard := 0; shard < 2048; shard++ {
		target := shardingActionTarget{
			Component: fmt.Sprintf("cluster-sharding-%04d", shard),
		}
		for replica := 0; replica < 3; replica++ {
			target.Pods = append(target.Pods, shardingActionTargetPod{
				Name:  fmt.Sprintf("cluster-sharding-%04d-%d", shard, replica),
				Rerun: replica == 1,
			})
		}
		targets.Targets = append(targets.Targets, target)
	}

	if err := setShardingActionTargets(comp, shardingAddActionTargetsKey, targets); err != nil {
		t.Fatalf("setShardingActionTargets() error = %v", err)
	}
	value := comp.Annotations[shardingAddActionTargetsKey]
	if len(value) >= 256*1024 {
		t.Fatalf("encoded targets annotation has %d bytes", len(value))
	}
	if !strings.HasPrefix(value, shardingActionTargetsGZIPPrefix) {
		t.Fatal("maximum-size target snapshot was not compacted")
	}

	got, found, err := getShardingActionTargets(comp, shardingAddActionTargetsKey)
	if err != nil {
		t.Fatalf("getShardingActionTargets() error = %v", err)
	}
	if !found {
		t.Fatal("getShardingActionTargets() found = false")
	}
	if len(got.Targets) != len(targets.Targets) {
		t.Fatalf("getShardingActionTargets() returned %d targets, want %d", len(got.Targets), len(targets.Targets))
	}
	if got.Targets[1024].Pods[1].Name != targets.Targets[1024].Pods[1].Name ||
		!got.Targets[1024].Pods[1].Rerun {
		t.Fatalf("getShardingActionTargets() did not preserve a participant: %#v", got.Targets[1024])
	}
}

func TestShardingActionTargetsAnnotationKeepsSmallSnapshotsReadable(t *testing.T) {
	comp := &appsv1.Component{}
	comp.Name = "source"
	targets := &shardingActionTargets{
		Version: shardingActionTargetsVersion,
		Targets: []shardingActionTarget{{
			Component: "shard-0",
			Pods:      []shardingActionTargetPod{{Name: "shard-0-0"}},
		}},
	}

	if err := setShardingActionTargets(comp, shardingAddActionTargetsKey, targets); err != nil {
		t.Fatalf("setShardingActionTargets() error = %v", err)
	}
	if value := comp.Annotations[shardingAddActionTargetsKey]; strings.HasPrefix(value, shardingActionTargetsGZIPPrefix) {
		t.Fatalf("small target snapshot should remain readable: %q", value)
	}
}

func TestShardingActionTargetsAnnotationRejectsOversizedDecodedData(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(bytes.Repeat([]byte("x"), shardingActionTargetsMaxDecodedSize+1)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	comp := &appsv1.Component{}
	comp.Name = "source"
	comp.Annotations = map[string]string{
		shardingAddActionTargetsKey: shardingActionTargetsGZIPPrefix +
			base64.StdEncoding.EncodeToString(compressed.Bytes()),
	}
	_, _, err := getShardingActionTargets(comp, shardingAddActionTargetsKey)
	if err == nil || !strings.Contains(err.Error(), "decoded data exceeds") {
		t.Fatalf("getShardingActionTargets() error = %v", err)
	}
}
