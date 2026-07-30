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
	"context"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestShardingActionStateIsChunked(t *testing.T) {
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster"},
	}
	source := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   cluster.Namespace,
			Name:        "cluster-shard-0000",
			UID:         types.UID("f876de31-9638-4d9e-87dd-cc21c2f523d8"),
			Annotations: map[string]string{},
		},
	}
	reader := &appsutil.MockReader{}
	graphCli := model.NewGraphClient(reader)
	transCtx := &clusterTransformContext{
		Context:     context.Background(),
		Client:      graphCli,
		Cluster:     cluster,
		OrigCluster: cluster.DeepCopy(),
	}
	dag := graph.NewDAG()
	graphCli.Root(dag, transCtx.OrigCluster, transCtx.Cluster, model.ActionUpdatePtr())

	state := &shardingActionTargets{Version: shardingActionTargetsVersion}
	for i := 0; i < 2048; i++ {
		target := shardingActionTarget{
			Component: fmt.Sprintf("cluster-shard-%04d-with-a-long-component-name", i),
		}
		for j := 0; j < 8; j++ {
			target.Pods = append(target.Pods, shardingActionTargetPod{
				Name:  fmt.Sprintf("cluster-shard-%04d-with-a-long-component-name-%05d", i, j),
				Rerun: true,
			})
		}
		state.Targets = append(state.Targets, target)
	}

	if err := setShardingActionState(transCtx, dag, source, shardingAddActionStateKey, state); err != nil {
		t.Fatal(err)
	}
	ref, found, err := getShardingActionStateRef(source, shardingAddActionStateKey)
	if err != nil {
		t.Fatal(err)
	}
	if !found || ref.Chunks < 2 {
		t.Fatalf("expected chunked state, got found=%v chunks=%d", found, ref.Chunks)
	}
	if len(source.Annotations[shardingAddActionStateKey]) >= 1024 {
		t.Fatalf("state reference is unexpectedly large: %d bytes",
			len(source.Annotations[shardingAddActionStateKey]))
	}
	if got := len(graphCli.FindAll(dag, &corev1.ConfigMap{})); got != ref.Chunks {
		t.Fatalf("expected %d ConfigMaps, got %d", ref.Chunks, got)
	}

	restored, found, err := getShardingActionState(transCtx, dag, source, shardingAddActionStateKey)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !reflect.DeepEqual(state, restored) {
		t.Fatal("restored state differs from persisted state")
	}
}

func TestPendingShardingActionPreventsUpToDateFastPath(t *testing.T) {
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster", Generation: 3},
	}
	comp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  cluster.Namespace,
			Name:       "cluster-shard-0",
			Generation: 2,
			Labels: map[string]string{
				constant.AppManagedByLabelKey: constant.AppName,
				constant.AppInstanceLabelKey:  cluster.Name,
			},
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "3",
				shardingAddActionStateKey:        `{"version":1,"name":"state","chunks":1}`,
			},
		},
		Status: appsv1.ComponentStatus{ObservedGeneration: 2},
	}
	if !hasPendingShardingAction(comp) {
		t.Fatal("expected persisted non-blocking state to keep reconciliation active")
	}
	transCtx := &clusterTransformContext{
		Context:     context.Background(),
		Client:      model.NewGraphClient(&appsutil.MockReader{Objects: []client.Object{comp}}),
		Cluster:     cluster,
		OrigCluster: cluster.DeepCopy(),
		components:  []*appsv1.ClusterComponentSpec{{Name: "shard-0"}},
	}
	upToDate, err := checkAllCompsUpToDate(transCtx, cluster)
	if err != nil {
		t.Fatal(err)
	}
	if upToDate {
		t.Fatal("pending sharding action must bypass the up-to-date fast path")
	}

	delete(comp.Annotations, shardingAddActionStateKey)
	upToDate, err = checkAllCompsUpToDate(transCtx, cluster)
	if err != nil {
		t.Fatal(err)
	}
	if !upToDate {
		t.Fatal("component should be up to date after the pending state is cleared")
	}
}
